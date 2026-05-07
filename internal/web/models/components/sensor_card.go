package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/qivitals/internal/web"
	"github.com/tomekjarosik/qivitals/internal/web/models"
)

type SensorCard struct {
	Data     models.SensorCardView
	renderer web.Renderer
}

func NewSensorCard(data models.SensorCardView, r web.Renderer) *SensorCard {
	return &SensorCard{Data: data, renderer: r}
}
func (c *SensorCard) Render(ctx context.Context, w io.Writer) error {
	return c.renderer.Render(ctx, w, "sensor-card", c.Data)
}
