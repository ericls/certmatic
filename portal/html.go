package portal

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"text/template"
)

//go:embed index.html.tmpl
var htmlTemplateFS embed.FS

var htmlTmpl = template.Must(template.ParseFS(htmlTemplateFS, "index.html.tmpl"))

// HTMLData holds the data passed to the portal HTML template.
// Add fields here to inject global variables or server-side config into the page.
type HTMLData struct {
	AssetsBase string // URL prefix for portal assets, e.g. "/portal/assets/"
	Version    string // cache-busting query param value
}

// GenerateHTML renders the portal index.html from the given data.
func GenerateHTML(data HTMLData) (string, error) {
	var buf bytes.Buffer
	if err := htmlTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ComputeVersion hashes the portal asset files to produce a short cache-busting token.
// Call this once at startup; the result is stable for the lifetime of the process.
func ComputeVersion(fsys fs.FS) string {
	h := sha256.New()
	for _, name := range []string{"main.js", "main.css"} {
		data, _ := fs.ReadFile(fsys, name)
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))[:8]
}
