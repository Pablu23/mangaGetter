//go:build Develop

package view

import (
	"html/template"
)

func GetViewTemplate(view View) (*template.Template, error) {
	var path string
	switch view {
	case Menu:
		path = "internal/view/Views/menu.gohtml"
	case Viewer:
		path = "internal/view/Views/viewer.gohtml"
	}
	return template.ParseFiles(path)
}
