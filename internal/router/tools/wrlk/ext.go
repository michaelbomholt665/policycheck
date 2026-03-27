// internal/router/tools/wrlk/ext.go
// Defines commands and logic for scaffolding, installing, and removing
// router capabilities and application extensions.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	extOptionalRelPath    = "internal/router/ext/optional_extensions.go"
	extApplicationRelPath = "internal/router/ext/extensions.go"
	extExtensionsRelDir   = "internal/router/ext/extensions"
	adapterRelDir         = "internal/adapters"
)

type extCommandOptions struct {
	name   string
	dryRun bool
}

type extCommandSpec struct {
	commandPath          string
	compositionRelPath   string
	compositionVarName   string
	importRelDir         string
	description          string
	dryRunTarget         string
	required             bool
	createPackage        bool
	requireExisting      bool
	snapshotReasonPrefix string
	successVerb          string
}

var (
	optionalScaffoldSpec = extCommandSpec{
		commandPath:          "add",
		compositionRelPath:   extOptionalRelPath,
		compositionVarName:   "optionalExtensions",
		importRelDir:         extExtensionsRelDir,
		description:          "router capability extension",
		dryRunTarget:         "optional_extensions.go",
		required:             false,
		createPackage:        true,
		requireExisting:      false,
		snapshotReasonPrefix: "wrlk ext add --name",
		successVerb:          "added",
	}
	optionalInstallSpec = extCommandSpec{
		commandPath:          "install",
		compositionRelPath:   extOptionalRelPath,
		compositionVarName:   "optionalExtensions",
		importRelDir:         extExtensionsRelDir,
		description:          "router capability extension",
		dryRunTarget:         "optional_extensions.go",
		required:             false,
		createPackage:        false,
		requireExisting:      true,
		snapshotReasonPrefix: "wrlk ext install --name",
		successVerb:          "wired",
	}
	optionalRemoveSpec = extCommandSpec{
		commandPath:          "remove",
		compositionRelPath:   extOptionalRelPath,
		compositionVarName:   "optionalExtensions",
		importRelDir:         extExtensionsRelDir,
		description:          "router capability extension",
		dryRunTarget:         "optional_extensions.go",
		required:             false,
		createPackage:        false,
		requireExisting:      false,
		snapshotReasonPrefix: "wrlk ext remove --name",
		successVerb:          "removed",
	}
	applicationInstallSpec = extCommandSpec{
		commandPath:          "app add",
		compositionRelPath:   extApplicationRelPath,
		compositionVarName:   "extensions",
		importRelDir:         adapterRelDir,
		description:          "application adapter extension",
		dryRunTarget:         "extensions.go",
		required:             true,
		createPackage:        false,
		requireExisting:      true,
		snapshotReasonPrefix: "wrlk ext app add --name",
		successVerb:          "wired",
	}
	applicationRemoveSpec = extCommandSpec{
		commandPath:          "app remove",
		compositionRelPath:   extApplicationRelPath,
		compositionVarName:   "extensions",
		importRelDir:         adapterRelDir,
		description:          "application adapter extension",
		dryRunTarget:         "extensions.go",
		required:             true,
		createPackage:        false,
		requireExisting:      false,
		snapshotReasonPrefix: "wrlk ext app remove --name",
		successVerb:          "removed",
	}
)

// RouterRunExtCommand dispatches the `ext` command group.
func RouterRunExtCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 || RouterIsHelpToken(args[0]) {
		return RouterWriteExtUsage(stdout)
	}

	switch args[0] {
	case "add":
		return RouterRunExtAddCommand(options, args[1:], stdout, stderr)
	case "install":
		return RouterRunExtInstallCommand(options, args[1:], stdout, stderr)
	case "remove":
		return RouterRunExtRemoveCommand(options, args[1:], stdout, stderr)
	case "app":
		return RouterRunExtAppCommand(options, args[1:], stdout, stderr)
	default:
		return &usageError{message: fmt.Sprintf("unknown ext subcommand %q", args[0])}
	}
}

// RouterWriteExtUsage prints the ext command usage message.
func RouterWriteExtUsage(writer io.Writer) error {
	lines := []string{
		"usage: Router [--root PATH] ext <subcommand>",
		"subcommands:",
		"  add       scaffold a new optional router capability extension package",
		"  install   wire an existing optional router capability extension",
		"  remove    remove an optional router capability extension from optional_extensions.go",
		"  app add    wire an existing application adapter into extensions.go",
		"  app remove  remove an application adapter from extensions.go",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write ext usage line: %w", err)
		}
	}

	return nil
}

// RouterRunExtAddCommand parses flags and scaffolds a new optional extension.
func RouterRunExtAddCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	return RouterRunExtensionMutationCommand(options, args, stdout, stderr, optionalScaffoldSpec, RouterPerformExtensionAdd)
}

// RouterRunExtInstallCommand parses flags and wires an existing optional extension.
func RouterRunExtInstallCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	return RouterRunExtensionMutationCommand(options, args, stdout, stderr, optionalInstallSpec, RouterPerformExtensionAdd)
}

// RouterRunExtRemoveCommand parses flags and removes an optional extension wiring.
func RouterRunExtRemoveCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	return RouterRunExtensionMutationCommand(options, args, stdout, stderr, optionalRemoveSpec, RouterPerformExtensionRemove)
}

// RouterRunExtAppCommand dispatches the `ext app` command group.
func RouterRunExtAppCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 || RouterIsHelpToken(args[0]) {
		return RouterWriteExtAppUsage(stdout)
	}

	switch args[0] {
	case "add":
		return RouterRunExtAppAddCommand(options, args[1:], stdout, stderr)
	case "remove":
		return RouterRunExtAppRemoveCommand(options, args[1:], stdout, stderr)
	default:
		return &usageError{message: fmt.Sprintf("unknown ext app subcommand %q", args[0])}
	}
}

// RouterWriteExtAppUsage prints the ext app usage message.
func RouterWriteExtAppUsage(writer io.Writer) error {
	lines := []string{
		"usage: Router [--root PATH] ext app <subcommand>",
		"subcommands:",
		"  add      wire an existing application adapter into extensions.go",
		"  remove   remove an application adapter from extensions.go",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write ext app usage line: %w", err)
		}
	}

	return nil
}

// RouterRunExtAppAddCommand parses flags and wires an application adapter.
func RouterRunExtAppAddCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	return RouterRunExtensionMutationCommand(options, args, stdout, stderr, applicationInstallSpec, RouterPerformExtensionAdd)
}

// RouterRunExtAppRemoveCommand parses flags and removes an application adapter wiring.
func RouterRunExtAppRemoveCommand(options globalOptions, args []string, stdout io.Writer, stderr io.Writer) error {
	return RouterRunExtensionMutationCommand(options, args, stdout, stderr, applicationRemoveSpec, RouterPerformExtensionRemove)
}

type extMutationFunc func(string, string, string, bool, io.Writer, extCommandSpec) error

// RouterRunExtensionMutationCommand parses flags and runs one ext mutation variant.
func RouterRunExtensionMutationCommand(
	options globalOptions,
	args []string,
	stdout io.Writer,
	_ io.Writer,
	spec extCommandSpec,
	mutation extMutationFunc,
) error {
	if len(args) > 0 && RouterIsHelpToken(args[0]) {
		return RouterWriteExtensionMutationUsage(stdout, spec)
	}

	commandOptions, err := RouterParseExtCommandFlags(args, spec)
	if err != nil {
		return &usageError{message: err.Error()}
	}

	modulePath, err := RouterReadModulePath(options.root)
	if err != nil {
		return fmt.Errorf("ext %s: %w", spec.commandPath, err)
	}

	if err := mutation(options.root, modulePath, commandOptions.name, commandOptions.dryRun, stdout, spec); err != nil {
		return fmt.Errorf("ext %s: %w", spec.commandPath, err)
	}

	return nil
}

// RouterWriteExtensionMutationUsage prints one ext mutation usage message.
func RouterWriteExtensionMutationUsage(writer io.Writer, spec extCommandSpec) error {
	lines := []string{
		fmt.Sprintf("usage: Router [--root PATH] ext %s [flags]", spec.commandPath),
		"flags:",
		fmt.Sprintf("  --name <ExtensionName>  package name for the %s", spec.description),
		"  --dry-run               print changes without writing",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write ext usage line: %w", err)
		}
	}

	return nil
}

// RouterParseExtCommandFlags parses the flags for ext mutation commands.
func RouterParseExtCommandFlags(args []string, spec extCommandSpec) (extCommandOptions, error) {
	options := extCommandOptions{}

	fs := flag.NewFlagSet("wrlk ext "+spec.commandPath, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&options.name, "name", "", "package name for the extension")
	fs.BoolVar(&options.dryRun, "dry-run", false, "print changes without writing")

	if err := fs.Parse(args); err != nil {
		return extCommandOptions{}, fmt.Errorf("parse ext %s flags: %w", spec.commandPath, err)
	}

	if options.name == "" {
		return extCommandOptions{}, fmt.Errorf("--name is required")
	}

	if err := RouterValidateExtensionName(options.name); err != nil {
		return extCommandOptions{}, err
	}

	return options, nil
}

// RouterValidateExtensionName returns an error if name is not a valid Go package name.
func RouterValidateExtensionName(name string) error {
	if name == "" {
		return fmt.Errorf("extension name must not be empty")
	}

	for index, value := range name {
		if index == 0 && !unicode.IsLetter(value) {
			return fmt.Errorf("extension name %q must start with a letter", name)
		}

		if !unicode.IsLetter(value) && !unicode.IsDigit(value) {
			return fmt.Errorf("extension name %q must contain only letters and digits (got %q)", name, value)
		}
	}

	return nil
}

// RouterPerformExtensionAdd scaffolds or wires one extension.
func RouterPerformExtensionAdd(
	root string,
	modulePath string,
	name string,
	dryRun bool,
	stdout io.Writer,
	spec extCommandSpec,
) error {
	compositionPath := filepath.Join(root, filepath.FromSlash(spec.compositionRelPath))
	compositionContent, readErr := os.ReadFile(compositionPath)
	if readErr != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(spec.compositionRelPath), readErr)
	}

	if err := RouterEnsureExtensionExistsForWiring(root, name, spec); err != nil {
		return err
	}

	updatedComposition, err := RouterInjectExtension(compositionContent, name, modulePath, spec)
	if err != nil {
		return fmt.Errorf("inject extension into %s: %w", filepath.Base(spec.compositionRelPath), err)
	}

	if dryRun {
		if err := RouterWriteExtensionDryRunOutput(stdout, name, modulePath, spec); err != nil {
			return fmt.Errorf("write dry-run output: %w", err)
		}

		return nil
	}

	if err := RouterWriteExtSnapshotBeforeMutation(root, fmt.Sprintf("%s %s", spec.snapshotReasonPrefix, name)); err != nil {
		return fmt.Errorf("write snapshot before ext mutation: %w", err)
	}

	if spec.createPackage {
		if err := RouterWriteOptionalExtensionPackage(root, modulePath, name, spec); err != nil {
			return err
		}
	}

	if err := RouterAtomicWriteFile(compositionPath, updatedComposition); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(spec.compositionRelPath), err)
	}

	return RouterWriteExtensionSuccess(stdout, name, spec)
}

// RouterPerformExtensionRemove removes one extension from a composition file.
func RouterPerformExtensionRemove(
	root string,
	modulePath string,
	name string,
	dryRun bool,
	stdout io.Writer,
	spec extCommandSpec,
) error {
	compositionPath := filepath.Join(root, filepath.FromSlash(spec.compositionRelPath))
	compositionContent, err := os.ReadFile(compositionPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(spec.compositionRelPath), err)
	}

	updatedComposition, err := RouterRemoveExtension(compositionContent, name, modulePath, spec)
	if err != nil {
		return fmt.Errorf("remove extension from %s: %w", filepath.Base(spec.compositionRelPath), err)
	}

	if dryRun {
		if err := RouterWriteExtensionDryRunOutput(stdout, name, modulePath, spec); err != nil {
			return fmt.Errorf("write dry-run output: %w", err)
		}

		return nil
	}

	if err := RouterWriteExtSnapshotBeforeMutation(root, fmt.Sprintf("%s %s", spec.snapshotReasonPrefix, name)); err != nil {
		return fmt.Errorf("write snapshot before ext mutation: %w", err)
	}

	if err := RouterAtomicWriteFile(compositionPath, updatedComposition); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(spec.compositionRelPath), err)
	}

	return RouterWriteExtensionSuccess(stdout, name, spec)
}

// RouterEnsureExtensionExistsForWiring validates that existing-package wiring targets exist.
func RouterEnsureExtensionExistsForWiring(root, name string, spec extCommandSpec) error {
	if !spec.requireExisting {
		return nil
	}

	targetPath := filepath.Join(root, filepath.FromSlash(spec.importRelDir), name)
	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s %q does not exist at %s", spec.description, name, filepath.ToSlash(filepath.Join(spec.importRelDir, name)))
		}

		return fmt.Errorf("stat existing %s %q: %w", spec.description, name, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s %q path is not a directory: %s", spec.description, name, filepath.ToSlash(filepath.Join(spec.importRelDir, name)))
	}

	return nil
}

// RouterWriteOptionalExtensionPackage writes a newly scaffolded optional extension package.
func RouterWriteOptionalExtensionPackage(root, modulePath, name string, spec extCommandSpec) error {
	extDir := filepath.Join(root, filepath.FromSlash(extExtensionsRelDir), name)
	if _, err := os.Stat(extDir); err == nil {
		return fmt.Errorf("extension %q already exists at %s", name, filepath.ToSlash(filepath.Join(extExtensionsRelDir, name)))
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat extension directory %s: %w", filepath.ToSlash(filepath.Join(extExtensionsRelDir, name)), err)
	}

	docPath := filepath.Join(extDir, "doc.go")
	docContent := RouterExtDocTemplate(name, spec)
	if err := RouterAtomicWriteFile(docPath, []byte(docContent)); err != nil {
		return fmt.Errorf("write extension doc.go for %s: %w", name, err)
	}

	extensionPath := filepath.Join(extDir, "extension.go")
	extensionContent := RouterExtExtensionTemplate(name, modulePath, spec)
	if err := RouterAtomicWriteFile(extensionPath, []byte(extensionContent)); err != nil {
		return fmt.Errorf("write extension.go for %s: %w", name, err)
	}

	return nil
}

// RouterInjectExtension splices a new import and entry into one extension composition file.
func RouterInjectExtension(content []byte, name string, modulePath string, spec extCommandSpec) ([]byte, error) {
	src := string(content)
	importPath := RouterComposeExtensionImportPath(modulePath, name, spec)

	importClosingIdx := strings.Index(src, "\n)")
	if importClosingIdx < 0 {
		return nil, fmt.Errorf("could not locate import block closing paren")
	}

	importLine := fmt.Sprintf("\t%q", importPath)
	if strings.Contains(src, importLine+"\n") || strings.Contains(src, importLine+"\r\n") {
		return nil, fmt.Errorf("extension %q is already imported in %s", name, spec.compositionVarName)
	}

	withImport := src[:importClosingIdx] + "\n" + importLine + src[importClosingIdx:]

	sliceMarker := fmt.Sprintf("var %s = []router.Extension{", spec.compositionVarName)
	sliceStartIdx := strings.Index(withImport, sliceMarker)
	if sliceStartIdx < 0 {
		return nil, fmt.Errorf("could not locate %s slice", spec.compositionVarName)
	}

	closingBrace, err := RouterFindSliceLiteralClosingBrace(withImport, sliceStartIdx, sliceMarker)
	if err != nil {
		return nil, err
	}

	entryLine := fmt.Sprintf("\t&%s.Extension{},", name)
	if strings.Contains(withImport, entryLine+"\n") || strings.Contains(withImport, entryLine+"\r\n") {
		return nil, fmt.Errorf("extension %q is already wired in %s", name, spec.compositionVarName)
	}

	result := RouterInsertExtensionEntry(withImport, closingBrace, entryLine)
	return []byte(result), nil
}

// RouterRemoveExtension removes one managed import and slice entry from a composition file.
func RouterRemoveExtension(content []byte, name string, modulePath string, spec extCommandSpec) ([]byte, error) {
	src := string(content)
	importLine := fmt.Sprintf("\t%q", RouterComposeExtensionImportPath(modulePath, name, spec))
	entryLine := fmt.Sprintf("\t&%s.Extension{},", name)

	if !strings.Contains(src, importLine+"\n") && !strings.Contains(src, importLine+"\r\n") {
		return nil, fmt.Errorf("extension %q is not imported in %s", name, spec.compositionVarName)
	}

	if !strings.Contains(src, entryLine+"\n") && !strings.Contains(src, entryLine+"\r\n") {
		return nil, fmt.Errorf("extension %q is not wired in %s", name, spec.compositionVarName)
	}

	withoutImport := strings.Replace(src, "\n"+importLine, "", 1)
	if withoutImport == src {
		withoutImport = strings.Replace(src, "\r\n"+importLine, "", 1)
	}

	withoutEntry := strings.Replace(withoutImport, "\n"+entryLine, "", 1)
	if withoutEntry == withoutImport {
		withoutEntry = strings.Replace(withoutImport, "\r\n"+entryLine, "", 1)
	}

	return []byte(withoutEntry), nil
}

// RouterComposeExtensionImportPath returns the import path for one extension wiring target.
func RouterComposeExtensionImportPath(modulePath, name string, spec extCommandSpec) string {
	return modulePath + "/" + strings.ReplaceAll(spec.importRelDir, "\\", "/") + "/" + name
}

// RouterFindSliceLiteralClosingBrace returns the closing brace index for one managed extension slice literal.
func RouterFindSliceLiteralClosingBrace(src string, sliceStartIdx int, sliceMarker string) (int, error) {
	openBraceIdx := sliceStartIdx + strings.LastIndex(sliceMarker, "{")
	depth := 0

	for index := openBraceIdx; index < len(src); index++ {
		switch src[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index, nil
			}
		}
	}

	return 0, fmt.Errorf("could not locate %s slice closing brace", sliceMarker)
}

// RouterInsertExtensionEntry inserts one extension entry before the slice closing brace.
func RouterInsertExtensionEntry(src string, closingBrace int, entryLine string) string {
	before := src[:closingBrace]
	after := src[closingBrace:]

	if strings.HasSuffix(before, "{") {
		return before + "\n" + entryLine + "\n" + after
	}

	return before + "\n" + entryLine + after
}

// RouterReadModulePath reads the module path from go.mod in the given root.
func RouterReadModulePath(root string) (_ string, returnErr error) {
	goModPath := filepath.Join(root, "go.mod")

	file, err := os.Open(goModPath)
	if err != nil {
		return "", fmt.Errorf("open go.mod: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close go.mod: %w", closeErr)
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
}

// RouterExtDocTemplate returns the doc.go content for a new router extension.
func RouterExtDocTemplate(name string, spec extCommandSpec) string {
	comment := "Failure to boot results in an OptionalExtensionFailed warning; boot continues."
	if spec.required {
		comment = "Boot failure is fatal because application extensions are required."
	}

	return fmt.Sprintf(`// Package %s is a %s.
//
// Usage:
//   - Describe what this extension provides and when a consumer should depend on it.
//
// Package Concerns:
//   - Implements router.Extension only; keep wiring explicit.
//   - %s
package %s
`, name, spec.description, comment, name)
}

// RouterExtExtensionTemplate returns the extension.go content for a new router extension.
func RouterExtExtensionTemplate(name, modulePath string, spec extCommandSpec) string {
	requiredValue := "false"
	requiredComment := "optional"
	if spec.required {
		requiredValue = "true"
		requiredComment = "required"
	}

	portSuffix := RouterUppercaseFirst(name)

	return fmt.Sprintf(`package %s

import (
	"log"

	"%s/internal/router"
)

// Extension is the %s for %s.
type Extension struct{}

// Required reports that the %s extension is %s.
func (e *Extension) Required() bool {
	return %s
}

// Consumes reports the ports this extension requires before it can boot.
func (e *Extension) Consumes() []router.PortName {
	return nil
}

// Provides reports the ports this extension registers during boot.
func (e *Extension) Provides() []router.PortName {
	return nil
}

// RouterProvideRegistration registers the %s provider into the boot registry.
func (e *Extension) RouterProvideRegistration(_ *router.Registry) error {
	// TODO: replace with the actual port and provider.
	// Example:
	//   if err := reg.RouterRegisterProvider(router.Port%s, yourProvider); err != nil {
	//       return fmt.Errorf("%s extension: %%w", err)
	//   }
	log.Printf("%s extension initialized")

	return nil
}
`, name, modulePath, spec.description, name, name, requiredComment, requiredValue, name, portSuffix, name, name)
}

// RouterWriteExtensionDryRunOutput prints dry-run intent to stdout.
func RouterWriteExtensionDryRunOutput(stdout io.Writer, name string, modulePath string, spec extCommandSpec) error {
	importPath := RouterComposeExtensionImportPath(modulePath, name, spec)
	extRelPath := filepath.ToSlash(filepath.Join(extExtensionsRelDir, name))

	lines := []string{
		fmt.Sprintf("wrlk dry-run: would %s %s %q", spec.successVerb, spec.description, name),
		fmt.Sprintf("  %s - %s import %q and &%s.Extension{}", spec.compositionRelPath, spec.successVerb, importPath, name),
	}

	if spec.createPackage {
		lines = []string{
			fmt.Sprintf("wrlk dry-run: would %s %s %q", spec.successVerb, spec.description, name),
			fmt.Sprintf("  %s - create extension package directory", extRelPath),
			fmt.Sprintf("  %s/doc.go - create package doc", extRelPath),
			fmt.Sprintf("  %s/extension.go - create Extension struct implementing router.Extension", extRelPath),
			fmt.Sprintf("  %s - wire import %q and &%s.Extension{}", spec.compositionRelPath, importPath, name),
		}
	}

	if spec.successVerb == "removed" {
		lines = []string{
			fmt.Sprintf("wrlk dry-run: would remove %s %q", spec.description, name),
			fmt.Sprintf("  %s - remove import %q and &%s.Extension{}", spec.compositionRelPath, importPath, name),
		}
	}

	for _, line := range lines {
		if err := RouterWriteExtMessage(stdout, "%s\n", line); err != nil {
			return err
		}
	}

	return nil
}

// RouterWriteExtensionSuccess writes the command success message.
func RouterWriteExtensionSuccess(stdout io.Writer, name string, spec extCommandSpec) error {
	if spec.createPackage {
		return RouterWriteExtMessage(
			stdout,
			"wrlk: added %s %q at %s\n",
			spec.description,
			name,
			filepath.ToSlash(filepath.Join(extExtensionsRelDir, name)),
		)
	}

	return RouterWriteExtMessage(
		stdout,
		"wrlk: %s %s %q in %s\n",
		spec.successVerb,
		spec.description,
		name,
		spec.compositionRelPath,
	)
}

// RouterWriteExtMessage writes one formatted ext message.
func RouterWriteExtMessage(writer io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(writer, format, args...); err != nil {
		return fmt.Errorf("write wrlk ext message: %w", err)
	}

	return nil
}

// RouterUppercaseFirst uppercases the first rune in a scaffold name for examples.
func RouterUppercaseFirst(value string) string {
	if value == "" {
		return value
	}

	runes := []rune(value)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// RouterWriteExtSnapshotBeforeMutation captures router core files and extension
// composition files before an ext mutation.
func RouterWriteExtSnapshotBeforeMutation(root, reason string) error {
	extSnapshotFiles := append(
		append([]string(nil), snapshotRouterFiles...),
		extOptionalRelPath,
		extApplicationRelPath,
	)
	snapshot, err := RouterCaptureNamedSnapshot(root, reason, extSnapshotFiles)
	if err != nil {
		return err
	}

	if err := RouterWriteSnapshot(root, snapshot); err != nil {
		return err
	}

	return nil
}

// RouterCaptureNamedSnapshot builds a snapshot for an explicit list of relative file paths.
func RouterCaptureNamedSnapshot(root, reason string, files []string) (routerMutationSnapshot, error) {
	snapshotFiles := make([]routerMutationSnapshotFile, 0, len(files))

	for _, relativePath := range files {
		absolutePath := filepath.Join(root, filepath.FromSlash(relativePath))
		content, err := os.ReadFile(absolutePath)
		if err != nil {
			if os.IsNotExist(err) {
				snapshotFiles = append(snapshotFiles, routerMutationSnapshotFile{
					File:   relativePath,
					Exists: false,
				})
				continue
			}

			return routerMutationSnapshot{}, fmt.Errorf("read ext snapshot file %s: %w", relativePath, err)
		}

		snapshotFiles = append(snapshotFiles, routerMutationSnapshotFile{
			File:    relativePath,
			Exists:  true,
			Content: string(content),
		})
	}

	return routerMutationSnapshot{
		CreatedAt: RouterSnapshotTimestamp(),
		Reason:    reason,
		Files:     snapshotFiles,
	}, nil
}
