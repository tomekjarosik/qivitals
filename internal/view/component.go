package view

import (
	"context"
	"html/template"
	"io"
)

// Component is any piece of UI that can render itself.
type Component interface {
	Render(ctx context.Context, w io.Writer) error
}

// Templates hold the parsed template set. Set once during startup.
var Templates *template.Template

// Init sets the compiled templates for the whole view layer.
func Init(t *template.Template) {
	Templates = t
}
