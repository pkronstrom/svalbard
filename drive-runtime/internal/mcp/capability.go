package mcp

import "context"

// ParamDef describes a single parameter for an action.
type ParamDef struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Desc     string   `json:"description"`
	Default  any      `json:"default,omitempty"`
	Enum     []string `json:"enum,omitempty"`
}

// ActionDef describes a named action that a capability exposes.
type ActionDef struct {
	Name   string     `json:"name"`
	Desc   string     `json:"description"`
	Params []ParamDef `json:"params"`
}

// ActionResult carries tool output. Errors use the error return only.
type ActionResult struct {
	Data any    `json:"data,omitempty"`
	Text string `json:"text,omitempty"`
}

// Capability is the interface that every MCP-exposed drive feature must implement.
type Capability interface {
	Tool() string
	Description() string
	Actions() []ActionDef
	Handle(ctx context.Context, action string, params map[string]any) (ActionResult, error)
	Close() error
}
