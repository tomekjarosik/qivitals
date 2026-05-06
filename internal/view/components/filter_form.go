package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/one-status/internal/view"
)

type FilterForm struct {
	Data view.FilterView
}

func NewFilterForm(data view.FilterView) *FilterForm { return &FilterForm{Data: data} }

func (c *FilterForm) Render(ctx context.Context, w io.Writer) error {
	return view.Templates.ExecuteTemplate(w, "filter-form", c.Data)
}
