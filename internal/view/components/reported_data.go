package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/one-status/internal/view"
)

type ReportedData struct {
	Data view.ReportedDataView
}

func NewReportedData(data view.ReportedDataView) *ReportedData { return &ReportedData{Data: data} }
func (c *ReportedData) Render(ctx context.Context, w io.Writer) error {
	return view.Templates.ExecuteTemplate(w, "reported-data", c.Data)
}
