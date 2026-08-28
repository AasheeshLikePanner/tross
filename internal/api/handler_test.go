package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tross/linkedin-profile-api/internal/api"
	"github.com/tross/linkedin-profile-api/internal/linkedin"
	"github.com/tross/linkedin-profile-api/internal/profile"
)

type mockFetcher struct {
	profile *linkedin.InternalProfile
	err     error
}

func (m *mockFetcher) FetchProfile(ctx context.Context, slug string) (*linkedin.InternalProfile, error) {
	return m.profile, m.err
}

func postRequest(handler http.Handler, body string, contentType string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/profiles", bytes.NewBufferString(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else {
		req.Header.Set("Content-Type", "application/json")
	}
	handler.ServeHTTP(rec, req)
	return rec
}

func TestHandler_Success(t *testing.T) {
	fetcher := &mockFetcher{
		profile: &linkedin.InternalProfile{
			Slug:             "testuser",
			PublicIdentifier: "testuser",
			FullName:         "Test User",
			Headline:         "Software Engineer",
		},
	}

	h := api.NewHandler(fetcher)
	rec := postRequest(h.Routes(), `{"url":"https://www.linkedin.com/in/testuser/"}`, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var p profile.Profile
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if p.Name.Full != "Test User" {
		t.Errorf("expected FullName 'Test User', got %q", p.Name.Full)
	}
	if p.PublicIdentifier != "testuser" {
		t.Errorf("expected publicIdentifier 'testuser', got %q", p.PublicIdentifier)
	}
}

func TestHandler_InvalidURL(t *testing.T) {
	fetcher := &mockFetcher{}
	h := api.NewHandler(fetcher)

	badURLs := []string{
		`{"url":"https://twitter.com/in/test"}`,
		`{"url":"https://www.linkedin.com/company/test"}`,
		`{"url":""}`,
		`{"url":"not-a-url"}`,
	}

	for _, body := range badURLs {
		rec := postRequest(h.Routes(), body, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for %s, got %d", body, rec.Code)
		}
	}
}

func TestHandler_WrongContentType(t *testing.T) {
	fetcher := &mockFetcher{}
	h := api.NewHandler(fetcher)

	rec := postRequest(h.Routes(), `{"url":"https://www.linkedin.com/in/test/"}`, "text/plain")
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("expected 415, got %d", rec.Code)
	}
}

func TestHandler_SessionExpired(t *testing.T) {
	fetcher := &mockFetcher{err: linkedin.ErrSessionExpired}
	h := api.NewHandler(fetcher)

	rec := postRequest(h.Routes(), `{"url":"https://www.linkedin.com/in/test/"}`, "")
	if rec.Code != http.StatusFailedDependency {
		t.Errorf("expected 424, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("LINKEDIN_SESSION_EXPIRED")) {
		t.Errorf("expected LINKEDIN_SESSION_EXPIRED code, got %s", rec.Body.String())
	}
}

func TestHandler_AuthChallenge(t *testing.T) {
	fetcher := &mockFetcher{err: linkedin.ErrAuthChallenge}
	h := api.NewHandler(fetcher)

	rec := postRequest(h.Routes(), `{"url":"https://www.linkedin.com/in/test/"}`, "")
	if rec.Code != http.StatusFailedDependency {
		t.Errorf("expected 424, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("LINKEDIN_AUTH_CHALLENGE")) {
		t.Errorf("expected LINKEDIN_AUTH_CHALLENGE code, got %s", rec.Body.String())
	}
}

func TestHandler_RateLimited(t *testing.T) {
	fetcher := &mockFetcher{err: linkedin.ErrRateLimited}
	h := api.NewHandler(fetcher)

	rec := postRequest(h.Routes(), `{"url":"https://www.linkedin.com/in/test/"}`, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

func TestHandler_ProfileNotFound(t *testing.T) {
	fetcher := &mockFetcher{err: linkedin.ErrProfileNotFound}
	h := api.NewHandler(fetcher)

	rec := postRequest(h.Routes(), `{"url":"https://www.linkedin.com/in/ghost/"}`, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandler_Health(t *testing.T) {
	fetcher := &mockFetcher{}
	h := api.NewHandler(fetcher)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"status":"ok"`)) {
		t.Errorf("expected status: ok, got %s", rec.Body.String())
	}
}

type serializedFetcher struct {
	mu          sync.Mutex
	activeCount int32
	maxActive   int32
	records     []fetchRecord
	sem         chan struct{}
}

type fetchRecord struct {
	slug  string
	start time.Time
	end   time.Time
}

func newSerializedFetcher() *serializedFetcher {
	return &serializedFetcher{
		sem: make(chan struct{}, 1),
	}
}

func (s *serializedFetcher) FetchProfile(ctx context.Context, slug string) (*linkedin.InternalProfile, error) {
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.sem }()

	curr := atomic.AddInt32(&s.activeCount, 1)
	for {
		old := atomic.LoadInt32(&s.maxActive)
		if curr <= old || atomic.CompareAndSwapInt32(&s.maxActive, old, curr) {
			break
		}
	}

	start := time.Now()
	time.Sleep(50 * time.Millisecond) // Simulate full RSC sequence
	end := time.Now()

	atomic.AddInt32(&s.activeCount, -1)

	s.mu.Lock()
	s.records = append(s.records, fetchRecord{slug: slug, start: start, end: end})
	s.mu.Unlock()

	return &linkedin.InternalProfile{
		Slug:             slug,
		PublicIdentifier: slug,
		FullName:         "User " + slug,
	}, nil
}

func TestHandler_ConcurrentRequests_Serialized(t *testing.T) {
	fetcher := newSerializedFetcher()
	h := api.NewHandler(fetcher)
	handler := h.Routes()

	const numReqs = 3
	slugs := []string{"user1", "user2", "user3"}

	var wg sync.WaitGroup
	codes := make([]int, numReqs)

	overallStart := time.Now()

	for i := 0; i < numReqs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rec := postRequest(handler, `{"url":"https://www.linkedin.com/in/`+slugs[idx]+`/"}`, "")
			codes[idx] = rec.Code
		}(i)
	}

	wg.Wait()
	overallDuration := time.Since(overallStart)

	// Verify all returned 200 OK
	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("request %d expected status 200, got %d", i, code)
		}
	}

	// Verify max concurrent LinkedIn fetches was 1
	if max := atomic.LoadInt32(&fetcher.maxActive); max != 1 {
		t.Errorf("expected max active LinkedIn fetches to be 1, got %d", max)
	}

	// Verify sequential timestamps
	fetcher.mu.Lock()
	records := append([]fetchRecord{}, fetcher.records...)
	fetcher.mu.Unlock()

	if len(records) != numReqs {
		t.Fatalf("expected %d records, got %d", numReqs, len(records))
	}

	for i := 0; i < len(records)-1; i++ {
		first := records[i]
		second := records[i+1]
		if second.start.Before(first.end) {
			t.Errorf("overlap detected between fetch %d (ended %v) and fetch %d (started %v)",
				i, first.end, i+1, second.start)
		}
	}

	// Total duration must be at least 3 * 50ms = 150ms
	if overallDuration < 140*time.Millisecond {
		t.Errorf("expected serialized duration >= 140ms, got %v", overallDuration)
	}
}
