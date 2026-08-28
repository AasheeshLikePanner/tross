package linkedin

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var slugRe = regexp.MustCompile(`^[a-zA-Z0-9\-_%]+$`)

func ParseProfileURL(raw string) (slug, normalizedURL string, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", fmt.Errorf("invalid url")
	}

	host := strings.ToLower(u.Hostname())
	if host != "linkedin.com" && host != "www.linkedin.com" {
		return "", "", fmt.Errorf("not a linkedin.com host")
	}

	path := strings.TrimSuffix(u.Path, "/")
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) < 2 || parts[0] != "in" || parts[1] == "" {
		return "", "", fmt.Errorf("not a /in/ profile url")
	}

	s := parts[1]
	if !slugRe.MatchString(s) {
		return "", "", fmt.Errorf("invalid slug")
	}

	return s, fmt.Sprintf("https://www.linkedin.com/in/%s/", s), nil
}
