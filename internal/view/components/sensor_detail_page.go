package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/one-status/internal/view"
)

type SensorDetailPage struct {
	Data view.SensorDetailPageView
}

func NewSensorDetailPage(data view.SensorDetailPageView) *SensorDetailPage {
	return &SensorDetailPage{Data: data}
}

func (c *SensorDetailPage) Render(ctx context.Context, w io.Writer) error {
	return view.Templates.ExecuteTemplate(w, "sensor-detail-page", c.Data)
}
