package clang

import (
	"strings"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_c "github.com/tree-sitter/tree-sitter-c/bindings/go"
	coder "github.com/vailang/vai/internal/coder/api"
)

type Query struct {
	parser *tree_sitter.Parser
}

func New() (*Query, error) {
	p := tree_sitter.NewParser()
	lang := tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_c.Language()))
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
		case "function_definition":
			sym := extractFuncDef(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}

		case "declaration":
			sym := extractDeclaration(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}

		case "struct_specifier":
			sym := extractStruct(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}

		case "type_definition":
			sym := extractTypedef(child, source)
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
	return findContiguousImports(root, source, "preproc_include"), nil
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

func extractFuncDef(node *tree_sitter.Node, source []byte) *coder.Symbol {
	declarator := node.ChildByFieldName("declarator")
	if declarator == nil {
		return nil
	}

	name := extractFuncName(declarator, source)
	if name == "" {
		return nil
	}

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

func extractDeclaration(node *tree_sitter.Node, source []byte) *coder.Symbol {
	declarator := node.ChildByFieldName("declarator")
	if declarator == nil {
		return nil
	}

	if declarator.Kind() == "function_declarator" {
		name := extractFuncName(declarator, source)
		if name == "" {
			return nil
		}

		sig := strings.TrimSpace(node.Utf8Text(source))
		sig = strings.TrimSuffix(sig, ";")
		sig = strings.TrimSpace(sig)

		return &coder.Symbol{
			Name:      name,
			Kind:      coder.SymbolFunction,
			Signature: sig,
			Doc:       extractDoc(node, source),
			StartByte: int(node.StartByte()),
			EndByte:   int(node.EndByte()),
		}
	}

	return nil
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

func extractTypedef(node *tree_sitter.Node, source []byte) *coder.Symbol {
	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Kind() == "struct_specifier" {
			declarator := node.ChildByFieldName("declarator")
			if declarator != nil {
				name := declarator.Utf8Text(source)
				return &coder.Symbol{
					Name:      name,
					Kind:      coder.SymbolStruct,
					Signature: "typedef struct " + name,
					Doc:       extractDoc(node, source),
					StartByte: int(node.StartByte()),
					EndByte:   int(node.EndByte()),
				}
			}
		}
	}
	return nil
}

func extractFuncName(declarator *tree_sitter.Node, source []byte) string {
	if declarator.Kind() == "function_declarator" {
		nameNode := declarator.ChildByFieldName("declarator")
		if nameNode != nil {
			if nameNode.Kind() == "identifier" {
				return nameNode.Utf8Text(source)
			}
			if nameNode.Kind() == "pointer_declarator" {
				for i := uint(0); i < nameNode.NamedChildCount(); i++ {
					child := nameNode.NamedChild(i)
					if child != nil && child.Kind() == "identifier" {
						return child.Utf8Text(source)
					}
				}
			}
			return nameNode.Utf8Text(source)
		}
	}
	return ""
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
		if child.Kind() == "function_definition" {
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

	var lines []string
	current := prev
	for current != nil && current.Kind() == "comment" {
		text := strings.TrimSpace(current.Utf8Text(source))
		text = strings.TrimPrefix(text, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		text = strings.TrimSpace(text)
		lines = append([]string{text}, lines...)
		current = current.PrevNamedSibling()
	}

	return strings.Join(lines, "\n")
}
