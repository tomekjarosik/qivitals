package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/one-status/internal/view"
)

type SensorGrid struct {
	Data view.SensorGridData
}

func NewSensorGrid(data view.SensorGridData) *SensorGrid {
	return &SensorGrid{Data: data}
}

func (c *SensorGrid) Render(ctx context.Context, w io.Writer) error {
	return view.Templates.ExecuteTemplate(w, "sensor-grid", c.Data)
}
