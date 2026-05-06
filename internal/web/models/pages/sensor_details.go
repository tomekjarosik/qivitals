package pages

import (
	"context"
	"io"

	"github.com/tomekjarosik/one-status/internal/web"
	"github.com/tomekjarosik/one-status/internal/web/models"
)

type SensorDetailPage struct {
	Data     models.SensorDetailPageView
	renderer web.Renderer
}

func NewSensorDetailPage(data models.SensorDetailPageView, r web.Renderer) *SensorDetailPage {
	return &SensorDetailPage{Data: data, renderer: r}
}

func (c *SensorDetailPage) Render(ctx context.Context, w io.Writer) error {
	return c.renderer.Render(ctx, w, "sensor-detail-page", c.Data)
}
