// internal/view/components/status_badge.go
package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/qivitals/internal/web"
	"github.com/tomekjarosik/qivitals/internal/web/models"
)

type StatusBadge struct {
	Data     models.StatusBadgeView
	renderer web.Renderer
}

func NewStatusBadge(data models.StatusBadgeView, r web.Renderer) *StatusBadge {
	return &StatusBadge{Data: data, renderer: r}
}

func (c *StatusBadge) Render(ctx context.Context, w io.Writer) error {
	return c.renderer.Render(ctx, w, "status-badge", c.Data)
}
