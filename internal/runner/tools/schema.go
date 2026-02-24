package tools

import (
	"encoding/json"

	"github.com/vailang/vai/internal/runner/provider"
)

// PlanSkeletonTool returns the tool definition for plan_skeleton.
func PlanSkeletonTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "plan_skeleton",
		Description: "Declare the desired state of target files. The compiler will diff against current state and apply changes. For 'keep' declarations, omit the 'code' field. For 'remove', omit 'code'. Only 'add' and 'modify' require 'code'.",
		InputSchema: mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"imports": map[string]any{
					"type":        "array",
					"description": "All import/include statements needed by the target file. One per element. Example: '#include <stdio.h>' or 'import fmt'.",
					"items":       map[string]any{"type": "string"},
				},
				"declarations": map[string]any{
					"type":        "array",
					"description": "Structural declarations: structs, enums, traits, function signatures",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"kind": map[string]any{
								"type": "string",
								"enum": []string{"struct", "enum", "trait", "interface", "function", "impl_block", "type_alias", "const"},
							},
							"name": map[string]any{
								"type":        "string",
								"description": "Fully qualified name. Use Type.method for methods.",
							},
							"action": map[string]any{
								"type": "string",
								"enum": []string{"add", "modify", "remove", "keep"},
							},
							"code": map[string]any{
								"type":        "string",
								"description": "Target language code. For functions, include signature with empty body placeholder. Required for 'add' and 'modify'. Omit for 'keep' and 'remove'.",
							},
							"target": map[string]any{
								"type":        "string",
								"description": "Target file path. Required for 'add' when creating in a new file.",
							},
						},
						"required": []string{"kind", "name", "action"},
					},
				},
				"impls": map[string]any{
					"type":        "array",
					"description": "Implementation instructions for the executor. One per function that needs a body.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{
								"type":        "string",
								"description": "Matches declaration name.",
							},
							"action": map[string]any{
								"type": "string",
								"enum": []string{"add", "update", "remove", "keep"},
							},
							"instruction": map[string]any{
								"type":        "string",
								"description": "Clear, concise instruction for the executor. Required for 'add' and 'update'.",
							},
							"uses": map[string]any{
								"type":        "array",
								"items":       map[string]any{"type": "string"},
								"description": "Symbol names this implementation depends on.",
							},
							"target": map[string]any{
								"type":        "string",
								"description": "Target file path for this impl. Required when plan has multiple targets. Omit to use the plan's primary target.",
							},
						},
						"required": []string{"name", "action"},
					},
				},
			},
			"required": []string{"imports", "declarations", "impls"},
		}),
	}
}

// WriteCodeTool returns the tool definition for write_code.
func WriteCodeTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "write_code",
		Description: "Write the complete implementation for a single function. Include the full function definition (signature and body). Do not include imports, type definitions, or other functions.",
		InputSchema: mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"code": map[string]any{
					"type":        "string",
					"description": "The complete function definition including signature and body. No imports. No type redefinitions. No other functions.",
				},
				"doc": map[string]any{
					"type":        "string",
					"description": "One-line documentation comment for the function.",
				},
				"no_change": map[string]any{
					"type":        "boolean",
					"description": "Set to true if the current implementation is already correct.",
				},
			},
			"required": []string{"code", "no_change"},
		}),
	}
}

// ReportFixTool returns the tool definition for report_fix.
func ReportFixTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "report_fix",
		Description: "Report the fix for a compilation error. Provide the corrected full file content or report inability to fix.",
		InputSchema: mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"code": map[string]any{
					"type":        "string",
					"description": "Corrected full file content including imports.",
				},
				"fixed": map[string]any{
					"type":        "boolean",
					"description": "True if the error was fixed. False if unable to fix.",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "If fixed=false, explain why.",
				},
			},
			"required": []string{"fixed"},
		}),
	}
}

// mustJSON marshals v to JSON. Panics on error — safe because callers
// only pass static map[string]any literals that are guaranteed to marshal.
func mustJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
