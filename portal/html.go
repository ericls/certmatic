package portal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"text/template"
)

// antiFlickerScript reads the stored theme from localStorage and applies the "dark"
// class to <html> synchronously before first paint, preventing a white flash on
// page load when dark mode is active.
const antiFlickerScript = `<script>
  (function(){
    var t=localStorage.getItem('certmatic-theme');
    if(t==='dark'||(t!=='light'&&window.matchMedia('(prefers-color-scheme:dark)').matches))
      document.documentElement.classList.add('dark');
  })();
</script>`

// this value needs to be updated in index.css too
const veryBasicCSS = `
html.dark {
  background-color: #111827; /* gray-900 */
}
`

var htmlTmpl = template.Must(template.New("portal").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Certmatic Portal</title>
  <style>` + veryBasicCSS + `</style>
` + antiFlickerScript + `
  <link rel="stylesheet" crossorigin href="{{.AssetsBase}}main.css?v={{.Version}}">
</head>
<body>
  <div id="root"></div>
  <script type="module" crossorigin src="{{.AssetsBase}}main.js?v={{.Version}}"></script>
</body>
</html>`))

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
