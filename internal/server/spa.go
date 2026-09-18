package server

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"regexp"
	"strings"

	assets "github.com/regutierrez/traicr/web"
)

const fallbackSPA = `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Traicr</title></head><body><input type="hidden" name="csrf" value="__TRAICR_CSRF__"></body></html>`

var inlineScriptPattern = regexp.MustCompile(`(?s)<script\b([^>]*)>(.*?)</script>`)

func (app *application) spa(w http.ResponseWriter, r *http.Request) {
	data, err := assets.Files.ReadFile("static/ui/index.html")
	if err != nil {
		data = []byte(fallbackSPA)
	}
	body := strings.ReplaceAll(string(data), "__TRAICR_CSRF__", app.csrf(r))
	// The global policy forbids unsafe-inline. Hash this document's bootstrap script instead.
	w.Header().Set("Content-Security-Policy", spaPolicy(body))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(body))
}

func spaPolicy(html string) string {
	scriptSrc := []string{"'self'"}
	for _, match := range inlineScriptPattern.FindAllStringSubmatch(html, -1) {
		if strings.Contains(strings.ToLower(match[1]), "src=") {
			continue
		}
		scriptSrc = append(scriptSrc, cspHash(match[2]))
	}
	// style-src stays 'self'. Attribute styles are separate: Svelte sets them at runtime, and
	// that relaxation is only on this document. The transcript viewer keeps the global policy.
	policy := "default-src 'none'; script-src " + strings.Join(scriptSrc, " ") + "; style-src 'self'; style-src-attr 'unsafe-inline'"
	return policy + "; img-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"
}

func cspHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

func (app *application) sourceOrSpa(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("download") == "1" {
		app.sourceFile(w, r)
		return
	}
	app.spa(w, r)
}
