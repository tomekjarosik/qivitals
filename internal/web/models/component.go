package models

import (
	"context"
	"io"
)

// Component is any piece of UI that can render itself.
type Component interface {
	Render(ctx context.Context, w io.Writer) error
}
