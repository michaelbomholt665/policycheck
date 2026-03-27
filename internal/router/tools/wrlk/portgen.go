// internal/router/tools/wrlk/portgen.go
//
// Package main implements the portgen CLI for the Port Router.
//
// Package Concerns:
//   - Single-action port registration: adds a constant to ports.go and a
//     switch case to registry_imports.go in one explicit command.
//   - Atomic writes for all three targets (ports.go, registry_imports.go, router.lock).
//   - Read-only dry-run mode for safe inspection.
//   - Zero third-party imports: stdlib only.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const (
	portsRelPath      = "internal/router/ports.go"
	validationRelPath = "internal/router/registry_imports.go"
	lockRelPath       = "internal/router/router.lock"
)

// RouterRunPortgenCommand executes the portgen CLI as a wrlk subcommand.
func RouterRunPortgenCommand(options globalOptions, args []string, stdout io.Writer, _ io.Writer) error {
	if len(args) > 0 && RouterIsHelpToken(args[0]) {
		return RouterWritePortgenUsage(stdout)
	}

	addOptions, err := RouterParsePortgenFlags(args)
	if err != nil {
		return &usageError{message: err.Error()}
	}

	if err := RouterAddPort(options.root, addOptions.name, addOptions.value, addOptions.dryRun, stdout); err != nil {
		return err
	}

	return nil
}

type portgenAddOptions struct {
	name   string
	value  string
	dryRun bool
}

type portgenMutation struct {
	portsPath         string
	validationPath    string
	updatedPorts      []byte
	updatedValidation []byte
}

// RouterParsePortgenFlags parses portgen add subcommand flags.
func RouterParsePortgenFlags(args []string) (portgenAddOptions, error) {
	options := portgenAddOptions{}

	fs := flag.NewFlagSet("wrlk add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&options.name, "name", "", "port constant name (e.g. PortFoo)")
	fs.StringVar(&options.value, "value", "", "port string value (e.g. foo)")
	fs.BoolVar(&options.dryRun, "dry-run", false, "print changes without writing")

	if err := fs.Parse(args); err != nil {
		return portgenAddOptions{}, fmt.Errorf("parse portgen add flags: %w", err)
	}

	if options.name == "" {
		return portgenAddOptions{}, fmt.Errorf("--name is required")
	}
	if options.value == "" {
		return portgenAddOptions{}, fmt.Errorf("--value is required")
	}

	return options, nil
}

// RouterAddPort is the top-level action: injects the constant, the switch case, and rewrites the lock.
func RouterAddPort(root, name, value string, dryRun bool, stdout io.Writer) (returnErr error) {
	mutation, err := RouterBuildPortgenMutation(root, name, value)
	if err != nil {
		return fmt.Errorf("build portgen mutation: %w", err)
	}

	if dryRun {
		return RouterWritePortgenDryRunOutput(stdout, name, value, portsRelPath, validationRelPath)
	}

	snapshot, err := RouterCaptureSnapshot(
		root,
		fmt.Sprintf("wrlk add --name %s --value %s", name, value),
	)
	if err != nil {
		return fmt.Errorf("capture snapshot before portgen: %w", err)
	}

	if err := RouterWriteSnapshot(root, snapshot); err != nil {
		return fmt.Errorf("write snapshot before portgen: %w", err)
	}
	defer func() {
		if returnErr == nil {
			return
		}

		if restoreErr := RouterRestoreSnapshot(root, snapshot); restoreErr != nil {
			returnErr = fmt.Errorf("%w (restore snapshot: %v)", returnErr, restoreErr)
		}
	}()

	if err := RouterWritePortsFile(mutation.portsPath, mutation.updatedPorts); err != nil {
		return fmt.Errorf("write updated ports file: %w", err)
	}

	if err := RouterWriteValidationFile(mutation.validationPath, mutation.updatedValidation); err != nil {
		return fmt.Errorf("write updated validation file: %w", err)
	}

	if err := RouterWriteLockAfterPortgen(root); err != nil {
		return fmt.Errorf("write updated router lock: %w", err)
	}

	if err := RouterWritePortgenMessage(stdout, "wrlk: added port %s = %q\n", name, value); err != nil {
		return fmt.Errorf("write portgen success message: %w", err)
	}

	return nil
}

// RouterBuildPortgenMutation reads the managed router files and builds the updated content.
func RouterBuildPortgenMutation(root, name, value string) (portgenMutation, error) {
	portsPath := filepath.Join(root, filepath.FromSlash(portsRelPath))
	validationPath := filepath.Join(root, filepath.FromSlash(validationRelPath))

	portsContent, validationContent, readErr := RouterReadManagedPortFiles(portsPath, validationPath)
	if readErr != nil {
		return portgenMutation{}, fmt.Errorf("read managed router files: %w", readErr)
	}

	if verifyErr := RouterVerifyManagedPortFiles(portsContent, validationContent); verifyErr != nil {
		return portgenMutation{}, fmt.Errorf("preflight managed router files: %w", verifyErr)
	}

	updatedPorts, updatedValidation, err := RouterBuildUpdatedManagedPortFiles(portsContent, validationContent, name, value)
	if err != nil {
		return portgenMutation{}, fmt.Errorf("build updated managed router files: %w", err)
	}

	return portgenMutation{
		portsPath:         portsPath,
		validationPath:    validationPath,
		updatedPorts:      updatedPorts,
		updatedValidation: updatedValidation,
	}, nil
}

// RouterReadManagedPortFiles loads the mutable router port files from disk.
func RouterReadManagedPortFiles(portsPath, validationPath string) ([]byte, []byte, error) {
	portsContent, err := os.ReadFile(portsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read ports file: %w", err)
	}

	validationContent, err := os.ReadFile(validationPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read validation file: %w", err)
	}

	return portsContent, validationContent, nil
}

// RouterBuildUpdatedManagedPortFiles applies one new port to the managed router files.
func RouterBuildUpdatedManagedPortFiles(
	portsContent []byte,
	validationContent []byte,
	name string,
	value string,
) ([]byte, []byte, error) {
	updatedPorts, err := RouterInjectPortConstant(portsContent, name, value)
	if err != nil {
		return nil, nil, fmt.Errorf("inject port constant: %w", err)
	}

	updatedValidation, err := RouterInjectValidationCase(validationContent, name)
	if err != nil {
		return nil, nil, fmt.Errorf("inject validation case: %w", err)
	}

	if err := RouterVerifyManagedPortFiles(updatedPorts, updatedValidation); err != nil {
		return nil, nil, fmt.Errorf("verify updated managed router files: %w", err)
	}

	return updatedPorts, updatedValidation, nil
}

// RouterInjectPortConstant appends a new PortName constant into the const block.
func RouterInjectPortConstant(content []byte, name, value string) ([]byte, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, portsRelPath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse ports file: %w", err)
	}

	constBlock, err := RouterFindPortConstBlock(file)
	if err != nil {
		return nil, err
	}

	declaredNames, err := RouterDeclaredPortNames(content)
	if err != nil {
		return nil, err
	}
	if slices.Contains(declaredNames, name) {
		return nil, fmt.Errorf("wrlk: port %q already declared in ports.go", name)
	}

	constBlock.Specs = append(constBlock.Specs, &ast.ValueSpec{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{{
				Text: fmt.Sprintf("// %s declares the %q provider port.", name, value),
			}},
		},
		Names: []*ast.Ident{ast.NewIdent(name)},
		Type:  ast.NewIdent("PortName"),
		Values: []ast.Expr{
			&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(value)},
		},
	})

	return RouterPrintGoFile(fileSet, file)
}

// RouterInjectValidationCase injects a new case into RouterValidatePortName's switch.
func RouterInjectValidationCase(content []byte, name string) ([]byte, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, validationRelPath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse validation file: %w", err)
	}

	trueClause, err := RouterFindValidationSwitchClauses(file)
	if err != nil {
		return nil, err
	}
	trueClause.List = append(trueClause.List, ast.NewIdent(name))

	return RouterPrintGoFile(fileSet, file)
}

// RouterWritePortsFile writes updated ports.go content atomically.
func RouterWritePortsFile(path string, content []byte) error {
	if err := RouterAtomicWriteFile(path, content); err != nil {
		return fmt.Errorf("write ports file %s: %w", path, err)
	}

	return nil
}

// RouterWriteValidationFile writes updated registry_imports.go content atomically.
func RouterWriteValidationFile(path string, content []byte) error {
	if err := RouterAtomicWriteFile(path, content); err != nil {
		return fmt.Errorf("write validation file %s: %w", path, err)
	}

	return nil
}

// RouterWriteLockAfterPortgen recomputes and rewrites router.lock using the same logic as wrlk lock update.
func RouterWriteLockAfterPortgen(root string) error {
	records, err := RouterComputeLockRecords(root)
	if err != nil {
		return fmt.Errorf("compute lock records after portgen: %w", err)
	}

	if err := RouterWriteLockRecords(root, records); err != nil {
		return fmt.Errorf("write lock file after portgen: %w", err)
	}

	return nil
}

// RouterAtomicWriteFile writes content to path using a temp-file-and-rename pattern.
func RouterAtomicWriteFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}

	tmpFile, err := os.CreateTemp(dir, "portgen.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}

	tmpPath := tmpFile.Name()
	writeErr := RouterWriteAndCloseTempFile(tmpFile, content)
	if writeErr != nil {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("write temp file %s: %w (cleanup: %v)", tmpPath, writeErr, removeErr)
		}

		return writeErr
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmpPath, path, err)
	}

	return nil
}

// RouterWriteAndCloseTempFile flushes and closes a temp file, cleaning up on error.
func RouterWriteAndCloseTempFile(file *os.File, content []byte) error {
	tmpPath := file.Name()

	if _, err := file.Write(content); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("write temp file %s: %w (close: %v)", tmpPath, err, closeErr)
		}

		return fmt.Errorf("write temp file %s: %w", tmpPath, err)
	}

	if err := file.Sync(); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("sync temp file %s: %w (close: %v)", tmpPath, err, closeErr)
		}

		return fmt.Errorf("sync temp file %s: %w", tmpPath, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp file %s: %w", tmpPath, err)
	}

	return nil
}

// RouterVerifyManagedPortFiles ensures the mutable router surfaces remain in sync.
func RouterVerifyManagedPortFiles(portsContent []byte, validationContent []byte) error {
	declaredNames, err := RouterDeclaredPortNames(portsContent)
	if err != nil {
		return fmt.Errorf("read declared ports: %w", err)
	}

	allowedNames, err := RouterAllowedPortNames(validationContent)
	if err != nil {
		return fmt.Errorf("read allowed ports: %w", err)
	}

	missingInValidation := RouterPortSetDifference(declaredNames, allowedNames)
	if len(missingInValidation) > 0 {
		return fmt.Errorf(
			"ports declared in %s but missing from %s: %s",
			portsRelPath,
			validationRelPath,
			strings.Join(missingInValidation, ", "),
		)
	}

	extraInValidation := RouterPortSetDifference(allowedNames, declaredNames)
	if len(extraInValidation) > 0 {
		return fmt.Errorf(
			"ports allowed in %s but missing from %s: %s",
			validationRelPath,
			portsRelPath,
			strings.Join(extraInValidation, ", "),
		)
	}

	return nil
}

// RouterDeclaredPortNames extracts the declared PortName constants from ports.go.
func RouterDeclaredPortNames(content []byte) ([]string, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, portsRelPath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse ports file: %w", err)
	}

	constBlock, err := RouterFindPortConstBlock(file)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(constBlock.Specs))
	for _, spec := range constBlock.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			return nil, fmt.Errorf("unsupported const spec in %s", portsRelPath)
		}
		if !RouterValueSpecHasPortNameType(valueSpec) {
			continue
		}
		for _, ident := range valueSpec.Names {
			names = append(names, ident.Name)
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("could not locate PortName constants in %s", portsRelPath)
	}

	slices.Sort(names)
	return slices.Compact(names), nil
}

// RouterAllowedPortNames extracts the allowed port names from RouterValidatePortName.
func RouterAllowedPortNames(content []byte) ([]string, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, validationRelPath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse validation file: %w", err)
	}

	trueClause, err := RouterFindValidationSwitchClauses(file)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(trueClause.List))
	for _, expr := range trueClause.List {
		ident, ok := expr.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("unsupported validation case in %s", validationRelPath)
		}
		names = append(names, ident.Name)
	}

	slices.Sort(names)
	return slices.Compact(names), nil
}

// RouterFindPortConstBlock returns the managed PortName const block from ports.go.
func RouterFindPortConstBlock(file *ast.File) (*ast.GenDecl, error) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if ok && RouterValueSpecHasPortNameType(valueSpec) {
				return genDecl, nil
			}
		}
	}

	return nil, fmt.Errorf("could not locate PortName const block in %s", portsRelPath)
}

// RouterFindValidationSwitchClauses returns the managed allow/default clauses.
func RouterFindValidationSwitchClauses(file *ast.File) (*ast.CaseClause, error) {
	for _, decl := range file.Decls {
		funcDecl := RouterValidationFuncDecl(decl)
		if funcDecl == nil {
			continue
		}
		switchStmt, err := RouterValidationSwitchStmt(funcDecl)
		if err != nil {
			return nil, err
		}

		return RouterExtractValidationAllowClause(switchStmt)
	}

	return nil, fmt.Errorf("could not locate RouterValidatePortName in %s", validationRelPath)
}

// RouterValidationFuncDecl returns RouterValidatePortName when decl matches the expected function.
func RouterValidationFuncDecl(decl ast.Decl) *ast.FuncDecl {
	funcDecl, ok := decl.(*ast.FuncDecl)
	if !ok || funcDecl.Name == nil || funcDecl.Name.Name != "RouterValidatePortName" {
		return nil
	}

	return funcDecl
}

// RouterValidationSwitchStmt returns the managed validation switch statement.
func RouterValidationSwitchStmt(funcDecl *ast.FuncDecl) (*ast.SwitchStmt, error) {
	if funcDecl.Body == nil || len(funcDecl.Body.List) != 1 {
		return nil, fmt.Errorf("unsupported RouterValidatePortName body in %s", validationRelPath)
	}

	switchStmt, ok := funcDecl.Body.List[0].(*ast.SwitchStmt)
	if !ok {
		return nil, fmt.Errorf("unsupported RouterValidatePortName shape in %s", validationRelPath)
	}

	if err := RouterValidateValidationSwitchTag(switchStmt.Tag); err != nil {
		return nil, err
	}

	return switchStmt, nil
}

// RouterValidateValidationSwitchTag verifies that RouterValidatePortName switches on `port`.
func RouterValidateValidationSwitchTag(tag ast.Expr) error {
	if tag == nil {
		return fmt.Errorf("unsupported RouterValidatePortName switch tag in %s", validationRelPath)
	}

	tagIdent, ok := tag.(*ast.Ident)
	if !ok || tagIdent.Name != "port" {
		return fmt.Errorf("unsupported RouterValidatePortName switch tag in %s", validationRelPath)
	}

	return nil
}

// RouterExtractValidationAllowClause extracts the managed allow validation case.
func RouterExtractValidationAllowClause(switchStmt *ast.SwitchStmt) (*ast.CaseClause, error) {
	var trueClause *ast.CaseClause
	var defaultClause *ast.CaseClause

	for _, stmt := range switchStmt.Body.List {
		caseClause, err := RouterRequireCaseClause(stmt)
		if err != nil {
			return nil, err
		}

		updatedTrueClause, updatedDefaultClause, err := RouterAssignValidationCaseClause(
			caseClause,
			trueClause,
			defaultClause,
		)
		if err != nil {
			return nil, err
		}

		trueClause = updatedTrueClause
		defaultClause = updatedDefaultClause
	}

	if trueClause == nil || defaultClause == nil {
		return nil, fmt.Errorf("unsupported RouterValidatePortName cases in %s", validationRelPath)
	}

	return trueClause, nil
}

// RouterRequireCaseClause asserts that stmt is a case clause in RouterValidatePortName.
func RouterRequireCaseClause(stmt ast.Stmt) (*ast.CaseClause, error) {
	caseClause, ok := stmt.(*ast.CaseClause)
	if !ok {
		return nil, fmt.Errorf("unsupported RouterValidatePortName case clause in %s", validationRelPath)
	}

	return caseClause, nil
}

// RouterAssignValidationCaseClause validates and assigns one allow/default case clause.
func RouterAssignValidationCaseClause(
	caseClause *ast.CaseClause,
	trueClause *ast.CaseClause,
	defaultClause *ast.CaseClause,
) (*ast.CaseClause, *ast.CaseClause, error) {
	if len(caseClause.List) == 0 {
		if defaultClause != nil || !RouterCaseClauseReturnsBool(caseClause, false) {
			return nil, nil, fmt.Errorf("unsupported RouterValidatePortName default case in %s", validationRelPath)
		}

		return trueClause, caseClause, nil
	}

	if trueClause != nil || !RouterCaseClauseReturnsBool(caseClause, true) {
		return nil, nil, fmt.Errorf("unsupported RouterValidatePortName allow-list case in %s", validationRelPath)
	}

	return caseClause, defaultClause, nil
}

// RouterValueSpecHasPortNameType reports whether a const spec is typed as PortName.
func RouterValueSpecHasPortNameType(valueSpec *ast.ValueSpec) bool {
	ident, ok := valueSpec.Type.(*ast.Ident)
	return ok && ident.Name == "PortName"
}

// RouterCaseClauseReturnsBool reports whether a case clause is a single return of the provided bool value.
func RouterCaseClauseReturnsBool(caseClause *ast.CaseClause, expected bool) bool {
	if len(caseClause.Body) != 1 {
		return false
	}

	returnStmt, ok := caseClause.Body[0].(*ast.ReturnStmt)
	if !ok || len(returnStmt.Results) != 1 {
		return false
	}

	ident, ok := returnStmt.Results[0].(*ast.Ident)
	if !ok {
		return false
	}

	if expected {
		return ident.Name == "true"
	}

	return ident.Name == "false"
}

// RouterPortSetDifference returns the sorted names present in left but missing from right.
func RouterPortSetDifference(left []string, right []string) []string {
	rightSet := make(map[string]struct{}, len(right))
	for _, name := range right {
		rightSet[name] = struct{}{}
	}

	diff := make([]string, 0)
	for _, name := range left {
		if _, exists := rightSet[name]; !exists {
			diff = append(diff, name)
		}
	}

	slices.Sort(diff)
	return slices.Compact(diff)
}

// RouterPrintGoFile prints one parsed Go file back to source form.
func RouterPrintGoFile(fileSet *token.FileSet, file *ast.File) ([]byte, error) {
	var builder strings.Builder
	config := &printer.Config{Mode: printer.TabIndent | printer.UseSpaces, Tabwidth: 8}
	if err := config.Fprint(&builder, fileSet, file); err != nil {
		return nil, fmt.Errorf("print generated Go file: %w", err)
	}

	return []byte(builder.String()), nil
}

// RouterWritePortgenDryRunOutput prints dry-run intent to stdout.
func RouterWritePortgenDryRunOutput(stdout io.Writer, name, value, portsRel, validationRel string) error {
	lines := []string{
		fmt.Sprintf("wrlk dry-run: would add port %s = %q", name, value),
		fmt.Sprintf("  %s — inject: %s PortName = %q", portsRel, name, value),
		fmt.Sprintf("  %s — inject: append %s to RouterValidatePortName", validationRel, name),
		fmt.Sprintf("  %s — rewrite with updated checksums", lockRelPath),
	}

	for _, line := range lines {
		if err := RouterWritePortgenMessage(stdout, "%s\n", line); err != nil {
			return err
		}
	}

	return nil
}

// RouterWritePortgenUsage prints the portgen CLI usage message.
func RouterWritePortgenUsage(w io.Writer) error {
	lines := []string{
		"usage: wrlk [--root PATH] add [flags]",
		"flags:",
		"  --name <ConstantName> --value <string> [--dry-run]",
	}

	for _, line := range lines {
		if err := RouterWritePortgenMessage(w, "%s\n", line); err != nil {
			return err
		}
	}

	return nil
}

// RouterWritePortgenMessage writes one formatted message.
func RouterWritePortgenMessage(w io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return fmt.Errorf("write wrlk message: %w", err)
	}

	return nil
}
