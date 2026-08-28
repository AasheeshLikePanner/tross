package linkedin

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	DefaultBaseURL   = "https://www.linkedin.com"
	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"
	defaultTimeout   = 20 * time.Second
	defaultPacing    = 250 * time.Millisecond
)

type Client struct {
	session         *Session
	httpClient      *http.Client
	baseURL         string
	pacingLock      sync.Mutex
	lastRequestTime time.Time
	pacingInterval  time.Duration
	fetchSem        chan struct{}
}

func NewClient(liAt, jsessionID, bcookie, bscookie, lidc string) (*Client, error) {
	return NewClientWithBaseURL(liAt, jsessionID, bcookie, bscookie, lidc, DefaultBaseURL)
}

func NewClientWithBaseURL(liAt, jsessionID, bcookie, bscookie, lidc, baseURL string) (*Client, error) {
	session, err := NewSession(liAt, jsessionID, bcookie, bscookie, lidc)
	if err != nil {
		return nil, fmt.Errorf("linkedin: initializing session: %w", err)
	}
	return NewClientWithSession(session, baseURL), nil
}

func NewClientWithSession(session *Session, baseURL string) *Client {
	c := &Client{
		session:        session,
		baseURL:        baseURL,
		pacingInterval: defaultPacing,
		fetchSem:       make(chan struct{}, 1),
	}

	c.httpClient = &http.Client{
		Timeout: defaultTimeout,
		Jar:     session.Jar(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Do not automatically follow redirects — let session inspect the raw 302
			return http.ErrUseLastResponse
		},
	}

	return c
}

func (c *Client) AcquireFetchLock(ctx context.Context) error {
	select {
	case c.fetchSem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) ReleaseFetchLock() {
	select {
	case <-c.fetchSem:
	default:
	}
}

func (c *Client) SetPacingInterval(d time.Duration) {
	c.pacingLock.Lock()
	defer c.pacingLock.Unlock()
	c.pacingInterval = d
}

func (c *Client) SetTimeout(d time.Duration) {
	c.httpClient.Timeout = d
}

func (c *Client) rscBaseURL() string {
	return c.baseURL + "/flagship-web/rsc-action/actions/component"
}

func (c *Client) setBrowserHeaders(req *http.Request, method, referer string) {
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="128", "Not;A=Brand";v="24", "Google Chrome";v="128"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("x-li-lang", "en_US")

	if method == http.MethodGet {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Dest", "document")
	} else {
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Dest", "empty")
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}
