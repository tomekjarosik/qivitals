package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/one-status/internal/web"
	"github.com/tomekjarosik/one-status/internal/web/models"
)

type LabelPills struct {
	Data     models.LabelPillsView
	renderer web.Renderer
}

func NewLabelPills(data models.LabelPillsView, r web.Renderer) *LabelPills {
	return &LabelPills{Data: data, renderer: r}
}

func (c *LabelPills) Render(ctx context.Context, w io.Writer) error {
	return c.renderer.Render(ctx, w, "label-pills", c.Data)
}
