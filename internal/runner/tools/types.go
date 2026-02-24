package tools

// PlanSkeletonInput is the parsed result from the plan_skeleton tool call.
type PlanSkeletonInput struct {
	Imports      []string       `json:"imports"`
	Declarations []SkeletonDecl `json:"declarations"`
	Impls        []SkeletonImpl `json:"impls"`
}

// SkeletonDecl represents a structural declaration from the architect.
type SkeletonDecl struct {
	Kind   string `json:"kind"`   // "struct", "enum", "trait", "interface", "function", "impl_block", "type_alias", "const"
	Name   string `json:"name"`   // fully qualified name, e.g. "Store", "Store::get"
	Action string `json:"action"` // "add", "modify", "remove", "keep"
	Code   string `json:"code"`   // target language code (for add/modify)
	Target string `json:"target"` // target file path (for add when creating in a new file)
}

// SkeletonImpl represents an implementation instruction from the architect.
type SkeletonImpl struct {
	Name        string   `json:"name"`        // matches declaration name
	Action      string   `json:"action"`      // "add", "update", "remove", "keep"
	Instruction string   `json:"instruction"` // natural language instruction for executor
	Uses        []string `json:"uses"`        // symbol dependencies
	Target      string   `json:"target"`      // target file path (optional, overrides plan default)
}

// WriteCodeInput is the parsed result from the write_code tool call.
type WriteCodeInput struct {
	Code     string `json:"code"`      // complete function definition
	Doc      string `json:"doc"`       // documentation comment
	NoChange bool   `json:"no_change"` // true if no changes needed
}

// ReportFixInput is the parsed result from the report_fix tool call.
type ReportFixInput struct {
	Code   string `json:"code"`   // corrected full file content
	Fixed  bool   `json:"fixed"`  // true if error was fixed
	Reason string `json:"reason"` // explanation if not fixed
}
