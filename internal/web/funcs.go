package web

import (
	"fmt"
	"html/template"
	"strings"
	"time"
)

// templateFuncs returns a FuncMap with custom functions used in templates.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		// contains checks whether item is in the slice.
		"contains": func(slice []string, item string) bool {
			for _, s := range slice {
				if s == item {
					return true
				}
			}
			return false
		},
		// join concatenates strings with a separator.
		"join": func(items []string, sep string) string {
			return strings.Join(items, sep)
		},
		"timeAgo": timeAgo,
	}
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
