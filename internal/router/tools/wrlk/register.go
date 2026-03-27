// internal/router/tools/wrlk/register.go
// Implements manifest-backed scaffolding and code generation for port
// and extension wiring.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

const (
	routerManifestRelPath     = "internal/router/router_manifest.go"
	appManifestRelPath        = "internal/router/ext/app_manifest.go"
	routerPortsRelPath        = "internal/router/ports.go"
	routerRegistryRelPath     = "internal/router/registry_imports.go"
	optionalExtensionsRelPath = "internal/router/ext/optional_extensions.go"
	applicationExtensionsPath = "internal/router/ext/extensions.go"
)

type registerOptions struct {
	registerPort   bool
	registerExt    bool
	registerRouter bool
	registerApp    bool
	name           string
	value          string
	dryRun         bool
}

type portManifestEntry struct {
	Name  string
	Value string
}

type optionalExtensionManifestEntry struct {
	Name string
}

type applicationExtensionManifestEntry struct {
	Name string
}

// RouterRunRegisterCommand runs the manifest-backed register command.
func RouterRunRegisterCommand(options globalOptions, args []string, stdout io.Writer) error {
	if len(args) > 0 && RouterIsHelpToken(args[0]) {
		return RouterWriteRegisterUsage(stdout)
	}

	registerFlags, err := RouterParseRegisterFlags(args)
	if err != nil {
		if err == flag.ErrHelp {
			return RouterWriteRegisterUsage(stdout)
		}

		return &usageError{message: err.Error()}
	}

	if err := RouterExecuteRegister(options.root, registerFlags, stdout); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	return nil
}

// RouterParseRegisterFlags parses register command selectors and payload.
func RouterParseRegisterFlags(args []string) (registerOptions, error) {
	options := registerOptions{}

	fs := flag.NewFlagSet("wrlk register", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&options.registerPort, "port", false, "register a router port")
	fs.BoolVar(&options.registerExt, "ext", false, "register an extension")
	fs.BoolVar(&options.registerRouter, "router", false, "target router-owned wiring")
	fs.BoolVar(&options.registerApp, "app", false, "target app-owned wiring")
	fs.StringVar(&options.name, "name", "", "port or extension name")
	fs.StringVar(&options.value, "value", "", "port value")
	fs.BoolVar(&options.dryRun, "dry-run", false, "print intended changes without writing")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return registerOptions{}, err
		}

		return registerOptions{}, fmt.Errorf("parse register flags: %w", err)
	}
	if len(fs.Args()) > 0 {
		return registerOptions{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}

	if options.registerPort == options.registerExt {
		return registerOptions{}, fmt.Errorf("exactly one of --port or --ext must be set")
	}
	if options.name == "" {
		return registerOptions{}, fmt.Errorf("--name is required")
	}

	if options.registerPort {
		if !options.registerRouter || options.registerApp {
			return registerOptions{}, fmt.Errorf("--port requires --router and cannot be used with --app")
		}
		if options.value == "" {
			return registerOptions{}, fmt.Errorf("--value is required with --port")
		}

		return options, nil
	}

	if options.registerRouter == options.registerApp {
		return registerOptions{}, fmt.Errorf("--ext requires exactly one of --router or --app")
	}
	if options.value != "" {
		return registerOptions{}, fmt.Errorf("--value is only valid with --port")
	}
	if err := RouterValidateExtensionName(options.name); err != nil {
		return registerOptions{}, err
	}

	return options, nil
}

// RouterWriteRegisterUsage prints the register command usage message.
func RouterWriteRegisterUsage(writer io.Writer) error {
	lines := []string{
		"usage: Router [--root PATH] register --port|--ext --router|--app --name <name> [flags]",
		"flags:",
		"  --port      register a router port declaration",
		"  --ext       register an extension declaration",
		"  --router    target router-owned manifests and generated wiring",
		"  --app       target app-owned extension manifests",
		"  --name      port or extension name",
		"  --value     port value; required with --port",
		"  --dry-run   print intended changes without writing",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write register usage line: %w", err)
		}
	}

	return nil
}

// RouterExecuteRegister applies one manifest-backed router mutation.
func RouterExecuteRegister(root string, options registerOptions, stdout io.Writer) (returnErr error) {
	if options.registerExt {
		if err := RouterEnsureRegisterExtensionTargetExists(root, options); err != nil {
			return err
		}
	}

	if options.dryRun {
		return RouterWriteRegisterDryRun(root, options, stdout)
	}

	reason := RouterRegisterSnapshotReason(options)
	snapshot, err := RouterCaptureSnapshot(root, reason)
	if err != nil {
		return fmt.Errorf("capture snapshot: %w", err)
	}

	if err := RouterWriteSnapshot(root, snapshot); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	defer func() {
		if returnErr == nil {
			return
		}

		if restoreErr := RouterRestoreSnapshot(root, snapshot); restoreErr != nil {
			returnErr = fmt.Errorf("%w (restore snapshot: %v)", returnErr, restoreErr)
		}
	}()

	if err := RouterAppendRegisterManifestEntry(root, options); err != nil {
		return err
	}

	if err := RouterGenerateRegisterOutputs(root, options); err != nil {
		return err
	}

	if err := RouterWriteLockAfterPortgen(root); err != nil {
		return fmt.Errorf("refresh router lock: %w", err)
	}

	if _, err := fmt.Fprintf(stdout, "register: updated manifests for %s\n", reason); err != nil {
		return fmt.Errorf("write register success message: %w", err)
	}

	return nil
}

// RouterRegisterSnapshotReason returns the user-facing mutation summary stored in the snapshot.
func RouterRegisterSnapshotReason(options registerOptions) string {
	if options.registerPort {
		return fmt.Sprintf(
			"wrlk register --port --router --name %s --value %s",
			options.name,
			options.value,
		)
	}

	if options.registerRouter {
		return fmt.Sprintf("wrlk register --ext --router --name %s", options.name)
	}

	return fmt.Sprintf("wrlk register --ext --app --name %s", options.name)
}

// RouterWriteRegisterDryRun prints the manifest and generated files affected by a register call.
func RouterWriteRegisterDryRun(root string, options registerOptions, stdout io.Writer) error {
	manifestPath, _, _ := RouterRegisterManifestTarget(root, options)
	generatedFiles := []string{routerPortsRelPath, routerRegistryRelPath}
	if options.registerExt && options.registerRouter {
		generatedFiles = append(generatedFiles, optionalExtensionsRelPath)
	}
	if options.registerExt && options.registerApp {
		generatedFiles = []string{applicationExtensionsPath}
	}

	if _, err := fmt.Fprintf(stdout, "register dry-run: %s\n", RouterRegisterSnapshotReason(options)); err != nil {
		return fmt.Errorf("write register dry-run header: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "manifest: %s\n", manifestPath); err != nil {
		return fmt.Errorf("write register dry-run manifest: %w", err)
	}
	for _, relativePath := range generatedFiles {
		if _, err := fmt.Fprintf(stdout, "would generate: %s\n", relativePath); err != nil {
			return fmt.Errorf("write register dry-run output %s: %w", relativePath, err)
		}
	}

	return nil
}

// RouterEnsureRegisterExtensionTargetExists rejects extension registrations that would generate broken imports.
func RouterEnsureRegisterExtensionTargetExists(root string, options registerOptions) error {
	extensionPath := RouterRegisterExtensionPath(root, options)
	info, err := os.Stat(extensionPath)
	if err != nil {
		return fmt.Errorf("extension package %s: %w", extensionPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("extension path %s is not a directory", extensionPath)
	}

	return nil
}

// RouterRegisterExtensionPath returns the package directory that must exist for extension registration.
func RouterRegisterExtensionPath(root string, options registerOptions) string {
	if options.registerApp {
		return filepath.Join(root, filepath.FromSlash(adapterRelDir), options.name)
	}

	return filepath.Join(root, filepath.FromSlash(extExtensionsRelDir), options.name)
}

// RouterAppendRegisterManifestEntry appends one new manifest entry to the correct manifest file.
func RouterAppendRegisterManifestEntry(root string, options registerOptions) error {
	manifestPath, variableName, entryFields := RouterRegisterManifestTarget(root, options)
	if err := RouterAppendManifestEntry(manifestPath, variableName, entryFields); err != nil {
		return fmt.Errorf("append manifest entry to %s: %w", manifestPath, err)
	}

	return nil
}

// RouterRegisterManifestTarget resolves the manifest target path, slice name, and field payload.
func RouterRegisterManifestTarget(root string, options registerOptions) (string, string, map[string]string) {
	if options.registerPort {
		return filepath.Join(root, filepath.FromSlash(routerManifestRelPath)), "DeclaredPorts", map[string]string{
			"Name":  options.name,
			"Value": options.value,
		}
	}

	if options.registerRouter {
		return filepath.Join(root, filepath.FromSlash(routerManifestRelPath)), "DeclaredOptionalExtensions", map[string]string{
			"Name": options.name,
		}
	}

	return filepath.Join(root, filepath.FromSlash(appManifestRelPath)), "DeclaredApplicationExtensions", map[string]string{
		"Name": options.name,
	}
}

// RouterAppendManifestEntry adds a keyed struct literal to a manifest slice.
func RouterAppendManifestEntry(filePath, variableName string, entryFields map[string]string) error {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filePath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse manifest file: %w", err)
	}

	compositeLiteral, findErr := RouterFindManifestCompositeLiteral(file, variableName)
	if findErr != nil {
		return findErr
	}

	if validateErr := RouterValidateManifestEntryUniqueness(compositeLiteral, variableName, entryFields); validateErr != nil {
		return validateErr
	}

	compositeLiteral.Elts = append(compositeLiteral.Elts, RouterBuildManifestCompositeLiteral(entryFields))

	formattedSource, err := RouterFormatASTFile(fileSet, file)
	if err != nil {
		return fmt.Errorf("format manifest file: %w", err)
	}

	if err := os.WriteFile(filePath, formattedSource, 0o644); err != nil {
		return fmt.Errorf("write manifest file: %w", err)
	}

	return nil
}

// RouterFindManifestCompositeLiteral locates the composite literal bound to the named manifest variable.
func RouterFindManifestCompositeLiteral(file *ast.File, variableName string) (*ast.CompositeLit, error) {
	for _, declaration := range file.Decls {
		compositeLiteral, found, err := RouterFindManifestCompositeLiteralInDecl(declaration, variableName)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		return compositeLiteral, nil
	}

	return nil, fmt.Errorf("manifest variable %s not found", variableName)
}

// RouterFindManifestCompositeLiteralInDecl searches one declaration for the target manifest variable.
func RouterFindManifestCompositeLiteralInDecl(
	declaration ast.Decl,
	variableName string,
) (*ast.CompositeLit, bool, error) {
	genDecl, ok := declaration.(*ast.GenDecl)
	if !ok || genDecl.Tok != token.VAR {
		return nil, false, nil
	}

	for _, spec := range genDecl.Specs {
		compositeLiteral, found, err := RouterFindManifestCompositeLiteralInSpec(spec, variableName)
		if err != nil {
			return nil, false, err
		}
		if found {
			return compositeLiteral, true, nil
		}
	}

	return nil, false, nil
}

// RouterFindManifestCompositeLiteralInSpec searches one value spec for the target manifest variable.
func RouterFindManifestCompositeLiteralInSpec(
	spec ast.Spec,
	variableName string,
) (*ast.CompositeLit, bool, error) {
	valueSpec, ok := spec.(*ast.ValueSpec)
	if !ok {
		return nil, false, nil
	}

	for index, name := range valueSpec.Names {
		if name.Name != variableName || index >= len(valueSpec.Values) {
			continue
		}

		compositeLiteral, ok := valueSpec.Values[index].(*ast.CompositeLit)
		if !ok {
			return nil, false, fmt.Errorf("manifest variable %s is not a slice literal", variableName)
		}

		return compositeLiteral, true, nil
	}

	return nil, false, nil
}

// RouterValidateManifestEntryUniqueness rejects duplicate names and port values.
func RouterValidateManifestEntryUniqueness(
	compositeLiteral *ast.CompositeLit,
	variableName string,
	entryFields map[string]string,
) error {
	for _, element := range compositeLiteral.Elts {
		if err := RouterValidateManifestEntryElement(element, variableName, entryFields); err != nil {
			return err
		}
	}

	return nil
}

// RouterValidateManifestEntryElement validates one existing manifest entry against the requested fields.
func RouterValidateManifestEntryElement(
	element ast.Expr,
	variableName string,
	entryFields map[string]string,
) error {
	existingFields, err := RouterReadManifestEntryFields(element)
	if err != nil {
		return err
	}

	if duplicateName := entryFields["Name"]; duplicateName != "" && existingFields["Name"] == duplicateName {
		return fmt.Errorf("entry with name %q already exists in %s", duplicateName, variableName)
	}
	if duplicateValue := entryFields["Value"]; duplicateValue != "" && existingFields["Value"] == duplicateValue {
		return fmt.Errorf("port value %q already exists in %s", duplicateValue, variableName)
	}

	return nil
}

// RouterReadManifestEntryFields extracts keyed string fields from one manifest element.
func RouterReadManifestEntryFields(element ast.Expr) (map[string]string, error) {
	compositeLiteral, ok := element.(*ast.CompositeLit)
	if !ok {
		return nil, fmt.Errorf("manifest entry is not a composite literal")
	}

	fields := make(map[string]string, len(compositeLiteral.Elts))
	for _, rawField := range compositeLiteral.Elts {
		keyValue, ok := rawField.(*ast.KeyValueExpr)
		if !ok {
			return nil, fmt.Errorf("manifest entry contains a non-keyed field")
		}

		keyIdent, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("manifest entry field key is not an identifier")
		}

		valueLiteral, ok := keyValue.Value.(*ast.BasicLit)
		if !ok || valueLiteral.Kind != token.STRING {
			return nil, fmt.Errorf("manifest entry field %s is not a string literal", keyIdent.Name)
		}

		value, err := strconv.Unquote(valueLiteral.Value)
		if err != nil {
			return nil, fmt.Errorf("parse manifest field %s: %w", keyIdent.Name, err)
		}
		fields[keyIdent.Name] = value
	}

	return fields, nil
}

// RouterBuildManifestCompositeLiteral constructs one keyed manifest element in stable field order.
func RouterBuildManifestCompositeLiteral(entryFields map[string]string) *ast.CompositeLit {
	fieldOrder := []string{"Name", "Value"}
	fields := make([]ast.Expr, 0, len(entryFields))
	for _, fieldName := range fieldOrder {
		value, exists := entryFields[fieldName]
		if !exists {
			continue
		}

		fields = append(fields, &ast.KeyValueExpr{
			Key: ast.NewIdent(fieldName),
			Value: &ast.BasicLit{
				Kind:  token.STRING,
				Value: strconv.Quote(value),
			},
		})
	}

	return &ast.CompositeLit{Elts: fields}
}

// RouterFormatASTFile prints and formats one Go file from an AST.
func RouterFormatASTFile(fileSet *token.FileSet, file *ast.File) ([]byte, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fileSet, file); err != nil {
		return nil, fmt.Errorf("print Go file: %w", err)
	}

	formattedSource, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format Go file: %w", err)
	}

	return formattedSource, nil
}

// RouterGenerateRegisterOutputs regenerates router-owned static outputs from manifests.
func RouterGenerateRegisterOutputs(root string, options registerOptions) error {
	if options.registerPort || options.registerRouter {
		if err := RouterGeneratePortsGo(root); err != nil {
			return err
		}
		if err := RouterGenerateRegistryImportsGo(root); err != nil {
			return err
		}
	}
	if options.registerExt && options.registerRouter {
		if err := RouterGenerateOptionalExtensionsGo(root); err != nil {
			return err
		}
	}
	if options.registerExt && options.registerApp {
		if err := RouterGenerateApplicationExtensionsGo(root); err != nil {
			return err
		}
	}

	return nil
}

// RouterGeneratePortsGo regenerates ports.go from the router manifest.
func RouterGeneratePortsGo(root string) error {
	const templateSource = `package router

// PortName is a typed router port identifier.
type PortName string

// Provider is the registered implementation for a router port.
type Provider any

const (
{{- range . }}
	{{ .Name }} PortName = "{{ .Value }}"
{{- end }}
)
`

	declaredPorts, err := RouterLoadDeclaredPorts(root)
	if err != nil {
		return fmt.Errorf("load declared ports: %w", err)
	}

	if err := RouterGenerateFileFromTemplate(
		filepath.Join(root, filepath.FromSlash(routerPortsRelPath)),
		templateSource,
		declaredPorts,
	); err != nil {
		return fmt.Errorf("generate %s: %w", routerPortsRelPath, err)
	}

	return nil
}

// RouterGenerateRegistryImportsGo regenerates the port validation whitelist from the router manifest.
func RouterGenerateRegistryImportsGo(root string) error {
	const templateSource = `package router

import "sync/atomic"

type routerRegistrySnapshot struct {
	providers    map[PortName]Provider
	restrictions map[PortName][]string
}

var registry atomic.Pointer[routerRegistrySnapshot]

// RouterValidatePortName reports whether the port is declared in the router whitelist.
func RouterValidatePortName(port PortName) bool {
	switch port {
{{- range . }}
	case {{ .Name }}:
		return true
{{- end }}
	default:
		return false
	}
}
`

	declaredPorts, err := RouterLoadDeclaredPorts(root)
	if err != nil {
		return fmt.Errorf("load declared ports: %w", err)
	}

	if err := RouterGenerateFileFromTemplate(
		filepath.Join(root, filepath.FromSlash(routerRegistryRelPath)),
		templateSource,
		declaredPorts,
	); err != nil {
		return fmt.Errorf("generate %s: %w", routerRegistryRelPath, err)
	}

	return nil
}

// RouterGenerateOptionalExtensionsGo regenerates optional router extension wiring from the router manifest.
func RouterGenerateOptionalExtensionsGo(root string) error {
	modulePath, err := RouterReadModulePath(root)
	if err != nil {
		return fmt.Errorf("read module path: %w", err)
	}

	const templateSource = `// Package ext provides the router extension wiring and boot orchestration.
//
// Package Concerns:
// - This package must never import internal packages that depend on router; ext handles all adapter wiring.
// - Callers boot via ext.RouterBootExtensions and resolve via RouterResolveProvider.
//
// # Optional Extensions
//
// Optional extensions are capability extensions that extend the base router without adding
// dependencies to the core router code. They load before application extensions during boot
// and are ideal for adding cross-cutting concerns like telemetry, logging, or metrics.
//
// To add a new optional extension:
//   1. Create the extension in internal/router/ext/extensions/<name>/
//   2. Reference it in the optionalExtensions slice below
//   3. Optional extensions that fail to load produce warnings but do not halt boot
//
// # Required Extensions
//
// Required extensions are core functionality that must be present for the router to operate.
// They are defined in extensions.go and boot failure is fatal if they fail to load.
package ext

import (
	"{{ .ModulePath }}/internal/router"
{{- range .Extensions }}
	"{{ $.ModulePath }}/internal/router/ext/extensions/{{ .Name }}"
{{- end }}
)

// optionalExtensions is the slice of capability extensions that extend the base router
// without adding dependencies to the core router code. These extensions load before
// application extensions during boot and are optional - boot continues with warnings
// if they fail to load.
var optionalExtensions = []router.Extension{
{{- range .Extensions }}
	&{{ .Name }}.Extension{},
{{- end }}
}
`

	declaredExtensions, err := RouterLoadDeclaredOptionalExtensions(root)
	if err != nil {
		return fmt.Errorf("load declared optional extensions: %w", err)
	}

	templateData := struct {
		ModulePath string
		Extensions []optionalExtensionManifestEntry
	}{
		ModulePath: modulePath,
		Extensions: declaredExtensions,
	}

	if err := RouterGenerateFileFromTemplate(
		filepath.Join(root, filepath.FromSlash(optionalExtensionsRelPath)),
		templateSource,
		templateData,
	); err != nil {
		return fmt.Errorf("generate %s: %w", optionalExtensionsRelPath, err)
	}

	return nil
}

// RouterGenerateApplicationExtensionsGo regenerates app-owned extension wiring from the app manifest.
func RouterGenerateApplicationExtensionsGo(root string) error {
	modulePath, err := RouterReadModulePath(root)
	if err != nil {
		return fmt.Errorf("read module path: %w", err)
	}

	const templateSource = `package ext

import (
	"context"
	"fmt"
	"os"
	"strings"

	"{{ .ModulePath }}/internal/router"
{{- range .Extensions }}
	"{{ $.ModulePath }}/internal/adapters/{{ .Name }}"
{{- end }}
)

// extensions contains generated required application extensions from app_manifest.go.
// It may be empty when the host application has no required adapters to boot here.
var extensions = []router.Extension{
{{- range .Extensions }}
	&{{ .Name }}.Extension{},
{{- end }}
}

const (
	wrlkEnvKey        = "WRLK_ENV"
	routerProfileKey  = "ROUTER_PROFILE"
	routerAllowAnyKey = "ROUTER_ALLOW_ANY"
)

// RouterBuildExtensionBundle returns the compiled-in optional and application extension bundles.
func RouterBuildExtensionBundle() ([]router.Extension, []router.Extension) {
	return append([]router.Extension(nil), optionalExtensions...), append([]router.Extension(nil), extensions...)
}

// RouterBootExtensions wires optional extensions first, then application extensions.
func RouterBootExtensions(ctx context.Context) ([]error, error) {
	if err := validateRouterBootPolicy(); err != nil {
		return nil, err
	}

	optionalBundle, applicationBundle := RouterBuildExtensionBundle()
	return router.RouterLoadExtensions(optionalBundle, applicationBundle, ctx)
}

// validateRouterBootPolicy rejects router profile combinations that are unsafe at boot.
func validateRouterBootPolicy() error {
	runtimeEnv := normalizeRouterProfile(os.Getenv(wrlkEnvKey))
	declaredProfile := normalizeRouterProfile(os.Getenv(routerProfileKey))

	if declaredProfile != "" && runtimeEnv != "" && declaredProfile != runtimeEnv {
		return &router.RouterError{
			Code: router.RouterEnvironmentMismatch,
			Err: fmt.Errorf(
				"%s=%q does not match %s=%q",
				routerProfileKey,
				declaredProfile,
				wrlkEnvKey,
				runtimeEnv,
			),
		}
	}

	if runtimeEnv == "prod" && parseRouterBoolEnv(os.Getenv(routerAllowAnyKey)) {
		return &router.RouterError{
			Code: router.RouterProfileInvalid,
			Err: fmt.Errorf(
				"%s=true is not allowed when %s=%q",
				routerAllowAnyKey,
				wrlkEnvKey,
				runtimeEnv,
			),
		}
	}

	return nil
}

// normalizeRouterProfile trims and lowercases a router profile value for stable comparison.
func normalizeRouterProfile(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// parseRouterBoolEnv reports whether an environment value should be treated as enabled.
func parseRouterBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}
`

	declaredExtensions, err := RouterLoadDeclaredApplicationExtensions(root)
	if err != nil {
		return fmt.Errorf("load declared app extensions: %w", err)
	}

	templateData := struct {
		ModulePath string
		Extensions []applicationExtensionManifestEntry
	}{
		ModulePath: modulePath,
		Extensions: declaredExtensions,
	}

	if err := RouterGenerateFileFromTemplate(
		filepath.Join(root, filepath.FromSlash(applicationExtensionsPath)),
		templateSource,
		templateData,
	); err != nil {
		return fmt.Errorf("generate %s: %w", applicationExtensionsPath, err)
	}

	return nil
}

// RouterGenerateFileFromTemplate executes a deterministic template and formats the result as Go source.
func RouterGenerateFileFromTemplate(filePath, templateSource string, data any) error {
	var builder strings.Builder
	tmpl, err := template.New(filepath.Base(filePath)).Parse(templateSource)
	if err != nil {
		return fmt.Errorf("parse generation template: %w", err)
	}

	if executeErr := tmpl.Execute(&builder, data); executeErr != nil {
		return fmt.Errorf("execute generation template: %w", executeErr)
	}

	formattedSource, err := format.Source([]byte(builder.String()))
	if err != nil {
		return fmt.Errorf("format generated Go source: %w", err)
	}

	if err := os.WriteFile(filePath, formattedSource, 0o644); err != nil {
		return fmt.Errorf("write generated file: %w", err)
	}

	return nil
}

// RouterLoadDeclaredPorts reads ordered port declarations from the router manifest.
func RouterLoadDeclaredPorts(root string) ([]portManifestEntry, error) {
	entries, err := RouterLoadManifestEntries(
		filepath.Join(root, filepath.FromSlash(routerManifestRelPath)),
		"DeclaredPorts",
	)
	if err != nil {
		return nil, err
	}

	declaredPorts := make([]portManifestEntry, 0, len(entries))
	for _, entryFields := range entries {
		declaredPorts = append(declaredPorts, portManifestEntry{
			Name:  entryFields["Name"],
			Value: entryFields["Value"],
		})
	}

	return declaredPorts, nil
}

// RouterLoadDeclaredOptionalExtensions reads ordered optional router extension declarations from the router manifest.
func RouterLoadDeclaredOptionalExtensions(root string) ([]optionalExtensionManifestEntry, error) {
	entries, err := RouterLoadManifestEntries(
		filepath.Join(root, filepath.FromSlash(routerManifestRelPath)),
		"DeclaredOptionalExtensions",
	)
	if err != nil {
		return nil, err
	}

	declaredExtensions := make([]optionalExtensionManifestEntry, 0, len(entries))
	for _, entryFields := range entries {
		declaredExtensions = append(declaredExtensions, optionalExtensionManifestEntry{
			Name: entryFields["Name"],
		})
	}

	return declaredExtensions, nil
}

// RouterLoadDeclaredApplicationExtensions reads ordered app extension declarations from the app manifest.
func RouterLoadDeclaredApplicationExtensions(root string) ([]applicationExtensionManifestEntry, error) {
	entries, err := RouterLoadManifestEntries(
		filepath.Join(root, filepath.FromSlash(appManifestRelPath)),
		"DeclaredApplicationExtensions",
	)
	if err != nil {
		return nil, err
	}

	declaredExtensions := make([]applicationExtensionManifestEntry, 0, len(entries))
	for _, entryFields := range entries {
		declaredExtensions = append(declaredExtensions, applicationExtensionManifestEntry{
			Name: entryFields["Name"],
		})
	}

	return declaredExtensions, nil
}

// RouterLoadManifestEntries reads keyed string field entries from one manifest slice literal.
func RouterLoadManifestEntries(filePath, variableName string) ([]map[string]string, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse manifest file %s: %w", filePath, err)
	}

	compositeLiteral, err := RouterFindManifestCompositeLiteral(file, variableName)
	if err != nil {
		return nil, err
	}

	entries := make([]map[string]string, 0, len(compositeLiteral.Elts))
	for _, element := range compositeLiteral.Elts {
		entryFields, err := RouterReadManifestEntryFields(element)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entryFields)
	}

	return entries, nil
}
