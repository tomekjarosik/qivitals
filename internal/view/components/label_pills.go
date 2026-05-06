package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/one-status/internal/view"
)

type LabelPills struct {
	Data view.LabelPillsView
}

func NewLabelPills(data view.LabelPillsView) *LabelPills { return &LabelPills{Data: data} }

func (c *LabelPills) Render(ctx context.Context, w io.Writer) error {
	return view.Templates.ExecuteTemplate(w, "label-pills", c.Data)
}
