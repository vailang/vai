// Package render produces markdown output from compiled vai AST.
//
// It depends only on the ast package and receives all external data
// (target file symbols, prompt/plan indexes) through the TargetInfo
// interface and pre-built maps. This keeps rendering independently
// testable with mocks — no coder or composer imports.
package render

import "github.com/vailang/vai/internal/compiler/ast"

// TargetInfo provides target file symbol data for rendering.
// Satisfied by compiler.targetResolverImpl without adapters.
type TargetInfo interface {
	// ResolveTarget returns symbols and signatures from a target file.
	ResolveTarget(path string) (symbols map[string]ast.SymbolKind, sigs map[string]string, err error)
	// GetCode returns the source code for a named symbol in a target file.
	GetCode(path, name string) (string, bool)
	// GetSkeleton returns the file structure with empty bodies.
	GetSkeleton(path string) (string, bool)
	// GetDoc returns the documentation comment for a named symbol.
	GetDoc(path, name string) (string, bool)
}
