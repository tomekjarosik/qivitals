package components

import (
	"context"
	"html/template"
	"io"

	"github.com/tomekjarosik/qivitals/internal/web"
)

type EmptyState struct {
	Title       string
	Description string
	IconSVG     template.HTML // allow raw SVG
	renderer    web.Renderer
}

func NewEmptyState(title, desc string, r web.Renderer) *EmptyState {
	return &EmptyState{
		Title:       title,
		Description: desc,
		IconSVG:     `<svg class="mx-auto h-16 w-16 text-slate-300" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" /></svg>`,
		renderer:    r,
	}
}

func (c *EmptyState) Render(ctx context.Context, w io.Writer) error {
	return c.renderer.Render(ctx, w, "empty-state", c)
}
