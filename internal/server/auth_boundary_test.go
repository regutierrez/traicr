package server_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBrowserSessionExpiryAndPurposeBoundaries(t *testing.T) {
	handler := testHandler(t, false)
	if remaining := time.Second - time.Duration(time.Now().Nanosecond()); remaining < 100*time.Millisecond {
		time.Sleep(remaining)
	}
	now := time.Now().Unix()
	expired := signedCookie("session", now)
	request := httptest.NewRequest(http.MethodGet, "/machines", nil)
	request.AddCookie(expired)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatalf("session expiring now was accepted: %d", response.Code)
	}

	loginPurpose := signedCookie("login", now+60)
	request = httptest.NewRequest(http.MethodGet, "/machines", nil)
	request.AddCookie(loginPurpose)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("login-purpose cookie authorized a browser page: %d", response.Code)
	}

	sessionPurpose := signedCookie("session", now+60)
	form := url.Values{"csrf": {csrfFor(sessionPurpose)}, "token": {adminToken}}
	response = submit(handler, "/login", form, sessionPurpose, "http://example.com")
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"login_expired"`) {
		t.Fatalf("session-purpose cookie authorized login: %d %s", response.Code, response.Body)
	}
}

func TestLoginAcceptsSameOriginRefererWithoutOrigin(t *testing.T) {
	handler := testHandler(t, false)
	cookie, csrf := freshLoginForm(t, handler)
	form := url.Values{"csrf": {csrf}, "token": {adminToken}}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Referer", "http://example.com/login")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("same-origin Referer rejected: %d %s", response.Code, response.Body)
	}
}

func TestLoginRejectsInvalidReferrersAndForwardedHeaderSpoofing(t *testing.T) {
	handler := testHandler(t, false)
	for name, configure := range map[string]func(*http.Request){
		"origin null": func(request *http.Request) {
			request.Header.Set("Origin", "null")
		},
		"malformed referer": func(request *http.Request) {
			request.Header.Set("Referer", "%")
		},
		"hostless referer": func(request *http.Request) {
			request.Header.Set("Referer", "/login")
		},
		"cross-origin referer": func(request *http.Request) {
			request.Header.Set("Referer", "https://attacker.invalid/login")
		},
		"forwarded host spoof": func(request *http.Request) {
			request.Header.Set("Origin", "http://attacker.invalid")
			request.Header.Set("Forwarded", "host=attacker.invalid")
			request.Header.Set("X-Forwarded-Host", "attacker.invalid")
		},
		"forwarded scheme spoof": func(request *http.Request) {
			request.Header.Set("Origin", "https://example.com")
			request.Header.Set("Forwarded", "proto=https")
			request.Header.Set("X-Forwarded-Proto", "https")
		},
	} {
		t.Run(name, func(t *testing.T) {
			cookie, csrf := freshLoginForm(t, handler)
			form := url.Values{"csrf": {csrf}, "token": {adminToken}}
			request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.AddCookie(cookie)
			configure(request)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"csrf"`) {
				t.Fatalf("invalid source accepted: %d %s", response.Code, response.Body)
			}
		})
	}
}

func freshLoginForm(t *testing.T, handler http.Handler) (*http.Cookie, string) {
	t.Helper()
	response := request(handler, http.MethodGet, "/login", nil, "")
	match := regexp.MustCompile(`name="csrf" value="([^"]+)"`).FindStringSubmatch(response.Body.String())
	if response.Code != http.StatusOK || len(match) != 2 || len(response.Result().Cookies()) != 1 {
		t.Fatalf("login page: %d %s", response.Code, response.Body)
	}
	return response.Result().Cookies()[0], match[1]
}

func signedCookie(purpose string, expires int64) *http.Cookie {
	value := purpose + "." + strconv.FormatInt(expires, 10) + ".boundary"
	return &http.Cookie{Name: "traicr_session", Value: value + "." + testSignature(value)}
}

func csrfFor(cookie *http.Cookie) string {
	return testSignature("csrf:" + cookie.Value)
}

func testSignature(value string) string {
	hash := hmac.New(sha256.New, []byte(adminToken))
	_, _ = hash.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(hash.Sum(nil))
}
