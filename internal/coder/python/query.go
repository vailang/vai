package python

import (
	"strings"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	coder "github.com/vailang/vai/internal/coder/api"
)

type Query struct {
	parser *tree_sitter.Parser
}

func New() (*Query, error) {
	p := tree_sitter.NewParser()
	lang := tree_sitter.NewLanguage(unsafe.Pointer(tree_sitter_python.Language()))
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
			sym := extractFunction(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}

		case "class_definition":
			sym := extractClass(child, source)
			if sym != nil {
				symbols = append(symbols, *sym)
			}

		case "decorated_definition":
			defNode := child.ChildByFieldName("definition")
			if defNode != nil {
				switch defNode.Kind() {
				case "function_definition":
					sym := extractFunction(defNode, source)
					if sym != nil {
						sym.StartByte = int(child.StartByte())
						symbols = append(symbols, *sym)
					}
				case "class_definition":
					sym := extractClass(defNode, source)
					if sym != nil {
						sym.StartByte = int(child.StartByte())
						symbols = append(symbols, *sym)
					}
				}
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
	var zone *coder.ImportZone

	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child == nil {
			continue
		}
		kind := child.Kind()
		if kind == "import_statement" || kind == "import_from_statement" {
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
			if kind == "comment" || kind == "expression_statement" {
				continue
			}
			break
		}
	}
	return zone, nil
}

func extractFunction(node *tree_sitter.Node, source []byte) *coder.Symbol {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := nameNode.Utf8Text(source)
	sig := buildFuncSignature(node, source)

	return &coder.Symbol{
		Name:      name,
		Kind:      coder.SymbolFunction,
		Signature: sig,
		Doc:       extractDocstring(node, source),
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

	sig := "class " + name
	superclasses := node.ChildByFieldName("superclasses")
	if superclasses != nil {
		sig += superclasses.Utf8Text(source)
	}

	sym := &coder.Symbol{
		Name:      name,
		Kind:      coder.SymbolClass,
		Signature: sig,
		Doc:       extractDocstring(node, source),
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

			var methNode *tree_sitter.Node
			if child.Kind() == "function_definition" {
				methNode = child
			} else if child.Kind() == "decorated_definition" {
				defNode := child.ChildByFieldName("definition")
				if defNode != nil && defNode.Kind() == "function_definition" {
					methNode = defNode
				}
			}

			if methNode == nil {
				continue
			}

			methName := methNode.ChildByFieldName("name")
			if methName == nil {
				continue
			}

			sym.Methods = append(sym.Methods, coder.Method{
				Name:      methName.Utf8Text(source),
				Signature: buildFuncSignature(methNode, source),
				Doc:       extractDocstring(methNode, source),
				StartByte: int(methNode.StartByte()),
				EndByte:   int(methNode.EndByte()),
			})
		}
	}

	return sym
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
		case "function_definition":
			collectPythonFuncBodyReplacement(child, &replacements)
		case "class_definition":
			collectPythonClassBodyReplacements(child, &replacements)
		case "decorated_definition":
			defNode := child.ChildByFieldName("definition")
			if defNode != nil {
				switch defNode.Kind() {
				case "function_definition":
					collectPythonFuncBodyReplacement(defNode, &replacements)
				case "class_definition":
					collectPythonClassBodyReplacements(defNode, &replacements)
				}
			}
		}
	}

	return coder.ApplyReplacements(source, replacements), nil
}

func collectPythonFuncBodyReplacement(funcNode *tree_sitter.Node, replacements *[]coder.BodyReplacement) {
	body := funcNode.ChildByFieldName("body")
	if body == nil {
		return
	}
	// The source before body.StartByte() already has newline + indentation,
	// so the stub needs no extra indent prefix.
	*replacements = append(*replacements, coder.BodyReplacement{
		StartByte: int(body.StartByte()),
		EndByte:   int(body.EndByte()),
		Stub:      "...",
	})
}

func collectPythonClassBodyReplacements(classNode *tree_sitter.Node, replacements *[]coder.BodyReplacement) {
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return
	}
	for i := uint(0); i < body.NamedChildCount(); i++ {
		child := body.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Kind() {
		case "function_definition":
			collectPythonFuncBodyReplacement(child, replacements)
		case "decorated_definition":
			defNode := child.ChildByFieldName("definition")
			if defNode != nil && defNode.Kind() == "function_definition" {
				collectPythonFuncBodyReplacement(defNode, replacements)
			}
		}
	}
}

func buildFuncSignature(node *tree_sitter.Node, source []byte) string {
	nameNode := node.ChildByFieldName("name")
	paramsNode := node.ChildByFieldName("parameters")
	returnNode := node.ChildByFieldName("return_type")

	if nameNode == nil {
		return ""
	}

	sig := "def " + nameNode.Utf8Text(source)
	if paramsNode != nil {
		sig += paramsNode.Utf8Text(source)
	}
	if returnNode != nil {
		sig += " -> " + returnNode.Utf8Text(source)
	}

	return sig
}

func extractDocstring(node *tree_sitter.Node, source []byte) string {
	body := node.ChildByFieldName("body")
	if body == nil {
		return ""
	}

	if body.NamedChildCount() == 0 {
		return ""
	}

	first := body.NamedChild(0)
	if first == nil || first.Kind() != "expression_statement" {
		return ""
	}

	if first.NamedChildCount() == 0 {
		return ""
	}

	strNode := first.NamedChild(0)
	if strNode == nil || strNode.Kind() != "string" {
		return ""
	}

	text := strNode.Utf8Text(source)
	text = strings.TrimPrefix(text, `"""`)
	text = strings.TrimPrefix(text, `'''`)
	text = strings.TrimSuffix(text, `"""`)
	text = strings.TrimSuffix(text, `'''`)
	return strings.TrimSpace(text)
}
