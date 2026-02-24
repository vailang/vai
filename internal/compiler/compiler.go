package compiler

import (
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/vailang/vai/internal/coder"
	"github.com/vailang/vai/internal/compiler/ast"
	"github.com/vailang/vai/internal/compiler/composer"
	"github.com/vailang/vai/internal/compiler/lexer"
	"github.com/vailang/vai/internal/compiler/parser"
	"github.com/vailang/vai/internal/compiler/reader"
	"github.com/vailang/vai/std"
)

// compiler implements the Compiler interface.
type compiler struct {
	baseDir string // project root (vai.toml dir); empty = use source file dir
}

// New creates a new Compiler.
func New() Compiler {
	return &compiler{}
}

// SetBaseDir sets the project root for relative path resolution.
func (c *compiler) SetBaseDir(dir string) {
	c.baseDir = dir
}

// Parse reads .vai/.plan files from path (file or directory) and compiles
// them through the full pipeline: read → lex → parse → compose → program.
func (c *compiler) Parse(path string) (Program, []error) {
	sources, err := reader.ReadPaths(path)
	if err != nil {
		return &program{}, []error{err}
	}
	if len(sources) == 0 {
		return &program{}, []error{fmt.Errorf("no .vai or .plan files found in %s", path)}
	}

	return c.ParseSources(sources)
}

// ParseSources compiles multiple vai sources into a single program.
func (c *compiler) ParseSources(sources map[string]string) (Program, []error) {
	// Sort paths for deterministic ordering.
	paths := make([]string, 0, len(sources))
	for p := range sources {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	// Lex and parse each file individually.
	var files []*ast.File
	var errs []error
	for _, filePath := range paths {
		cs := reader.NewVaiSource(sources[filePath])
		scanner := lexer.NewScanner(cs)
		p := parser.New(scanner)
		file, parseErrs := p.ParseFile()
		for _, pe := range parseErrs {
			errs = append(errs, pe)
		}
		file.SourcePath = filePath
		files = append(files, file)
	}
	if len(errs) > 0 {
		return &program{}, errs
	}

	// Load standard library prompts.
	stdPrompts, stdErrs := loadStdPrompts()
	if len(stdErrs) > 0 {
		return &program{}, stdErrs
	}

	// Build knownPrompts map for composer validation of std.X references.
	knownPrompts := make(map[string]bool, len(stdPrompts))
	for name := range stdPrompts {
		knownPrompts[name] = true
	}

	// Create a target resolver that lazily loads coders on demand.
	tr := newTargetResolver()

	astSrc := &fileSource{files: files}
	comp := composer.New(astSrc, nil, tr, knownPrompts, c.baseDir)
	compErrs := comp.Validate()

	// Separate warnings from errors — only errors block compilation.
	var warnings []error
	for _, ce := range compErrs {
		if ce.IsWarning() {
			warnings = append(warnings, ce)
		} else {
			errs = append(errs, ce)
		}
	}
	if len(errs) > 0 {
		tr.Close()
		return &program{}, errs
	}

	// Merge all files for program execution.
	merged, mergeErr := ast.MergeFiles(files)
	if mergeErr != nil {
		tr.Close()
		return &program{}, []error{mergeErr}
	}

	requests := comp.Requests()
	return &program{file: merged, requests: requests, targetResolver: tr, stdPrompts: stdPrompts, projectDir: c.baseDir, warnings: warnings}, nil
}

// loadStdPrompts parses the embedded standard library .vai files and returns
// a map of qualified prompt names (e.g. "std.planner_role") to their AST declarations.
func loadStdPrompts() (map[string]*ast.PromptDecl, []error) {
	prompts := make(map[string]*ast.PromptDecl)

	err := fs.WalkDir(std.Standard, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".vai") {
			return nil
		}

		content, err := fs.ReadFile(std.Standard, path)
		if err != nil {
			return err
		}

		cs := reader.NewVaiSource(string(content))
		scanner := lexer.NewScanner(cs)
		p := parser.New(scanner)
		file, parseErrs := p.ParseFile()
		if len(parseErrs) > 0 {
			return parseErrs[0]
		}

		for _, decl := range file.Declarations {
			if pd, ok := decl.(*ast.PromptDecl); ok {
				prompts["std."+pd.Name] = pd
			}
		}
		return nil
	})
	if err != nil {
		return nil, []error{fmt.Errorf("loading std library: %w", err)}
	}

	return prompts, nil
}

// ---------------------------------------------------------------------------
// targetResolverImpl — lazily loads coders with caching
// ---------------------------------------------------------------------------

type targetResolverImpl struct {
	coders   map[string]*coder.Coder // absolute path → loaded coder
	rawFiles map[string]string       // absolute path → raw content (non-code files)
}

func newTargetResolver() *targetResolverImpl {
	return &targetResolverImpl{
		coders:   make(map[string]*coder.Coder),
		rawFiles: make(map[string]string),
	}
}

func (r *targetResolverImpl) getOrCreate(path string) (*coder.Coder, error) {
	if cod, ok := r.coders[path]; ok {
		return cod, nil
	}

	lang, err := coder.DetectLanguage(path)
	if err != nil {
		// Non-code file (e.g. go.mod, Cargo.toml) — read raw content instead.
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, nil // file doesn't exist, no symbols and no raw content
		}
		r.rawFiles[path] = string(src)
		return nil, nil // no coder, but not an error
	}

	cod, err := coder.New(lang, path)
	if err != nil {
		return nil, err
	}

	src, err := os.ReadFile(path)
	if err != nil {
		// Target file doesn't exist yet — coder with no symbols.
		r.coders[path] = cod
		return cod, nil
	}

	if err := cod.Load(src); err != nil {
		cod.Close()
		return nil, fmt.Errorf("loading symbols from %s: %w", path, err)
	}

	r.coders[path] = cod
	return cod, nil
}

func (r *targetResolverImpl) ResolveTarget(path string) (map[string]ast.SymbolKind, map[string]string, error) {
	cod, err := r.getOrCreate(path)
	if err != nil {
		return nil, nil, err
	}
	if cod == nil {
		// Non-code file — no symbols to extract.
		return map[string]ast.SymbolKind{}, map[string]string{}, nil
	}

	raw := cod.Symbols()
	symbols := make(map[string]ast.SymbolKind, len(raw))
	sigs := make(map[string]string, len(raw))
	for name, kind := range raw {
		symbols[name] = ast.SymbolKind(kind)
		if resolved, ok := cod.Resolve(name); ok {
			sigs[name] = resolved.Signature
		}
	}
	return symbols, sigs, nil
}

func (r *targetResolverImpl) GetCode(path, name string) (string, bool) {
	cod, ok := r.coders[path]
	if !ok {
		return "", false
	}
	if resolved, ok := cod.Resolve(name); ok {
		return resolved.Code, resolved.Code != ""
	}
	return "", false
}

func (r *targetResolverImpl) GetSkeleton(path string) (string, bool) {
	cod, err := r.getOrCreate(path)
	if err != nil || cod == nil {
		return "", false
	}
	skeleton, err := cod.Skeleton()
	if err != nil {
		return "", false
	}
	return skeleton, skeleton != ""
}

func (r *targetResolverImpl) GetRawContent(path string) (string, bool) {
	content, ok := r.rawFiles[path]
	return content, ok
}

func (r *targetResolverImpl) GetDoc(path, name string) (string, bool) {
	cod, ok := r.coders[path]
	if !ok {
		return "", false
	}
	if resolved, ok := cod.Resolve(name); ok {
		return resolved.Doc, resolved.Doc != ""
	}
	return "", false
}

func (r *targetResolverImpl) Close() {
	for _, cod := range r.coders {
		cod.Close()
	}
}

// ---------------------------------------------------------------------------
// fileSource — implements composer.ASTSource
// ---------------------------------------------------------------------------

type fileSource struct {
	files []*ast.File
}

func (f *fileSource) Files() []*ast.File {
	return f.files
}
