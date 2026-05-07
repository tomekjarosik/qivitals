package pages

import (
	"context"
	"io"

	"github.com/tomekjarosik/qivitals/internal/web"
	"github.com/tomekjarosik/qivitals/internal/web/models"
)

type DashboardPage struct {
	Data     models.DashboardPageView
	renderer web.Renderer
}

func NewDashboardPage(data models.DashboardPageView, r web.Renderer) *DashboardPage {
	return &DashboardPage{Data: data, renderer: r}
}
func (c *DashboardPage) Render(ctx context.Context, w io.Writer) error {
	return c.renderer.Render(ctx, w, "dashboard-page", c.Data)
}
