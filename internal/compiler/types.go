package compiler

// Compiler is the main entry point for the vai compilation pipeline.
type Compiler interface {
	// Parse reads .vai files from path, validates, and returns a compiled Program.
	Parse(vaiPath string) (Program, []error)

	// Eval reads .vai files from path, appends the eval source, compiles
	// everything together, and returns the rendered output.
	Eval(vaiPath string, eval string) (string, error)
}

// Program represents a compiled vai program ready for inspection and execution.
type Program interface {
	Tasks() int
	Exec() (string, error)
	Render() string
	HasPrompt(name string) bool
	HasPlan(name string) bool
	ListPrompts() []string
	ListConstraints() []string
	GetPlanSpec(name string) string
	GetPlanImpl(name string) []string
}
