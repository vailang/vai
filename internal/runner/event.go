package runner

// EventKind classifies the type of a runner event.
type EventKind string

const (
	// Step lifecycle events.
	EventStepStart    EventKind = "step_start"
	EventStepComplete EventKind = "step_complete"
	EventStepFailed   EventKind = "step_failed"

	// Informational events.
	EventInfo    EventKind = "info"
	EventWarning EventKind = "warning"

	// Impl lifecycle events.
	EventImplStart EventKind = "impl_start"

	// Structured data events.
	EventSkeleton EventKind = "skeleton"
	EventSummary  EventKind = "summary"
	EventDone     EventKind = "done"
)

// Event represents a single runner lifecycle event.
type Event struct {
	Kind    EventKind   `json:"kind"`
	Step    string      `json:"step,omitempty"`
	Name    string      `json:"name,omitempty"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// SkeletonData is the structured data attached to EventSkeleton events.
type SkeletonData struct {
	ImportCount int `json:"import_count"`
	DeclCount   int `json:"decl_count"`
	ImplCount   int `json:"impl_count"`
}
