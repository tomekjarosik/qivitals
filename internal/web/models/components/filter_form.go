package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/one-status/internal/web"
	"github.com/tomekjarosik/one-status/internal/web/models"
)

type FilterForm struct {
	Data     models.FilterView
	renderer web.Renderer
}

func NewFilterForm(data models.FilterView, r web.Renderer) *FilterForm {
	return &FilterForm{Data: data, renderer: r}
}

func (c *FilterForm) Render(ctx context.Context, w io.Writer) error {
	return c.renderer.Render(ctx, w, "filter-form", c.Data)
}
