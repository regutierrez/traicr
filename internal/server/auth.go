package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const sessionCookie = "traicr_session"

func (app *application) sign(value string) string {
	hash := hmac.New(sha256.New, []byte(app.config.AdminToken))
	hash.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(hash.Sum(nil))
}

func (app *application) validToken(token string) bool {
	want := sha256.Sum256([]byte(app.config.AdminToken))
	got := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

func (app *application) api(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, present := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if present && app.validToken(token) {
			next(w, r)
			return
		}
		// The browser UI sends the session cookie. Mutations stay bearer-only so a page cannot be driven into an import.
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && app.session(r, "session") != "" {
			next(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, "unauthorized", "a valid bearer token is required")
	})
}

func (app *application) session(r *http.Request, purpose string) string {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 4 || parts[0] != purpose {
		return ""
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() >= expires {
		return ""
	}
	if !hmac.Equal([]byte(parts[3]), []byte(app.sign(strings.Join(parts[:3], ".")))) {
		return ""
	}
	return cookie.Value
}

func (app *application) setSession(w http.ResponseWriter, purpose string) string {
	expires := time.Now().Add(24 * time.Hour)
	value := purpose + "." + strconv.FormatInt(expires.Unix(), 10) + "." + rand.Text()
	value += "." + app.sign(value)
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: value, Path: "/", HttpOnly: true, Secure: app.config.SecureCookies, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: 86400})
	return value
}

func (app *application) csrf(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return app.sign("csrf:" + cookie.Value)
}

func (app *application) validForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_form", "invalid or oversized form")
		return false
	}
	// Never trust forwarded headers. The proxy must preserve the original Host.
	scheme := "http"
	if app.config.SecureCookies || r.TLS != nil {
		scheme = "https"
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		referrer, err := url.Parse(r.Referer())
		if err == nil && referrer.Host != "" {
			origin = referrer.Scheme + "://" + referrer.Host
		}
	}
	if origin != scheme+"://"+r.Host || r.Header.Get("Sec-Fetch-Site") == "cross-site" || !hmac.Equal([]byte(r.PostForm.Get("csrf")), []byte(app.csrf(r))) || r.PostForm.Get("csrf") == "" {
		writeError(w, http.StatusForbidden, "csrf", "form expired or came from another site; reload and try again")
		return false
	}
	return true
}

func (app *application) browser(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.session(r, "session") == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !app.validForm(w, r) {
			return
		}
		next(w, r)
	})
}

func (app *application) loginPage(w http.ResponseWriter, r *http.Request) {
	if app.session(r, "session") != "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	// Reuse a live login cookie so a second GET /login cannot invalidate the
	// CSRF token already rendered in the Svelte form.
	if app.session(r, "login") == "" {
		value := app.setSession(w, "login")
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: value})
		r.Header.Set("Cookie", sessionCookie+"="+value)
	}
	app.spa(w, r)
}

func (app *application) csrfAPI(w http.ResponseWriter, r *http.Request) {
	// If the browser already has a session cookie, sign that value. Minting a
	// second cookie here desyncs the form token when the Set-Cookie is dropped
	// (SvelteKit's load fetch) or arrives after the page has rendered.
	if _, err := r.Cookie(sessionCookie); err != nil && app.session(r, "session") == "" && app.session(r, "login") == "" {
		value := app.setSession(w, "login")
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: value})
		r.Header.Set("Cookie", sessionCookie+"="+value)
	}
	writeJSON(w, map[string]string{"csrf": app.csrf(r)})
}

func (app *application) login(w http.ResponseWriter, r *http.Request) {
	if app.session(r, "login") == "" || !app.validForm(w, r) {
		if app.session(r, "login") == "" {
			writeError(w, http.StatusForbidden, "login_expired", "reload the login page and try again")
		}
		return
	}
	if !app.validToken(r.PostForm.Get("token")) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid admin token")
		return
	}
	app.setSession(w, "session")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: app.config.SecureCookies, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
