package compiler

import (
	"github.com/vailang/vai/internal/compiler/ast"
	"github.com/vailang/vai/internal/compiler/composer"
)

// Compiler is the main entry point for the vai compilation pipeline.
type Compiler interface {
	// Parse reads .vai files from path, validates, and returns a compiled Program.
	Parse(vaiPath string) (Program, []error)

	// ParseSources compiles pre-loaded vai sources into a single program.
	ParseSources(sources map[string]string) (Program, []error)

	// SetBaseDir sets the project root for relative path resolution.
	// When set, target/reference paths resolve relative to this directory
	// instead of relative to each source file's directory.
	SetBaseDir(dir string)
}

// Program represents a compiled vai program ready for inspection and execution.
type Program interface {
	// Execution
	Tasks() int                          // Number of compiled requests
	Exec() (string, error)               // Resolve all inject declarations
	Eval(source string) (string, error)  // Evaluate vai source in this program's context
	Render() string                      // Full structured markdown output

	// Diagnostics
	Warnings() []error                   // Non-fatal warnings from compilation

	// Inspection
	File() *ast.File                     // Merged AST
	Requests() []composer.Request        // Compiled requests
	HasPrompt(name string) bool
	HasPlan(name string) bool
	ListPrompts() []string
	ListConstraints() []string
	GetPlanSpec(name string) string
	GetPlanImpl(name string) []string
}
