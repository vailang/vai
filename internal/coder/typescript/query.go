package typescript

import (
	"strings"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
	coder "github.com/vailang/vai/internal/coder/api"
)

type Query struct {
	parser *tree_sitter.Parser
}

// New creates a TypeScript query. Pass tsx=true for .tsx files.
func New(tsx bool) (*Query, error) {
	p := tree_sitter.NewParser()

	var langPtr unsafe.Pointer
	if tsx {
		langPtr = tree_sitter_typescript.LanguageTSX()
	} else {
		langPtr = tree_sitter_typescript.LanguageTypescript()
	}

	lang := tree_sitter.NewLanguage(langPtr)
	if err := p.SetLanguage(lang); err != nil {
		p.Close()
		return nil, err
	}
	return &Query{parser: p}, nil
}

func (q *Query) Close() { q.parser.Close() }

func (q *Query) ReadSymbols(source []byte) ([]coder.Symbol, error) {
	tree := q.parser.Parse(source, nil)
	if tree == nil {
		return nil, nil
	}
	defer tree.Close()

	root := tree.RootNode()
	var symbols []coder.Symbol

	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "function_declaration":
			sym := extractFunction(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}

		case "class_declaration":
			sym := extractClass(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}

		case "interface_declaration":
			sym := extractInterface(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}

		case "export_statement":
			syms := extractExport(child, source)
			symbols = append(symbols, syms...)

		case "type_alias_declaration":
			sym := extractTypeAlias(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}

		case "enum_declaration":
			sym := extractEnum(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}
		}
	}

	return symbols, nil
}

func (q *Query) ReadImportZone(source []byte) (*coder.ImportZone, error) {
	tree := q.parser.Parse(source, nil)
	if tree == nil {
		return nil, nil
	}
	defer tree.Close()

	root := tree.RootNode()
	return findContiguousImports(root, source, "import_statement"), nil
}

func findContiguousImports(root *tree_sitter.Node, source []byte, nodeKind string) *coder.ImportZone {
	var zone *coder.ImportZone
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child == nil {
			continue
		}
		if child.Kind() == nodeKind {
			start := int(child.StartByte())
			end := int(child.EndByte())
			text := strings.TrimSpace(string(source[start:end]))

			if zone == nil {
				zone = &coder.ImportZone{StartByte: start, EndByte: end}
			} else {
				zone.EndByte = end
			}
			zone.Existing = append(zone.Existing, text)
		} else if zone != nil {
			if child.Kind() == "comment" || child.Kind() == "line_comment" {
				continue
			}
			break
		}
	}
	return zone
}

func extractFunction(node *tree_sitter.Node, source []byte) *coder.Symbol {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := nameNode.Utf8Text(source)
	sig := signatureBeforeBody(node, source)

	return &coder.Symbol{
		Name:      name,
		Kind:      coder.SymbolFunction,
		Signature: sig,
		Doc:       extractDoc(node, source),
		StartByte: int(node.StartByte()),
		EndByte:   int(node.EndByte()),
	}
}

func extractClass(node *tree_sitter.Node, source []byte) *coder.Symbol {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := nameNode.Utf8Text(source)

	sym := &coder.Symbol{
		Name:      name,
		Kind:      coder.SymbolClass,
		Signature: "class " + name,
		Doc:       extractDoc(node, source),
		StartByte: int(node.StartByte()),
		EndByte:   int(node.EndByte()),
	}

	body := node.ChildByFieldName("body")
	if body != nil {
		for i := uint(0); i < body.NamedChildCount(); i++ {
			child := body.NamedChild(i)
			if child == nil {
				continue
			}

			switch child.Kind() {
			case "method_definition":
				m := extractMethodDef(child, source)
				if m != nil {
					sym.Methods = append(sym.Methods, *m)
				}
			}
		}
	}

	return sym
}

func extractInterface(node *tree_sitter.Node, source []byte) *coder.Symbol {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := nameNode.Utf8Text(source)

	sym := &coder.Symbol{
		Name:      name,
		Kind:      coder.SymbolInterface,
		Signature: "interface " + name,
		Doc:       extractDoc(node, source),
		StartByte: int(node.StartByte()),
		EndByte:   int(node.EndByte()),
	}

	body := node.ChildByFieldName("body")
	if body != nil {
		for i := uint(0); i < body.NamedChildCount(); i++ {
			child := body.NamedChild(i)
			if child == nil {
				continue
			}

			switch child.Kind() {
			case "method_signature":
				m := extractMethodSig(child, source)
				if m != nil {
					sym.Methods = append(sym.Methods, *m)
				}
			}
		}
	}

	return sym
}

func extractExport(node *tree_sitter.Node, source []byte) []coder.Symbol {
	var symbols []coder.Symbol

	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "function_declaration":
			sym := extractFunction(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}
		case "class_declaration":
			sym := extractClass(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}
		case "interface_declaration":
			sym := extractInterface(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}
		case "type_alias_declaration":
			sym := extractTypeAlias(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}
		case "enum_declaration":
			sym := extractEnum(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}
		}
	}

	return symbols
}

func extractTypeAlias(node *tree_sitter.Node, source []byte) *coder.Symbol {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := nameNode.Utf8Text(source)
	sig := strings.TrimSpace(node.Utf8Text(source))
	sig = strings.TrimSuffix(sig, ";")

	return &coder.Symbol{
		Name:      name,
		Kind:      coder.SymbolStruct,
		Signature: strings.TrimSpace(sig),
		Doc:       extractDoc(node, source),
		StartByte: int(node.StartByte()),
		EndByte:   int(node.EndByte()),
	}
}

func extractEnum(node *tree_sitter.Node, source []byte) *coder.Symbol {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Utf8Text(source)
	return &coder.Symbol{
		Name:      name,
		Kind:      coder.SymbolStruct, // enum treated as struct-like
		Signature: "enum " + name,
		Doc:       extractDoc(node, source),
		StartByte: int(node.StartByte()),
		EndByte:   int(node.EndByte()),
	}
}

func (q *Query) ReadSkeleton(source []byte) (string, error) {
	tree := q.parser.Parse(source, nil)
	if tree == nil {
		return string(source), nil
	}
	defer tree.Close()

	root := tree.RootNode()
	var replacements []coder.BodyReplacement

	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child == nil {
			continue
		}
		collectTSReplacements(child, &replacements)
	}

	return coder.ApplyReplacements(source, replacements), nil
}

func collectTSReplacements(node *tree_sitter.Node, replacements *[]coder.BodyReplacement) {
	switch node.Kind() {
	case "function_declaration":
		body := node.ChildByFieldName("body")
		if body != nil {
			*replacements = append(*replacements, coder.BodyReplacement{
				StartByte: int(body.StartByte()),
				EndByte:   int(body.EndByte()),
				Stub:      "{ ... }",
			})
		}
	case "class_declaration":
		body := node.ChildByFieldName("body")
		if body != nil {
			for i := uint(0); i < body.NamedChildCount(); i++ {
				child := body.NamedChild(i)
				if child != nil && child.Kind() == "method_definition" {
					methBody := child.ChildByFieldName("body")
					if methBody != nil {
						*replacements = append(*replacements, coder.BodyReplacement{
							StartByte: int(methBody.StartByte()),
							EndByte:   int(methBody.EndByte()),
							Stub:      "{ ... }",
						})
					}
				}
			}
		}
	case "export_statement":
		for i := uint(0); i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child != nil {
				collectTSReplacements(child, replacements)
			}
		}
	}
}

func extractMethodDef(node *tree_sitter.Node, source []byte) *coder.Method {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := nameNode.Utf8Text(source)
	sig := signatureBeforeBody(node, source)

	return &coder.Method{
		Name:      name,
		Signature: sig,
		Doc:       extractDoc(node, source),
		StartByte: int(node.StartByte()),
		EndByte:   int(node.EndByte()),
	}
}

func extractMethodSig(node *tree_sitter.Node, source []byte) *coder.Method {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := nameNode.Utf8Text(source)
	sig := strings.TrimSpace(node.Utf8Text(source))
	sig = strings.TrimSuffix(sig, ";")

	return &coder.Method{
		Name:      name,
		Signature: strings.TrimSpace(sig),
		Doc:       extractDoc(node, source),
		StartByte: int(node.StartByte()),
		EndByte:   int(node.EndByte()),
	}
}

func signatureBeforeBody(node *tree_sitter.Node, source []byte) string {
	bodyNode := node.ChildByFieldName("body")
	if bodyNode == nil {
		text := strings.TrimSpace(node.Utf8Text(source))
		text = strings.TrimSuffix(text, ";")
		return strings.TrimSpace(text)
	}

	start := node.StartByte()
	end := bodyNode.StartByte()
	if end > start {
		return strings.TrimSpace(string(source[start:end]))
	}
	return strings.TrimSpace(node.Utf8Text(source))
}

func extractDoc(node *tree_sitter.Node, source []byte) string {
	prev := node.PrevNamedSibling()
	if prev == nil || prev.Kind() != "comment" {
		return ""
	}

	text := strings.TrimSpace(prev.Utf8Text(source))

	if strings.HasPrefix(text, "/**") {
		text = strings.TrimPrefix(text, "/**")
		text = strings.TrimSuffix(text, "*/")
		lines := strings.Split(text, "\n")
		var cleaned []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "*")
			line = strings.TrimSpace(line)
			if line != "" {
				cleaned = append(cleaned, line)
			}
		}
		return strings.Join(cleaned, "\n")
	}

	var lines []string
	current := prev
	for current != nil && current.Kind() == "comment" {
		line := strings.TrimSpace(current.Utf8Text(source))
		line = strings.TrimPrefix(line, "//")
		line = strings.TrimSpace(line)
		lines = append([]string{line}, lines...)
		current = current.PrevNamedSibling()
	}

	return strings.Join(lines, "\n")
}
