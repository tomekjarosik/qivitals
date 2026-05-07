package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/qivitals/internal/web"
	"github.com/tomekjarosik/qivitals/internal/web/models"
)

type SensorGrid struct {
	Data     models.SensorGridData
	renderer web.Renderer
}

func NewSensorGrid(data models.SensorGridData, r web.Renderer) *SensorGrid {
	return &SensorGrid{Data: data, renderer: r}
}

func (c *SensorGrid) Render(ctx context.Context, w io.Writer) error {
	return c.renderer.Render(ctx, w, "sensor-grid", c.Data)
}
