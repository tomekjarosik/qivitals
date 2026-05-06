package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"slices"
	"strings"
	"time"
)

//go:embed templates/**/*.html
var TemplateFS embed.FS

// Renderer is the engine responsible for executing templates.
type Renderer interface {
	Render(ctx context.Context, w io.Writer, templateName string, data interface{}) error
}

func NewTemplateRenderer() *TemplateRenderer {
	tmpl, err := initTemplates()
	if err != nil {
		log.Fatal(err)
	}
	return &TemplateRenderer{
		templates: tmpl,
	}
}

// TemplateRenderer is the concrete implementation using html/template.
type TemplateRenderer struct {
	templates *template.Template
}

func (te *TemplateRenderer) Render(ctx context.Context, w io.Writer, name string, data interface{}) error {
	return te.templates.ExecuteTemplate(w, name, data)
}

// initTemplates parses the embedded templates and returns a configured template object.
func initTemplates() (*template.Template, error) {
	// Use fs.Sub to allow templates to be referenced as "file.html" instead of "templates/file.html"
	subFS, err := fs.Sub(TemplateFS, "templates")
	if err != nil {
		return nil, fmt.Errorf("failed to create sub-filesystem: %w", err)
	}

	funcs := template.FuncMap{
		"timeAgo": timeAgo,
		"contains": func(slice []string, target string) bool {
			return slices.Contains(slice, target)
		},
		"join": strings.Join,
	}

	tmpl, err := template.New("").Funcs(funcs).ParseFS(subFS, "**/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return tmpl, nil
}

func timeAgo(ts int64) string {
	if ts == 0 {
		return "Never"
	}
	t := time.Unix(ts, 0)
	dur := time.Since(t)
	switch {
	case dur < time.Minute:
		return "just now"
	case dur < time.Hour:
		minutes := int(dur.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case dur < 24*time.Hour:
		hours := int(dur.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case dur < 48*time.Hour:
		return "yesterday"
	default:
		days := int(dur.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	}
}
