package web

import (
	"html/template"
	"strings"
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
	}
}
