//go:build !Develop

package view

import (
	_ "embed"
	"errors"
	"html/template"
)

//go:embed Views/menu.gohtml
var menu string

//go:embed Views/viewer.gohtml
var viewer string

//go:embed Views/login.gohtml
var login string

func GetViewTemplate(view View) (*template.Template, error) {
	switch view {
	case Menu:
		return template.New("menu").Parse(menu)
	case Viewer:
		return template.New("viewer").Parse(viewer)
  case Login:
    return template.New("login").Parse(login)
	}
	return nil, errors.New("invalid view")
}
