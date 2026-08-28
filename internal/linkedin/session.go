package linkedin

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
)

var linkedInBaseURL = mustParseURL("https://www.linkedin.com")

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

type Session struct {
	mu  sync.RWMutex
	jar *cookiejar.Jar
}

func cleanCookieValue(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, `"'`)
	v = strings.Trim(v, `"'`)
	return strings.TrimSpace(v)
}

func NewSession(liAt, jsessionID, bcookie, bscookie, lidc string) (*Session, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("linkedin: creating cookie jar: %w", err)
	}

	cleanLiAt := cleanCookieValue(liAt)
	cleanJS := cleanCookieValue(jsessionID)

	cookies := []*http.Cookie{
		{Name: "li_at", Value: cleanLiAt, Path: "/", Domain: ".linkedin.com", Secure: true, HttpOnly: true},
		{Name: "JSESSIONID", Value: cleanJS, Path: "/", Domain: ".linkedin.com", Secure: true},
	}
	if b := cleanCookieValue(bcookie); b != "" {
		cookies = append(cookies, &http.Cookie{Name: "bcookie", Value: b, Path: "/", Domain: ".linkedin.com", Secure: true})
	}
	if bs := cleanCookieValue(bscookie); bs != "" {
		cookies = append(cookies, &http.Cookie{Name: "bscookie", Value: bs, Path: "/", Domain: ".www.linkedin.com", Secure: true, HttpOnly: true})
	}
	if l := cleanCookieValue(lidc); l != "" {
		cookies = append(cookies, &http.Cookie{Name: "lidc", Value: l, Path: "/", Domain: ".linkedin.com", Secure: true})
	}

	jar.SetCookies(linkedInBaseURL, cookies)
	return &Session{jar: jar}, nil
}

func (s *Session) Jar() *cookiejar.Jar {
	return s.jar
}

func (s *Session) CSRFToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.jar.Cookies(linkedInBaseURL) {
		if c.Name == "JSESSIONID" {
			return strings.Trim(c.Value, `"`)
		}
	}
	return ""
}

func (s *Session) HasAuth() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.jar.Cookies(linkedInBaseURL) {
		if c.Name == "li_at" && c.Value != "" && c.Value != "delete me" {
			return true
		}
	}
	return false
}

func (s *Session) InspectResponse(resp *http.Response) error {
	for _, h := range resp.Header["Set-Cookie"] {
		if strings.Contains(h, "li_at") && (strings.Contains(h, "Max-Age=0") || strings.Contains(h, "delete me")) {
			return ErrSessionExpired
		}
	}

	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		loc := resp.Header.Get("Location")
		switch {
		case strings.Contains(loc, "/checkpoint"):
			return ErrAuthChallenge
		case strings.Contains(loc, "/login") || strings.Contains(loc, "/authwall"):
			return ErrSessionExpired
		}
	}

	return nil
}
