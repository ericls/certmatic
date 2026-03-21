package portal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"text/template"
)

const webClientBase = "/web_client/portal/"

type viteManifestEntry struct {
	File    string   `json:"file"`
	CSS     []string `json:"css"`
	IsEntry bool     `json:"isEntry"`
}

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
{{- range .CSS}}
  <link rel="stylesheet" crossorigin href="{{$.Base}}{{.}}">
{{- end}}
</head>
<body>
  <div id="root"></div>
{{- range .JS}}
  <script type="module" crossorigin src="{{$.Base}}{{.}}"></script>
{{- end}}
</body>
</html>`))

type htmlData struct {
	Base string
	CSS  []string
	JS   []string
}

// GenerateProductionHTML reads the Vite manifest from fsys and returns a complete index.html.
func GenerateProductionHTML(fsys fs.FS) (string, error) {
	f, err := fsys.Open(".vite/manifest.json")
	if err != nil {
		return "", fmt.Errorf("open vite manifest: %w", err)
	}
	defer f.Close()

	var manifest map[string]viteManifestEntry
	if err := json.NewDecoder(f).Decode(&manifest); err != nil {
		return "", fmt.Errorf("decode vite manifest: %w", err)
	}

	data := htmlData{Base: webClientBase}
	for _, entry := range manifest {
		if entry.IsEntry {
			data.JS = append(data.JS, entry.File)
			data.CSS = append(data.CSS, entry.CSS...)
		}
	}

	var buf bytes.Buffer
	if err := htmlTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render HTML: %w", err)
	}
	return buf.String(), nil
}

// GenerateDevHTML returns an index.html for dev mode with Vite HMR scripts.
// Assets are loaded from /web_client/ (served by Caddy → Vite proxy), so
// there are no cross-origin issues.
func GenerateDevHTML() string {
	base := webClientBase
	return "<!DOCTYPE html>\n" +
		"<html lang=\"en\">\n" +
		"<head>\n" +
		"  <meta charset=\"UTF-8\" />\n" +
		"  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\" />\n" +
		"  <title>Certmatic Portal</title>\n" +
		"  <style>" + veryBasicCSS + "</style>\n" +
		antiFlickerScript + "\n" +
		"</head>\n" +
		"<body>\n" +
		"  <div id=\"root\"></div>\n" +
		"  <script type=\"module\" src=\"" + base + "@vite/client\"></script>\n" +
		"  <script type=\"module\">\n" +
		"    import RefreshRuntime from '" + base + "@react-refresh'\n" +
		"    RefreshRuntime.injectIntoGlobalHook(window)\n" +
		"    window.$RefreshReg$ = () => {}\n" +
		"    window.$RefreshSig$ = () => (type) => type\n" +
		"    window.__vite_plugin_react_preamble_installed__ = true\n" +
		"  </script>\n" +
		"  <script type=\"module\" src=\"" + base + "src/main.tsx\"></script>\n" +
		"</body>\n" +
		"</html>\n"
}

// ensure webClientBase ends with slash (compile-time check via usage)
var _ = strings.TrimSuffix(webClientBase, "/")
