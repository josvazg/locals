package render

import (
	"bytes"
	"embed"
	"fmt"
	"path/filepath"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// State represents the variables required to render all scripts.
type State struct {
	DNSListen string
	LocalsDir string
	SystemCA  string
}

func renderTemplate(s State, name string) ([]byte, error) {
	tmpl, err := template.ParseFS(templateFS, filepath.Join("templates/", name))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func On(s State) ([]byte, error) {
	return renderTemplate(s, "on.sh.tmpl")
}

func Off(s State) ([]byte, error) {
	return renderTemplate(s, "off.sh.tmpl")
}

func Add(s State) ([]byte, error) {
	return renderTemplate(s, "add.sh.tmpl")
}

func Remove(s State) ([]byte, error) {
	return renderTemplate(s, "rm.sh.tmpl")
}
