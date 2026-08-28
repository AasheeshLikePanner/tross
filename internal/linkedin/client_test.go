package linkedin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tross/linkedin-profile-api/internal/linkedin"
)

func TestClient_FetchProfile_FullSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/in/") {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<!DOCTYPE html><html><head>
<title>Jane Doe | LinkedIn</title>
<a href="/messaging/compose/?profileUrn=urn%3Ali%3Afsd_profile%3AACoAATestJaneDoe1234567">Msg</a>
</head></html>`))
			return
		}

		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "actions/component") {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`0:["$","div",null,{"children":["Sample Data"]}]`))
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	session, err := linkedin.NewSession("mock_li_at", `"ajax:mock"`, "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	client := linkedin.NewClientWithSession(session, server.URL)
	client.SetPacingInterval(0) // No delay in tests

	p, err := client.FetchProfile(context.Background(), "janedoe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.FullName != "Jane Doe" {
		t.Errorf("expected FullName Jane Doe, got %q", p.FullName)
	}
	if p.VieweeProfileID != "ACoAATestJaneDoe1234567" {
		t.Errorf("expected VieweeProfileID ACoAATestJaneDoe1234567, got %q", p.VieweeProfileID)
	}
}

func TestClient_FetchProfile_AuthExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	session, _ := linkedin.NewSession("mock_li_at", `"ajax:mock"`, "", "", "")
	client := linkedin.NewClientWithSession(session, server.URL)
	client.SetPacingInterval(0)

	_, err := client.FetchProfile(context.Background(), "janedoe")
	if err != linkedin.ErrSessionExpired {
		t.Errorf("expected ErrSessionExpired, got %v", err)
	}
}

func TestClient_FetchProfile_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	session, _ := linkedin.NewSession("mock_li_at", `"ajax:mock"`, "", "", "")
	client := linkedin.NewClientWithSession(session, server.URL)
	client.SetPacingInterval(0)

	_, err := client.FetchProfile(context.Background(), "janedoe")
	if err != linkedin.ErrRateLimited {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestClient_FetchProfile_5xxUpstream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	session, _ := linkedin.NewSession("mock_li_at", `"ajax:mock"`, "", "", "")
	client := linkedin.NewClientWithSession(session, server.URL)
	client.SetPacingInterval(0)

	_, err := client.FetchProfile(context.Background(), "janedoe")
	if err != linkedin.ErrLinkedInUnavailable {
		t.Errorf("expected ErrLinkedInUnavailable, got %v", err)
	}
}

func TestClient_FetchProfile_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	session, _ := linkedin.NewSession("mock_li_at", `"ajax:mock"`, "", "", "")
	client := linkedin.NewClientWithSession(session, server.URL)
	client.SetPacingInterval(0)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := client.FetchProfile(ctx, "janedoe")
	if err == nil {
		t.Fatal("expected context deadline error, got nil")
	}
}

func TestClient_Semaphore_Serialization(t *testing.T) {
	var (
		activeCount int32
		maxActive   int32
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		curr := atomic.AddInt32(&activeCount, 1)
		for {
			oldMax := atomic.LoadInt32(&maxActive)
			if curr <= oldMax || atomic.CompareAndSwapInt32(&maxActive, oldMax, curr) {
				break
			}
		}
		// Hold the request briefly to simulate processing time
		time.Sleep(60 * time.Millisecond)
		atomic.AddInt32(&activeCount, -1)

		if r.Method == http.MethodGet {
			w.Write([]byte(`<html><head><title>User | LinkedIn</title><a href="/messaging/compose/?profileUrn=urn:li:fsd_profile:ACoAA123">M</a></head></html>`))
			return
		}
		w.Write([]byte(`0:["$","div",null,{}]`))
	}))
	defer server.Close()

	session, _ := linkedin.NewSession("mock_li_at", `"ajax:mock"`, "", "", "")
	client := linkedin.NewClientWithSession(session, server.URL)
	client.SetPacingInterval(0)

	var wg sync.WaitGroup
	start := time.Now()

	// Launch two concurrent profile fetches
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = client.FetchProfile(context.Background(), "user")
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	if max := atomic.LoadInt32(&maxActive); max > 1 {
		t.Errorf("expected max concurrent active requests to be 1, got %d", max)
	}
	// Two sequential runs with multiple requests each must take at least 100ms
	if duration < 100*time.Millisecond {
		t.Errorf("expected serialization duration >= 100ms, took %v", duration)
	}
}

func TestClient_Semaphore_CancelledWaiter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(80 * time.Millisecond)
		w.Write([]byte(`<html><head><title>User | LinkedIn</title><a href="/messaging/compose/?profileUrn=urn:li:fsd_profile:ACoAA123">M</a></head></html>`))
	}))
	defer server.Close()

	session, _ := linkedin.NewSession("mock_li_at", `"ajax:mock"`, "", "", "")
	client := linkedin.NewClientWithSession(session, server.URL)
	client.SetPacingInterval(0)

	// First goroutine grabs the semaphore
	started := make(chan struct{})
	go func() {
		close(started)
		_, _ = client.FetchProfile(context.Background(), "user1")
	}()

	<-started
	time.Sleep(10 * time.Millisecond) // Ensure first call has acquired semaphore

	// Second goroutine tries with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := client.FetchProfile(ctx, "user2")
	if err == nil {
		t.Fatal("expected context deadline error for waiting request, got nil")
	}
}

func TestClient_Semaphore_ReleasesOnError(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusBadGateway) // First call fails
			return
		}
		w.Write([]byte(`<html><head><title>OK | LinkedIn</title><a href="/messaging/compose/?profileUrn=urn:li:fsd_profile:ACoAA123">M</a></head></html>`))
	}))
	defer server.Close()

	session, _ := linkedin.NewSession("mock_li_at", `"ajax:mock"`, "", "", "")
	client := linkedin.NewClientWithSession(session, server.URL)
	client.SetPacingInterval(0)

	// Call 1 fails
	_, err1 := client.FetchProfile(context.Background(), "user1")
	if err1 == nil {
		t.Fatal("expected error on call 1")
	}

	// Call 2 must succeed immediately (proves semaphore was released despite error)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	p, err2 := client.FetchProfile(ctx, "user2")
	if err2 != nil {
		t.Fatalf("call 2 failed to acquire released semaphore: %v", err2)
	}
	if p == nil || p.FullName != "OK" {
		t.Errorf("unexpected profile on call 2: %+v", p)
	}
}
