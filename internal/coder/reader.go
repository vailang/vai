package coder

import (
	"fmt"
	"strings"

	"github.com/vailang/vai/internal/coder/api"
)

// Load parses source code and stores the symbols for later access.
// After calling Load, Resolve() can be used to look up symbols by name.
func (c *Coder) Load(source []byte) error {
	symbols, err := c.query.ReadSymbols(source)
	if err != nil {
		return fmt.Errorf("parsing symbols: %w", err)
	}
	c.source = source
	c.symbols = symbols
	c.loaded = true
	return nil
}

// Resolve looks up a symbol by name and returns its resolved information.
// Supports bare names ("my_func") and qualified names ("MyStruct.Method").
// Returns all extracted data: kind, signature, full code block, and documentation.
// Requires Load() to have been called.
func (c *Coder) Resolve(name string) (api.ResolvedSymbol, bool) {
	for _, sym := range c.symbols {
		if sym.Name == name {
			return c.resolveSymbol(sym), true
		}
		for _, m := range sym.Methods {
			if m.Name == name || sym.Name+"."+m.Name == name {
				return c.resolveMethod(m), true
			}
		}
	}
	return api.ResolvedSymbol{}, false
}

func (c *Coder) resolveSymbol(sym Symbol) api.ResolvedSymbol {
	var code string
	if sym.StartByte >= 0 && sym.EndByte <= len(c.source) && sym.StartByte < sym.EndByte {
		code = string(c.source[sym.StartByte:sym.EndByte])
	}
	return api.ResolvedSymbol{
		Kind:      string(sym.Kind),
		Signature: sym.Signature,
		Code:      code,
		Doc:       sym.Doc,
		StartByte: sym.StartByte,
		EndByte:   sym.EndByte,
	}
}

func (c *Coder) resolveMethod(m Method) api.ResolvedSymbol {
	var code string
	if m.StartByte >= 0 && m.EndByte <= len(c.source) && m.StartByte < m.EndByte {
		code = string(c.source[m.StartByte:m.EndByte])
	}
	return api.ResolvedSymbol{
		Kind:      string(SymbolFunction),
		Signature: m.Signature,
		Code:      code,
		Doc:       m.Doc,
		StartByte: m.StartByte,
		EndByte:   m.EndByte,
	}
}

// Skeleton returns the file source with implementation bodies replaced by stubs.
// Requires Load() to have been called.
func (c *Coder) Skeleton() (string, error) {
	if !c.loaded {
		return "", fmt.Errorf("Skeleton() called before Load()")
	}
	return c.query.ReadSkeleton(c.source)
}

// Symbols returns a map of symbol names to their kinds.
// Includes methods as "Parent.Method" entries.
// Requires Load() to have been called.
func (c *Coder) Symbols() map[string]string {
	m := make(map[string]string, len(c.symbols))
	for _, sym := range c.symbols {
		m[sym.Name] = string(sym.Kind)
		for _, meth := range sym.Methods {
			m[sym.Name+"."+meth.Name] = string(SymbolFunction)
		}
	}
	return m
}

// ParseSymbols extracts all top-level symbols from source code.
func (c *Coder) ParseSymbols(source []byte) ([]Symbol, error) {
	return c.query.ReadSymbols(source)
}

// FindImportZone detects the import block in source code.
// Strips vai:output zones before parsing so only host imports are found.
func (c *Coder) FindImportZone(source []byte) (*ImportZone, error) {
	cleanSource := stripOutputZones(source)
	return c.query.ReadImportZone(cleanSource)
}

// StripLeadingImports removes import lines from the beginning of a code block.
// Returns the cleaned code and the extracted import lines.
func (c *Coder) StripLeadingImports(code string) (string, []string) {
	lines := strings.Split(code, "\n")
	var imports []string
	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if c.IsImportLine(line) {
			imports = append(imports, line)
			i++
			continue
		}
		break
	}
	if len(imports) == 0 {
		return code, nil
	}
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	return strings.Join(lines[i:], "\n"), imports
}

// IsImportLine returns true if the line looks like an import statement.
func (c *Coder) IsImportLine(line string) bool {
	line = strings.TrimSpace(line)
	switch c.lang {
	case C:
		return strings.HasPrefix(line, "#include")
	case Go:
		return strings.HasPrefix(line, "import ") || line == "import ("
	case Rust:
		return strings.HasPrefix(line, "use ")
	case Python:
		return strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "from ")
	case TypeScript:
		return strings.HasPrefix(line, "import ")
	default:
		return false
	}
}
