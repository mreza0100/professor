package chat

import (
	"context"
	"io"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/headless"
)

// Verbs binds the verb functions to one process's runtime, for a surface that
// holds a handle instead of threading a runtime through every call — the MCP
// server. Each method is the package function of the same name.
type Verbs struct {
	Runtime  *pfmconfig.Runtime
	Warnings io.Writer
}

// Last is chat.Last over the bound runtime.
func (verbs Verbs) Last(ctx context.Context, request LastRequest) (LastResult, error) {
	return Last(ctx, verbs.Runtime, request)
}

// Status is chat.Status over the bound runtime.
func (verbs Verbs) Status(ctx context.Context, request StatusRequest) (headless.Status, error) {
	warnings := verbs.Warnings
	if warnings == nil {
		warnings = io.Discard
	}
	return Status(ctx, verbs.Runtime, request, warnings)
}
