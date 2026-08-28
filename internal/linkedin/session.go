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

func NewSession(liAt, jsessionID, bcookie, bscookie, lidc string) (*Session, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("linkedin: creating cookie jar: %w", err)
	}

	cleanJS := strings.Trim(jsessionID, `"`)
	cookies := []*http.Cookie{
		{Name: "li_at", Value: liAt, Path: "/", Domain: ".linkedin.com", Secure: true, HttpOnly: true},
		{Name: "JSESSIONID", Value: cleanJS, Path: "/", Domain: ".linkedin.com", Secure: true},
	}
	if bcookie != "" {
		cookies = append(cookies, &http.Cookie{Name: "bcookie", Value: bcookie, Path: "/", Domain: ".linkedin.com", Secure: true})
	}
	if bscookie != "" {
		cookies = append(cookies, &http.Cookie{Name: "bscookie", Value: bscookie, Path: "/", Domain: ".www.linkedin.com", Secure: true, HttpOnly: true})
	}
	if lidc != "" {
		cookies = append(cookies, &http.Cookie{Name: "lidc", Value: lidc, Path: "/", Domain: ".linkedin.com", Secure: true})
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
