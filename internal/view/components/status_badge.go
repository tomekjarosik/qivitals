// internal/view/components/status_badge.go
package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/one-status/internal/view"
)

type StatusBadge struct {
	Data view.StatusBadgeView
}

func NewStatusBadge(data view.StatusBadgeView) *StatusBadge {
	return &StatusBadge{Data: data}
}

func (c *StatusBadge) Render(ctx context.Context, w io.Writer) error {
	return view.Templates.ExecuteTemplate(w, "status-badge", c.Data)
}
