package server

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"regexp"
	"strings"

	assets "github.com/regutierrez/traicr/web"
)

// spaDocument is the built SvelteKit shell with the Content-Security-Policy it needs.
// Both are fixed for the life of the process, so they are computed once at startup.
type spaDocument struct {
	html   []byte
	policy string
}

const fallbackSPA = `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Traicr</title></head><body><p>Build the UI in web/app to use the browser pages.</p></body></html>`

var inlineScriptPattern = regexp.MustCompile(`(?s)<script\b([^>]*)>(.*?)</script>`)

func loadSPA() spaDocument {
	html, err := assets.Files.ReadFile("static/ui/index.html")
	if err != nil {
		html = []byte(fallbackSPA)
	}
	return spaDocument{html: html, policy: spaPolicy(string(html))}
}

func (app *application) spa(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Security-Policy", app.ui.policy)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(app.ui.html)
}

// spaPolicy relaxes the global policy only as far as the SvelteKit shell needs: its inline
// bootstrap script is allowed by hash instead of unsafe-inline, and attribute styles are
// allowed because Svelte sets them at runtime.
func spaPolicy(html string) string {
	scriptSrc := []string{"'self'"}
	for _, match := range inlineScriptPattern.FindAllStringSubmatch(html, -1) {
		if strings.Contains(strings.ToLower(match[1]), "src=") {
			continue
		}
		scriptSrc = append(scriptSrc, cspHash(match[2]))
	}
	return "default-src 'none'; script-src " + strings.Join(scriptSrc, " ") +
		"; style-src 'self'; style-src-attr 'unsafe-inline'; img-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"
}

func cspHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

func immutableAssets(files http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SvelteKit hashes everything under /_app/immutable, so it is safe to cache forever.
		if strings.HasPrefix(r.URL.Path, "/_app/immutable/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

func apiNotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "unknown API route")
}
