package linkedin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/tross/linkedin-profile-api/internal/linkedin"
)

func TestSession_SeedingAndCSRF(t *testing.T) {
	session, err := linkedin.NewSession(
		"test_li_at_val",
		`"ajax:123456789"`,
		"v=2&test-bcookie",
		"v=1&test-bscookie",
		"b=test:lidc",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !session.HasAuth() {
		t.Error("expected session to have auth")
	}

	if csrf := session.CSRFToken(); csrf != "ajax:123456789" {
		t.Errorf("expected clean CSRF token ajax:123456789, got %q", csrf)
	}
}

func TestSession_CookieUpdates(t *testing.T) {
	session, err := linkedin.NewSession("init_li_at", `"ajax:111"`, "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	jar := session.Jar()
	u, _ := url.Parse("https://www.linkedin.com")

	// Simulate LinkedIn sending Set-Cookie with updated JSESSIONID
	jar.SetCookies(u, []*http.Cookie{
		{Name: "JSESSIONID", Value: `"ajax:999999"`, Path: "/", Domain: ".linkedin.com"},
	})

	if csrf := session.CSRFToken(); csrf != "ajax:999999" {
		t.Errorf("expected updated CSRF token ajax:999999, got %q", csrf)
	}
}

func TestSession_InspectResponse_LiAtDeleted(t *testing.T) {
	session, _ := linkedin.NewSession("init_li_at", `"ajax:111"`, "", "", "")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Set-Cookie": []string{
				"li_at=delete me; Version=1; Path=/; Domain=.www.linkedin.com; Max-Age=0",
			},
		},
	}

	err := session.InspectResponse(resp)
	if err != linkedin.ErrSessionExpired {
		t.Errorf("expected ErrSessionExpired, got %v", err)
	}
}

func TestSession_InspectResponse_RedirectLogin(t *testing.T) {
	session, _ := linkedin.NewSession("init_li_at", `"ajax:111"`, "", "", "")

	resp := &http.Response{
		StatusCode: http.StatusFound,
		Header: http.Header{
			"Location": []string{"https://www.linkedin.com/login?session_redirect=..."},
		},
	}

	err := session.InspectResponse(resp)
	if err != linkedin.ErrSessionExpired {
		t.Errorf("expected ErrSessionExpired, got %v", err)
	}
}

func TestSession_InspectResponse_RedirectCheckpoint(t *testing.T) {
	session, _ := linkedin.NewSession("init_li_at", `"ajax:111"`, "", "", "")

	resp := &http.Response{
		StatusCode: http.StatusFound,
		Header: http.Header{
			"Location": []string{"https://www.linkedin.com/checkpoint/challenge/..."},
		},
	}

	err := session.InspectResponse(resp)
	if err != linkedin.ErrAuthChallenge {
		t.Errorf("expected ErrAuthChallenge, got %v", err)
	}
}
