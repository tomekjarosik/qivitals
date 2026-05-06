package components

import (
	"context"
	"io"

	"github.com/tomekjarosik/one-status/internal/view"
)

type DashboardPage struct {
	Data view.DashboardPageView
}

func NewDashboardPage(data view.DashboardPageView) *DashboardPage { return &DashboardPage{Data: data} }
func (c *DashboardPage) Render(ctx context.Context, w io.Writer) error {
	return view.Templates.ExecuteTemplate(w, "dashboard-page", c.Data)
}
