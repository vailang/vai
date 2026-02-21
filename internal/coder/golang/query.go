package golang

import (
	"strings"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	coder "github.com/vailang/vai/internal/coder/api"
)

type Query struct {
	parser *tree_sitter.Parser
}

func New() (*Query, error) {
	p := tree_sitter.NewParser()
	lang := tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_go.Language()))
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

	structMap := make(map[string]int)

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

		case "method_declaration":
			// Handled in second pass

		case "type_declaration":
			syms := extractTypeDecl(child, source)
			for _, sym := range syms {
				if sym.Kind == coder.SymbolStruct {
					structMap[sym.Name] = len(symbols)
				}
				symbols = append(symbols, sym)
			}
		}
	}

	// Second pass: attach methods to structs
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child == nil || child.Kind() != "method_declaration" {
			continue
		}

		m, receiverType := extractMethod(child, source)
		if m == nil {
			continue
		}

		if idx, ok := structMap[receiverType]; ok {
			symbols[idx].Methods = append(symbols[idx].Methods, *m)
		} else {
			symbols = append(symbols, coder.Symbol{
				Name:      m.Name,
				Kind:      coder.SymbolFunction,
				Signature: m.Signature,
				Doc:       m.Doc,
				StartByte: m.StartByte,
				EndByte:   m.EndByte,
			})
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
	var zone *coder.ImportZone

	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child == nil || child.Kind() != "import_declaration" {
			continue
		}
		start := int(child.StartByte())
		end := int(child.EndByte())
		text := strings.TrimSpace(string(source[start:end]))

		if zone == nil {
			zone = &coder.ImportZone{StartByte: start, EndByte: end}
		} else {
			zone.EndByte = end
		}

		zone.Existing = append(zone.Existing, extractGoImportPaths(text)...)
	}
	return zone, nil
}

func extractGoImportPaths(text string) []string {
	var paths []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, `"`); idx >= 0 {
			rest := line[idx:]
			if end := strings.Index(rest[1:], `"`); end >= 0 {
				paths = append(paths, rest[:end+2])
			}
		}
	}
	return paths
}

func extractFunction(node *tree_sitter.Node, source []byte) *coder.Symbol {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := nameNode.Utf8Text(source)
	sig := signatureBeforeBody(node, "body", source)

	return &coder.Symbol{
		Name:      name,
		Kind:      coder.SymbolFunction,
		Signature: sig,
		Doc:       extractDoc(node, source),
		StartByte: int(node.StartByte()),
		EndByte:   int(node.EndByte()),
	}
}

func extractMethod(node *tree_sitter.Node, source []byte) (*coder.Method, string) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil, ""
	}

	name := nameNode.Utf8Text(source)
	sig := signatureBeforeBody(node, "body", source)

	receiverType := ""
	recvNode := node.ChildByFieldName("receiver")
	if recvNode != nil {
		receiverType = extractReceiverType(recvNode, source)
	}

	m := &coder.Method{
		Name:      name,
		Signature: sig,
		Doc:       extractDoc(node, source),
		StartByte: int(node.StartByte()),
		EndByte:   int(node.EndByte()),
	}

	return m, receiverType
}

func extractReceiverType(recvNode *tree_sitter.Node, source []byte) string {
	text := recvNode.Utf8Text(source)
	text = strings.TrimPrefix(text, "(")
	text = strings.TrimSuffix(text, ")")
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return ""
	}
	typeName := parts[len(parts)-1]
	typeName = strings.TrimPrefix(typeName, "*")
	return typeName
}

func extractTypeDecl(node *tree_sitter.Node, source []byte) []coder.Symbol {
	var symbols []coder.Symbol

	for i := uint(0); i < node.NamedChildCount(); i++ {
		spec := node.NamedChild(i)
		if spec == nil || spec.Kind() != "type_spec" {
			continue
		}

		nameNode := spec.ChildByFieldName("name")
		typeNode := spec.ChildByFieldName("type")
		if nameNode == nil || typeNode == nil {
			continue
		}

		name := nameNode.Utf8Text(source)

		switch typeNode.Kind() {
		case "struct_type":
			symbols = append(symbols, coder.Symbol{
				Name:      name,
				Kind:      coder.SymbolStruct,
				Signature: "type " + name + " struct",
				Doc:       extractDoc(node, source),
				StartByte: int(node.StartByte()),
				EndByte:   int(node.EndByte()),
			})

		case "interface_type":
			sym := coder.Symbol{
				Name:      name,
				Kind:      coder.SymbolInterface,
				Signature: "type " + name + " interface",
				Doc:       extractDoc(node, source),
				StartByte: int(node.StartByte()),
				EndByte:   int(node.EndByte()),
			}
			sym.Methods = extractInterfaceMethods(typeNode, source)
			symbols = append(symbols, sym)
		}
	}

	return symbols
}

func extractInterfaceMethods(interfaceNode *tree_sitter.Node, source []byte) []coder.Method {
	var methods []coder.Method

	for i := uint(0); i < interfaceNode.NamedChildCount(); i++ {
		child := interfaceNode.NamedChild(i)
		if child == nil {
			continue
		}

		if child.Kind() == "method_elem" || child.Kind() == "method_spec" {
			nameNode := child.ChildByFieldName("name")
			if nameNode == nil {
				continue
			}

			methods = append(methods, coder.Method{
				Name:      nameNode.Utf8Text(source),
				Signature: strings.TrimSpace(child.Utf8Text(source)),
				Doc:       extractDoc(child, source),
				StartByte: int(child.StartByte()),
				EndByte:   int(child.EndByte()),
			})
		}
	}

	return methods
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
		switch child.Kind() {
		case "function_declaration", "method_declaration":
			body := child.ChildByFieldName("body")
			if body != nil {
				replacements = append(replacements, coder.BodyReplacement{
					StartByte: int(body.StartByte()),
					EndByte:   int(body.EndByte()),
					Stub:      "{ ... }",
				})
			}
		}
	}

	return coder.ApplyReplacements(source, replacements), nil
}

func signatureBeforeBody(node *tree_sitter.Node, bodyField string, source []byte) string {
	bodyNode := node.ChildByFieldName(bodyField)
	if bodyNode == nil {
		return strings.TrimSpace(node.Utf8Text(source))
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

	var lines []string
	current := prev
	for current != nil && current.Kind() == "comment" {
		text := strings.TrimSpace(current.Utf8Text(source))
		text = strings.TrimPrefix(text, "//")
		text = strings.TrimSpace(text)
		lines = append([]string{text}, lines...)
		current = current.PrevNamedSibling()
	}

	return strings.Join(lines, "\n")
}
