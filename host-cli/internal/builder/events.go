package builder

// EventState is a JSON Lines-compatible build lifecycle state.
type EventState string

const (
	EventQueued    EventState = "queued"
	EventStarted   EventState = "started"
	EventProgress  EventState = "progress"
	EventLog       EventState = "log"
	EventSkipped   EventState = "skipped"
	EventCompleted EventState = "completed"
	EventFailed    EventState = "failed"
)

// BuildEvent is the shared progress shape consumed by CLI and TUI adapters.
type BuildEvent struct {
	RecipeID   string     `json:"recipe_id"`
	Procedure  string     `json:"procedure_id,omitempty"`
	State      EventState `json:"state"`
	Message    string     `json:"message,omitempty"`
	Downloaded int64      `json:"downloaded,omitempty"`
	Total      int64      `json:"total,omitempty"`
	Error      string     `json:"error,omitempty"`
}

func emit(opts Options, event BuildEvent) {
	if opts.OnEvent != nil {
		opts.OnEvent(event)
	}
	if opts.OnStatus != nil && event.Message != "" {
		opts.OnStatus(event.Message)
	}
}
