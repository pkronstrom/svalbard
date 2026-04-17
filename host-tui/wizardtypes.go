// Package hosttui re-exports wizard types so external callers (host-cli)
// can build WizardConfig without importing the internal wizard package.
package hosttui

import "github.com/pkronstrom/svalbard/host-tui/internal/wizard"

// Type aliases for wizard data types used by host-cli to build configs.
type (
	WizardConfig     = wizard.WizardConfig
	WizardResult     = wizard.WizardResult
	WizardApplyEvent = wizard.ApplyEvent
	Volume           = wizard.Volume
	PresetOption     = wizard.PresetOption
	PackGroup        = wizard.PackGroup
	Pack             = wizard.Pack
	PackSource       = wizard.PackSource
)
