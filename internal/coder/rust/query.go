package rust

import (
	"strings"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	coder "github.com/vailang/vai/internal/coder/api"
)

type Query struct {
	parser *tree_sitter.Parser
}

func New() (*Query, error) {
	p := tree_sitter.NewParser()
	lang := tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_rust.Language()))
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
		case "function_item":
			sym := extractFunction(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}

		case "struct_item":
			sym := extractStruct(child, source)
			if sym != nil {
				structMap[sym.Name] = len(symbols)
				symbols = append(symbols, *sym)
			}

		case "enum_item":
			sym := extractEnum(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}

		case "trait_item":
			sym := extractTrait(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}

		case "impl_item":
			attachImplMethods(child, source, structMap, &symbols)
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
	return findContiguousImports(root, source, "use_declaration"), nil
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

func extractStruct(node *tree_sitter.Node, source []byte) *coder.Symbol {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := nameNode.Utf8Text(source)

	return &coder.Symbol{
		Name:      name,
		Kind:      coder.SymbolStruct,
		Signature: "struct " + name,
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

func extractTrait(node *tree_sitter.Node, source []byte) *coder.Symbol {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := nameNode.Utf8Text(source)

	sym := &coder.Symbol{
		Name:      name,
		Kind:      coder.SymbolTrait,
		Signature: "trait " + name,
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

			if child.Kind() == "function_item" || child.Kind() == "function_signature_item" {
				methName := child.ChildByFieldName("name")
				if methName == nil {
					continue
				}

				sym.Methods = append(sym.Methods, coder.Method{
					Name:      methName.Utf8Text(source),
					Signature: signatureBeforeBody(child, source),
					Doc:       extractDoc(child, source),
					StartByte: int(child.StartByte()),
					EndByte:   int(child.EndByte()),
				})
			}
		}
	}

	return sym
}

func attachImplMethods(node *tree_sitter.Node, source []byte, structMap map[string]int, symbols *[]coder.Symbol) {
	typeNode := node.ChildByFieldName("type")
	if typeNode == nil {
		return
	}

	typeName := typeNode.Utf8Text(source)

	body := node.ChildByFieldName("body")
	if body == nil {
		return
	}

	for i := uint(0); i < body.NamedChildCount(); i++ {
		child := body.NamedChild(i)
		if child == nil || child.Kind() != "function_item" {
			continue
		}

		methName := child.ChildByFieldName("name")
		if methName == nil {
			continue
		}

		m := coder.Method{
			Name:      methName.Utf8Text(source),
			Signature: signatureBeforeBody(child, source),
			Doc:       extractDoc(child, source),
			StartByte: int(child.StartByte()),
			EndByte:   int(child.EndByte()),
		}

		if idx, ok := structMap[typeName]; ok {
			(*symbols)[idx].Methods = append((*symbols)[idx].Methods, m)
		} else {
			*symbols = append(*symbols, coder.Symbol{
				Name:      m.Name,
				Kind:      coder.SymbolFunction,
				Signature: m.Signature,
				Doc:       m.Doc,
				StartByte: m.StartByte,
				EndByte:   m.EndByte,
			})
		}
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
		switch child.Kind() {
		case "function_item":
			body := child.ChildByFieldName("body")
			if body != nil {
				replacements = append(replacements, coder.BodyReplacement{
					StartByte: int(body.StartByte()),
					EndByte:   int(body.EndByte()),
					Stub:      "{ ... }",
				})
			}
		case "impl_item":
			collectImplMethodBodyReplacements(child, &replacements)
		case "trait_item":
			collectTraitMethodBodyReplacements(child, &replacements)
		}
	}

	return coder.ApplyReplacements(source, replacements), nil
}

func collectImplMethodBodyReplacements(implNode *tree_sitter.Node, replacements *[]coder.BodyReplacement) {
	body := implNode.ChildByFieldName("body")
	if body == nil {
		return
	}
	for i := uint(0); i < body.NamedChildCount(); i++ {
		child := body.NamedChild(i)
		if child != nil && child.Kind() == "function_item" {
			fnBody := child.ChildByFieldName("body")
			if fnBody != nil {
				*replacements = append(*replacements, coder.BodyReplacement{
					StartByte: int(fnBody.StartByte()),
					EndByte:   int(fnBody.EndByte()),
					Stub:      "{ ... }",
				})
			}
		}
	}
}

func collectTraitMethodBodyReplacements(traitNode *tree_sitter.Node, replacements *[]coder.BodyReplacement) {
	body := traitNode.ChildByFieldName("body")
	if body == nil {
		return
	}
	for i := uint(0); i < body.NamedChildCount(); i++ {
		child := body.NamedChild(i)
		if child != nil && child.Kind() == "function_item" {
			fnBody := child.ChildByFieldName("body")
			if fnBody != nil {
				*replacements = append(*replacements, coder.BodyReplacement{
					StartByte: int(fnBody.StartByte()),
					EndByte:   int(fnBody.EndByte()),
					Stub:      "{ ... }",
				})
			}
		}
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
	if prev == nil {
		return ""
	}

	var lines []string
	current := prev
	for current != nil && current.Kind() == "line_comment" {
		text := strings.TrimSpace(current.Utf8Text(source))
		if !strings.HasPrefix(text, "///") {
			break
		}
		text = strings.TrimPrefix(text, "///")
		text = strings.TrimSpace(text)
		lines = append([]string{text}, lines...)
		current = current.PrevNamedSibling()
	}

	if len(lines) > 0 {
		return strings.Join(lines, "\n")
	}

	if prev.Kind() == "block_comment" {
		text := prev.Utf8Text(source)
		if strings.HasPrefix(text, "/**") {
			text = strings.TrimPrefix(text, "/**")
			text = strings.TrimSuffix(text, "*/")
			return strings.TrimSpace(text)
		}
	}

	return ""
}
