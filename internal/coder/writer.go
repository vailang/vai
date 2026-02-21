package coder

import (
	"fmt"
	"os"
	"strings"
)

// InsertImports reads the target file, detects existing imports via tree-sitter,
// deduplicates against new imports, and inserts only missing ones non-destructively.
func (c *Coder) InsertImports(targetPath string, newImports []string) error {
	if len(newImports) == 0 {
		return nil
	}

	source, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("reading target for import insertion: %w", err)
	}

	zone, err := c.FindImportZone(source)
	if err != nil {
		return fmt.Errorf("detecting import zone: %w", err)
	}

	var existing []string
	if zone != nil {
		existing = zone.Existing
	}
	missing := deduplicateImports(existing, newImports)
	if len(missing) == 0 {
		return nil
	}

	comment := string(CommentPrefix(c.lang)) + " from vailang compiler"
	block := c.BuildImportBlock(missing, comment)

	var result []byte
	if zone != nil {
		result = append(result, source[:zone.EndByte]...)
		result = append(result, '\n')
		result = append(result, []byte(block)...)
		result = append(result, source[zone.EndByte:]...)
	} else {
		result = append(result, []byte(block)...)
		result = append(result, '\n')
		result = append(result, source...)
	}

	return os.WriteFile(targetPath, result, 0644)
}

// BuildImportBlock formats the missing imports as a ready-to-insert text block.
func (c *Coder) BuildImportBlock(imports []string, comment string) string {
	var b strings.Builder
	b.WriteString(comment + "\n")

	switch c.lang {
	case Go:
		b.WriteString("import (\n")
		for _, imp := range imports {
			b.WriteString("\t" + imp + "\n")
		}
		b.WriteString(")\n")
	default:
		for _, imp := range imports {
			b.WriteString(imp + "\n")
		}
	}

	return b.String()
}
