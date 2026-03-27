// internal/router/tools/wrlk/guide_current.go
// Implements the CLI guide generation to describe the currently wired
// router capabilities and application extensions over the console.

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type routerGuideSection struct {
	Title      string
	Extensions []routerGuideExtensionInfo
}

type routerGuideExtensionInfo struct {
	Name       string
	PackageRel string
	Summary    string
	Usage      []string
	Consumes   []string
	Provides   []string
	Required   bool
}

type routerGuideComposition struct {
	Imports    map[string]string
	Extensions []routerGuideCompositionEntry
}

type routerGuideCompositionEntry struct {
	Alias string
}

// RouterRunGuideCommand dispatches router guide subcommands.
func RouterRunGuideCommand(options globalOptions, args []string, stdout io.Writer) error {
	if len(args) == 0 || RouterIsHelpToken(args[0]) {
		return RouterWriteGuide(stdout)
	}

	switch args[0] {
	case "current":
		return RouterWriteCurrentGuide(options.root, stdout)
	case "extension":
		return RouterWriteExtensionGuide(stdout)
	default:
		return &usageError{message: fmt.Sprintf("unknown guide subcommand %q", args[0])}
	}
}

// RouterWriteCurrentGuide prints the currently wired router inventory for the target root.
func RouterWriteCurrentGuide(root string, writer io.Writer) error {
	declaredPorts, err := RouterReadDeclaredPortValues(root)
	if err != nil {
		return fmt.Errorf("read declared ports: %w", err)
	}

	optionalSection, err := RouterReadGuideSection(root, extOptionalRelPath, "Optional capability extensions")
	if err != nil {
		return fmt.Errorf("read optional extension guide: %w", err)
	}

	applicationSection, err := RouterReadGuideSection(root, extApplicationRelPath, "Application adapter extensions")
	if err != nil {
		return fmt.Errorf("read application extension guide: %w", err)
	}

	if _, err := fmt.Fprintln(writer, "Router current guide:"); err != nil {
		return fmt.Errorf("write guide current header: %w", err)
	}
	if _, err := fmt.Fprintln(writer, ""); err != nil {
		return fmt.Errorf("write guide current spacer: %w", err)
	}

	if err := RouterWriteGuidePorts(writer, declaredPorts); err != nil {
		return fmt.Errorf("write guide ports: %w", err)
	}
	if err := RouterWriteGuideSection(writer, optionalSection); err != nil {
		return fmt.Errorf("write optional guide section: %w", err)
	}
	if err := RouterWriteGuideSection(writer, applicationSection); err != nil {
		return fmt.Errorf("write application guide section: %w", err)
	}

	return nil
}

// RouterReadDeclaredPortValues loads the declared ports from ports.go.
func RouterReadDeclaredPortValues(root string) ([]string, error) {
	portsPath := filepath.Join(root, filepath.FromSlash(portsRelPath))
	content, err := os.ReadFile(portsPath)
	if err != nil {
		return nil, fmt.Errorf("read ports file: %w", err)
	}

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, portsRelPath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse ports file: %w", err)
	}

	constBlock, err := RouterFindPortConstBlock(file)
	if err != nil {
		return nil, err
	}

	values := make([]string, 0, len(constBlock.Specs))
	for _, spec := range constBlock.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok || !RouterValueSpecHasPortNameType(valueSpec) || len(valueSpec.Names) == 0 || len(valueSpec.Values) == 0 {
			continue
		}

		basicLit, ok := valueSpec.Values[0].(*ast.BasicLit)
		if !ok || basicLit.Kind != token.STRING {
			continue
		}

		rawValue, err := strconv.Unquote(basicLit.Value)
		if err != nil {
			return nil, fmt.Errorf("parse port value for %s: %w", valueSpec.Names[0].Name, err)
		}

		values = append(values, fmt.Sprintf("%s = %q", valueSpec.Names[0].Name, rawValue))
	}

	slices.Sort(values)
	return values, nil
}

// RouterReadGuideSection reads one extension composition file and the extension packages it wires.
func RouterReadGuideSection(root, compositionRelPath, title string) (routerGuideSection, error) {
	composition, err := RouterReadGuideComposition(root, compositionRelPath)
	if err != nil {
		return routerGuideSection{}, err
	}

	extensions := make([]routerGuideExtensionInfo, 0, len(composition.Extensions))
	for _, entry := range composition.Extensions {
		importPath, exists := composition.Imports[entry.Alias]
		if !exists {
			return routerGuideSection{}, fmt.Errorf("missing import for extension alias %q in %s", entry.Alias, compositionRelPath)
		}

		packageRel, err := RouterImportPathToRelativePath(importPath)
		if err != nil {
			return routerGuideSection{}, fmt.Errorf("resolve extension import %q: %w", importPath, err)
		}

		extensionInfo, err := RouterReadGuideExtensionInfo(root, packageRel)
		if err != nil {
			return routerGuideSection{}, fmt.Errorf("read extension %s: %w", packageRel, err)
		}
		extensions = append(extensions, extensionInfo)
	}

	return routerGuideSection{
		Title:      title,
		Extensions: extensions,
	}, nil
}

// RouterReadGuideComposition parses one extension composition file for its imports and entries.
func RouterReadGuideComposition(root, compositionRelPath string) (routerGuideComposition, error) {
	compositionPath := filepath.Join(root, filepath.FromSlash(compositionRelPath))
	content, err := os.ReadFile(compositionPath)
	if err != nil {
		return routerGuideComposition{}, fmt.Errorf("read composition file: %w", err)
	}

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, compositionRelPath, content, parser.ParseComments)
	if err != nil {
		return routerGuideComposition{}, fmt.Errorf("parse composition file: %w", err)
	}

	imports := make(map[string]string, len(file.Imports))
	for _, importSpec := range file.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			return routerGuideComposition{}, fmt.Errorf("parse import path: %w", err)
		}

		alias := filepath.Base(importPath)
		if importSpec.Name != nil {
			alias = importSpec.Name.Name
		}
		imports[alias] = importPath
	}

	entries := make([]routerGuideCompositionEntry, 0)
	ast.Inspect(file, func(node ast.Node) bool {
		compositeLit, ok := node.(*ast.CompositeLit)
		if !ok {
			return true
		}

		selectorExpr, ok := compositeLit.Type.(*ast.SelectorExpr)
		if !ok || selectorExpr.Sel == nil || selectorExpr.Sel.Name != "Extension" {
			return true
		}

		aliasIdent, ok := selectorExpr.X.(*ast.Ident)
		if !ok {
			return true
		}

		entries = append(entries, routerGuideCompositionEntry{Alias: aliasIdent.Name})
		return true
	})

	return routerGuideComposition{
		Imports:    imports,
		Extensions: entries,
	}, nil
}

// RouterImportPathToRelativePath converts a module import path to a repository-relative path.
func RouterImportPathToRelativePath(importPath string) (string, error) {
	const marker = "/internal/"

	index := strings.Index(importPath, marker)
	if index < 0 {
		return "", fmt.Errorf("import path %q does not point into internal/", importPath)
	}

	return filepath.ToSlash(importPath[index+1:]), nil
}

// RouterReadGuideExtensionInfo reads guide metadata from one extension package.
func RouterReadGuideExtensionInfo(root, packageRel string) (routerGuideExtensionInfo, error) {
	docSummary, usageLines, err := RouterReadGuideDocMetadata(root, packageRel)
	if err != nil {
		return routerGuideExtensionInfo{}, fmt.Errorf("read doc metadata: %w", err)
	}

	required, consumes, provides, err := RouterReadGuideExtensionMethods(root, packageRel)
	if err != nil {
		return routerGuideExtensionInfo{}, fmt.Errorf("read extension methods: %w", err)
	}

	return routerGuideExtensionInfo{
		Name:       filepath.Base(packageRel),
		PackageRel: packageRel,
		Summary:    docSummary,
		Usage:      usageLines,
		Consumes:   consumes,
		Provides:   provides,
		Required:   required,
	}, nil
}

// RouterReadGuideDocMetadata parses the extension package comment and usage notes from doc.go.
func RouterReadGuideDocMetadata(root, packageRel string) (string, []string, error) {
	docPath := filepath.Join(root, filepath.FromSlash(packageRel), "doc.go")
	content, err := os.ReadFile(docPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, nil
		}

		return "", nil, fmt.Errorf("read doc.go: %w", err)
	}

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, docPath, content, parser.ParseComments)
	if err != nil {
		return "", nil, fmt.Errorf("parse doc.go: %w", err)
	}

	if file.Doc == nil {
		return "", nil, nil
	}

	summary, usage := RouterExtractGuideDocSections(file.Doc.Text())
	return summary, usage, nil
}

// RouterExtractGuideDocSections extracts a summary and usage block from package doc text.
func RouterExtractGuideDocSections(docText string) (string, []string) {
	lines := strings.Split(docText, "\n")
	return RouterExtractGuideSummary(lines), RouterExtractGuideUsage(lines)
}

// RouterExtractGuideSummary collects the opening package summary before section headers.
func RouterExtractGuideSummary(lines []string) string {
	summaryParts := make([]string, 0)
	summaryStarted := false
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			if summaryStarted {
				break
			}
			continue
		}

		if line == "Package Concerns:" || line == "Usage:" {
			break
		}

		summaryStarted = true
		summaryParts = append(summaryParts, line)
	}
	return strings.Join(summaryParts, " ")
}

// RouterExtractGuideUsage collects bullet-style usage guidance from the Usage section.
func RouterExtractGuideUsage(lines []string) []string {
	usage := make([]string, 0)
	inUsage := false
	currentUsage := ""
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "Usage:" {
			inUsage = true
			continue
		}

		if !inUsage {
			continue
		}

		if line == "" || RouterGuideUsageReachedNextSection(line) {
			usage = RouterAppendGuideUsageLine(usage, currentUsage)
			currentUsage = ""
			if RouterGuideUsageReachedNextSection(line) {
				break
			}
			continue
		}

		if strings.HasPrefix(line, "- ") {
			usage = RouterAppendGuideUsageLine(usage, currentUsage)
			currentUsage = strings.TrimPrefix(line, "- ")
			continue
		}

		if currentUsage == "" {
			currentUsage = line
			continue
		}

		currentUsage = currentUsage + " " + line
	}

	return RouterAppendGuideUsageLine(usage, currentUsage)
}

// RouterGuideUsageReachedNextSection reports whether parsing has reached a new doc section.
func RouterGuideUsageReachedNextSection(line string) bool {
	return strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "- ")
}

// RouterAppendGuideUsageLine appends a completed usage item when one is present.
func RouterAppendGuideUsageLine(usage []string, currentUsage string) []string {
	if currentUsage == "" {
		return usage
	}

	return append(usage, currentUsage)
}

// RouterReadGuideExtensionMethods parses Required, Consumes, and Provides from extension.go.
func RouterReadGuideExtensionMethods(root, packageRel string) (bool, []string, []string, error) {
	extensionPath := filepath.Join(root, filepath.FromSlash(packageRel), "extension.go")
	content, err := os.ReadFile(extensionPath)
	if err != nil {
		return false, nil, nil, fmt.Errorf("read extension.go: %w", err)
	}

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, extensionPath, content, parser.ParseComments)
	if err != nil {
		return false, nil, nil, fmt.Errorf("parse extension.go: %w", err)
	}

	required := false
	consumes := make([]string, 0)
	provides := make([]string, 0)

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name == nil {
			continue
		}

		nextRequired, nextConsumes, nextProvides, err := RouterApplyGuideExtensionMethod(
			required,
			consumes,
			provides,
			funcDecl,
		)
		if err != nil {
			return false, nil, nil, err
		}

		required = nextRequired
		consumes = nextConsumes
		provides = nextProvides
	}

	slices.Sort(consumes)
	slices.Sort(provides)
	return required, consumes, provides, nil
}

// RouterApplyGuideExtensionMethod applies one extension method declaration to the guide state.
func RouterApplyGuideExtensionMethod(
	required bool,
	consumes []string,
	provides []string,
	funcDecl *ast.FuncDecl,
) (bool, []string, []string, error) {
	switch funcDecl.Name.Name {
	case "Required":
		parsedRequired, err := RouterParseGuideRequiredValue(funcDecl)
		if err != nil {
			return false, nil, nil, err
		}
		return parsedRequired, consumes, provides, nil
	case "Consumes":
		ports, err := RouterParseGuidePortList(funcDecl)
		if err != nil {
			return false, nil, nil, err
		}
		return required, ports, provides, nil
	case "Provides":
		ports, err := RouterParseGuidePortList(funcDecl)
		if err != nil {
			return false, nil, nil, err
		}
		return required, consumes, ports, nil
	default:
		return required, consumes, provides, nil
	}
}

// RouterParseGuideRequiredValue returns the bool literal returned by Required().
func RouterParseGuideRequiredValue(funcDecl *ast.FuncDecl) (bool, error) {
	if funcDecl.Body == nil || len(funcDecl.Body.List) != 1 {
		return false, fmt.Errorf("unsupported Required() shape in %s", funcDecl.Name.Name)
	}

	returnStmt, ok := funcDecl.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(returnStmt.Results) != 1 {
		return false, fmt.Errorf("unsupported Required() return in %s", funcDecl.Name.Name)
	}

	ident, ok := returnStmt.Results[0].(*ast.Ident)
	if !ok {
		return false, fmt.Errorf("unsupported Required() result expression")
	}

	switch ident.Name {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("unsupported Required() bool literal %q", ident.Name)
	}
}

// RouterParseGuidePortList parses a nil or []router.PortName literal returned from Consumes/Provides.
func RouterParseGuidePortList(funcDecl *ast.FuncDecl) ([]string, error) {
	if funcDecl.Body == nil || len(funcDecl.Body.List) != 1 {
		return nil, fmt.Errorf("unsupported %s() shape", funcDecl.Name.Name)
	}

	returnStmt, ok := funcDecl.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(returnStmt.Results) != 1 {
		return nil, fmt.Errorf("unsupported %s() return", funcDecl.Name.Name)
	}

	switch expr := returnStmt.Results[0].(type) {
	case *ast.Ident:
		if expr.Name == "nil" {
			return nil, nil
		}
	case *ast.CompositeLit:
		ports := make([]string, 0, len(expr.Elts))
		for _, elt := range expr.Elts {
			selectorExpr, ok := elt.(*ast.SelectorExpr)
			if !ok || selectorExpr.Sel == nil {
				return nil, fmt.Errorf("unsupported %s() port entry", funcDecl.Name.Name)
			}
			ports = append(ports, selectorExpr.Sel.Name)
		}
		return ports, nil
	}

	return nil, fmt.Errorf("unsupported %s() result expression", funcDecl.Name.Name)
}

// RouterWriteGuidePorts prints the declared router port list.
func RouterWriteGuidePorts(writer io.Writer, declaredPorts []string) error {
	if _, err := fmt.Fprintln(writer, "Declared ports:"); err != nil {
		return fmt.Errorf("write declared ports heading: %w", err)
	}

	if len(declaredPorts) == 0 {
		if _, err := fmt.Fprintln(writer, "  - none declared"); err != nil {
			return fmt.Errorf("write empty declared ports: %w", err)
		}
		if _, err := fmt.Fprintln(writer, ""); err != nil {
			return fmt.Errorf("write declared ports spacer: %w", err)
		}
		return nil
	}

	for _, port := range declaredPorts {
		if _, err := fmt.Fprintf(writer, "  - %s\n", port); err != nil {
			return fmt.Errorf("write declared port %q: %w", port, err)
		}
	}

	if _, err := fmt.Fprintln(writer, ""); err != nil {
		return fmt.Errorf("write declared ports spacer: %w", err)
	}

	return nil
}

// RouterWriteGuideSection prints one extension category section.
func RouterWriteGuideSection(writer io.Writer, section routerGuideSection) error {
	if _, err := fmt.Fprintf(writer, "%s:\n", section.Title); err != nil {
		return fmt.Errorf("write section title %q: %w", section.Title, err)
	}

	if len(section.Extensions) == 0 {
		if _, err := fmt.Fprintln(writer, "  - none wired"); err != nil {
			return fmt.Errorf("write empty section %q: %w", section.Title, err)
		}
		if _, err := fmt.Fprintln(writer, ""); err != nil {
			return fmt.Errorf("write section spacer %q: %w", section.Title, err)
		}
		return nil
	}

	for _, extensionInfo := range section.Extensions {
		if err := RouterWriteGuideExtension(writer, extensionInfo); err != nil {
			return fmt.Errorf("write extension %s: %w", extensionInfo.Name, err)
		}
	}

	if _, err := fmt.Fprintln(writer, ""); err != nil {
		return fmt.Errorf("write section spacer %q: %w", section.Title, err)
	}

	return nil
}

// RouterWriteGuideExtension prints one extension inventory entry.
func RouterWriteGuideExtension(writer io.Writer, extensionInfo routerGuideExtensionInfo) error {
	requiredLabel := "optional"
	if extensionInfo.Required {
		requiredLabel = "required"
	}

	if _, err := fmt.Fprintf(
		writer,
		"  - %s (%s)\n",
		extensionInfo.Name,
		requiredLabel,
	); err != nil {
		return fmt.Errorf("write extension heading: %w", err)
	}

	lines := []string{
		fmt.Sprintf("    package: %s", extensionInfo.PackageRel),
		fmt.Sprintf("    provides: %s", RouterFormatGuidePortList(extensionInfo.Provides)),
		fmt.Sprintf("    consumes: %s", RouterFormatGuidePortList(extensionInfo.Consumes)),
	}

	if extensionInfo.Summary != "" {
		lines = append(lines, fmt.Sprintf("    summary: %s", extensionInfo.Summary))
	}

	for _, usageLine := range extensionInfo.Usage {
		lines = append(lines, fmt.Sprintf("    usage: %s", usageLine))
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write extension detail line: %w", err)
		}
	}

	return nil
}

// RouterFormatGuidePortList formats a port name list for guide output.
func RouterFormatGuidePortList(ports []string) string {
	if len(ports) == 0 {
		return "none"
	}

	return strings.Join(ports, ", ")
}
