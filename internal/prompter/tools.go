package prompter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vailang/vai/internal/compiler"
	"github.com/vailang/vai/internal/config"
	"github.com/vailang/vai/internal/runner/provider"
)

// ToolServer handles tool calls from the LLM during the prompter loop.
type ToolServer struct {
	baseDir string
	cfg     *config.Config
	written map[string]string // in-memory overlay: absolute path → content
	changes []Change
}

// NewToolServer creates a ToolServer for the given project.
func NewToolServer(baseDir string, cfg *config.Config) *ToolServer {
	return &ToolServer{
		baseDir: baseDir,
		cfg:     cfg,
		written: make(map[string]string),
	}
}

// Changes returns all file changes made during the session.
func (s *ToolServer) Changes() []Change {
	return s.changes
}

// Execute dispatches a tool call by name and returns the result string.
func (s *ToolServer) Execute(name string, rawInput string) string {
	switch name {
	case "list_plans":
		return s.listPlans()
	case "read_spec":
		return s.readSpec(rawInput)
	case "update_spec":
		return s.updateSpec(rawInput)
	default:
		return fmt.Sprintf("error: unknown tool %q", name)
	}
}

// ToolDefinitions returns the tool definitions for the prompter.
func ToolDefinitions() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "list_plans",
			Description: "List all available plans in the project with their names.",
			InputSchema: mustJSON(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		{
			Name:        "read_spec",
			Description: "Read the current specification of a named plan.",
			InputSchema: mustJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "The plan name",
					},
				},
				"required": []string{"name"},
			}),
		},
		{
			Name:        "update_spec",
			Description: "Update the specification of a named plan with new content.",
			InputSchema: mustJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "The plan name to update",
					},
					"spec": map[string]any{
						"type":        "string",
						"description": "The new specification text",
					},
				},
				"required": []string{"name", "spec"},
			}),
		},
	}
}

// --- Tool implementations ---

func (s *ToolServer) listPlans() string {
	prog, err := s.compileProject()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	type planEntry struct {
		Name    string `json:"name"`
		HasSpec bool   `json:"has_spec"`
	}

	var plans []planEntry
	for _, req := range prog.Requests() {
		plans = append(plans, planEntry{
			Name:    req.Name,
			HasSpec: prog.GetPlanSpec(req.Name) != "",
		})
	}

	data, _ := json.MarshalIndent(plans, "", "  ")
	return string(data)
}

func (s *ToolServer) readSpec(rawInput string) string {
	var input struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(rawInput), &input); err != nil {
		return fmt.Sprintf("error: invalid input: %v", err)
	}

	prog, err := s.compileProject()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	spec := prog.GetPlanSpec(input.Name)
	if spec == "" {
		return fmt.Sprintf("error: plan %q not found or has no spec", input.Name)
	}
	return spec
}

func (s *ToolServer) updateSpec(rawInput string) string {
	var input struct {
		Name string `json:"name"`
		Spec string `json:"spec"`
	}
	if err := json.Unmarshal([]byte(rawInput), &input); err != nil {
		return fmt.Sprintf("error: invalid input: %v", err)
	}

	// Find which source file contains this plan.
	prog, err := s.compileProject()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	var sourcePath string
	for _, req := range prog.Requests() {
		if req.Name == input.Name {
			sourcePath = req.SourcePath
			break
		}
	}
	if sourcePath == "" {
		return fmt.Sprintf("error: plan %q not found", input.Name)
	}

	// Read the source file (prefer in-memory overlay).
	source, err := s.readSourceFile(sourcePath)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	// Replace the spec block content.
	updated, err := replaceSpecContent(source, input.Name, input.Spec)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	// Track the change.
	relPath, _ := filepath.Rel(s.baseDir, sourcePath)
	changeType := ChangeModified
	var original string

	if prev, ok := s.written[sourcePath]; ok {
		// Already modified this session — find original from prior change.
		for i := range s.changes {
			if s.changes[i].Path == relPath {
				original = s.changes[i].Original
				if s.changes[i].Type == ChangeCreated {
					changeType = ChangeCreated
				}
				s.changes = append(s.changes[:i], s.changes[i+1:]...)
				break
			}
		}
		_ = prev
	} else {
		// First modification — original is on disk.
		original = source
	}

	s.written[sourcePath] = updated
	s.changes = append(s.changes, Change{
		Path:     relPath,
		Type:     changeType,
		Content:  updated,
		Original: original,
		IsVai:    true,
	})

	return "ok"
}

// --- Helpers ---

// compileProject parses all .vai/.plan files in the project, including in-memory overlays.
func (s *ToolServer) compileProject() (compiler.Program, error) {
	comp := compiler.New()
	comp.SetBaseDir(s.baseDir)

	pkg := &config.Package{
		Name:       s.cfg.Lib.Name,
		ConfigPath: filepath.Join(s.baseDir, "vai.toml"),
		RootDir:    s.baseDir,
		SrcDir:     filepath.Join(s.baseDir, s.cfg.Lib.Prompts),
		Config:     s.cfg,
	}
	files, err := config.LoadPackageFiles(pkg)
	if err != nil {
		return nil, fmt.Errorf("loading package files: %w", err)
	}

	// Overlay in-memory changes.
	for absPath, content := range s.written {
		ext := filepath.Ext(absPath)
		if ext == ".vai" || ext == ".plan" {
			files[absPath] = content
		}
	}

	prog, errs := comp.ParseSources(files)
	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		return nil, fmt.Errorf("compilation errors:\n%s", strings.Join(msgs, "\n"))
	}
	return prog, nil
}

// readSourceFile reads a source file, preferring the in-memory overlay.
func (s *ToolServer) readSourceFile(absPath string) (string, error) {
	if content, ok := s.written[absPath]; ok {
		return content, nil
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// replaceSpecContent finds the spec block inside the named plan and replaces its body.
func replaceSpecContent(source, planName, newSpec string) (string, error) {
	planStart := findPlanStart(source, planName)
	if planStart < 0 {
		return "", fmt.Errorf("plan %q not found in source", planName)
	}

	planEnd := findMatchingBrace(source, planStart)
	if planEnd < 0 {
		return "", fmt.Errorf("no matching brace for plan %q", planName)
	}

	// Search for spec block within the plan body.
	planBody := source[planStart : planEnd+1]
	specBodyStart, specBodyEnd := findSpecBlock(planBody)
	if specBodyStart < 0 {
		return "", fmt.Errorf("no spec block found in plan %q", planName)
	}

	// Convert offsets to absolute positions in source.
	absSpecStart := planStart + specBodyStart
	absSpecEnd := planStart + specBodyEnd

	// Build the replacement: indent the new spec text.
	var newBody strings.Builder
	newBody.WriteByte('\n')
	for _, line := range strings.Split(strings.TrimSpace(newSpec), "\n") {
		newBody.WriteString("        ")
		newBody.WriteString(line)
		newBody.WriteByte('\n')
	}
	newBody.WriteString("    ")

	var b strings.Builder
	b.WriteString(source[:absSpecStart])
	b.WriteString(newBody.String())
	b.WriteString(source[absSpecEnd:])
	return b.String(), nil
}

// findPlanStart finds the byte offset of "plan <name>" followed by '{' in source.
func findPlanStart(source, name string) int {
	target := "plan " + name
	idx := 0
	for {
		pos := strings.Index(source[idx:], target)
		if pos < 0 {
			return -1
		}
		pos += idx

		// Must be at start of line or start of file.
		if pos > 0 && source[pos-1] != '\n' && source[pos-1] != '\r' {
			idx = pos + len(target)
			continue
		}

		// Next non-space after name must be '{'.
		rest := strings.TrimSpace(source[pos+len(target):])
		if len(rest) > 0 && rest[0] == '{' {
			return pos
		}

		idx = pos + len(target)
	}
}

// findMatchingBrace finds the matching closing brace starting from planStart.
func findMatchingBrace(source string, planStart int) int {
	braceStart := strings.Index(source[planStart:], "{")
	if braceStart < 0 {
		return -1
	}
	braceStart += planStart

	depth := 0
	for i := braceStart; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// findSpecBlock finds the spec { ... } block within a plan body string.
// Returns the start and end byte offsets of the spec body content
// (between the opening '{' and closing '}').
func findSpecBlock(planBody string) (int, int) {
	// Find "spec" keyword followed by '{'.
	idx := 0
	for {
		pos := strings.Index(planBody[idx:], "spec")
		if pos < 0 {
			return -1, -1
		}
		pos += idx

		// Check it's a standalone keyword (not part of "inspect" etc).
		if pos > 0 {
			prev := planBody[pos-1]
			if prev != ' ' && prev != '\t' && prev != '\n' && prev != '\r' {
				idx = pos + 4
				continue
			}
		}

		// Find the opening brace after "spec".
		rest := planBody[pos+4:]
		braceIdx := strings.Index(rest, "{")
		if braceIdx < 0 {
			return -1, -1
		}

		// Everything between "spec" and '{' must be whitespace.
		between := rest[:braceIdx]
		if strings.TrimSpace(between) != "" {
			idx = pos + 4
			continue
		}

		bodyStart := pos + 4 + braceIdx + 1 // after '{'

		// Find matching closing brace.
		depth := 1
		for i := bodyStart; i < len(planBody); i++ {
			switch planBody[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return bodyStart, i
				}
			}
		}
		return -1, -1
	}
}

func mustJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
