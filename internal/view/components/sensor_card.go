package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/one-status/internal/view"
)

type SensorCard struct {
	Data view.SensorCardView
}

func NewSensorCard(data view.SensorCardView) *SensorCard { return &SensorCard{Data: data} }
func (c *SensorCard) Render(ctx context.Context, w io.Writer) error {
	return view.Templates.ExecuteTemplate(w, "sensor-card", c.Data)
}
