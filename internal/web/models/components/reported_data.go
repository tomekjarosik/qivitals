package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/qivitals/internal/web"
	"github.com/tomekjarosik/qivitals/internal/web/models"
)

type ReportedData struct {
	Data     models.ReportedDataView
	renderer web.Renderer
}

func NewReportedData(data models.ReportedDataView, r web.Renderer) *ReportedData {
	return &ReportedData{Data: data, renderer: r}
}

func (c *ReportedData) Render(ctx context.Context, w io.Writer) error {
	return c.renderer.Render(ctx, w, "reported-data", c.Data)
}
