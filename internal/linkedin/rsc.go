package linkedin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const (
	rscActionBaseURL = "https://www.linkedin.com/flagship-web/rsc-action/actions/component"
	maxResponseBody  = 5 * 1024 * 1024

	CompAbove = "com.linkedin.sdui.generated.profile.dsl.impl.profileCardsAboveActivity"
	CompExp   = "com.linkedin.sdui.generated.profile.dsl.impl.profileCardsExperienceOnly"
	CompPart1 = "com.linkedin.sdui.generated.profile.dsl.impl.profileCardsBelowActivityPart1WithoutExp"
	CompPart4 = "com.linkedin.sdui.generated.profile.dsl.impl.profileCardsBelowActivityPart4"
	CompPart7 = "com.linkedin.sdui.generated.profile.dsl.impl.profileCardsBelowActivityPart7"
)

func (c *Client) FetchProfileDocument(ctx context.Context, slug string) (string, *RawProfileTopCard, error) {
	initURL := fmt.Sprintf("%s/in/%s/", c.baseURL, slug)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, initURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("linkedin: creating document request: %w", err)
	}

	c.setBrowserHeaders(req, http.MethodGet, "")

	resp, body, err := c.doRequest(ctx, req)
	if err != nil {
		return "", nil, err
	}

	if err := c.session.InspectResponse(resp); err != nil {
		return "", nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// success
	case http.StatusNotFound:
		return "", nil, ErrProfileNotFound
	case http.StatusTooManyRequests:
		return "", nil, ErrRateLimited
	case http.StatusUnauthorized, http.StatusForbidden:
		return "", nil, ErrSessionExpired
	default:
		if resp.StatusCode >= 500 {
			return "", nil, ErrLinkedInUnavailable
		}
		return "", nil, fmt.Errorf("%w: status %d", ErrInvalidLinkedInResponse, resp.StatusCode)
	}

	htmlContent := string(body)
	topCard, err := DecodeTopCardFromHTML(htmlContent)
	if err != nil {
		return "", nil, err
	}

	return htmlContent, topCard, nil
}

func (c *Client) FetchComponent(ctx context.Context, componentID, slug, vieweeProfileID, referer string) ([]byte, error) {
	reqURL := fmt.Sprintf("%s?componentId=%s&sduiid=%s",
		c.rscBaseURL(),
		url.QueryEscape(componentID),
		url.QueryEscape(componentID),
	)

	payload := map[string]interface{}{
		"clientArguments": map[string]interface{}{
			"payload": map[string]interface{}{
				"isSelfView": false,
				"vanityName": slug,
				"replaceableSectionArgs": map[string]interface{}{
					"vanityName":                      slug,
					"hideCardsForGoldenGate":          false,
					"shouldSetupReplaceableComponent": true,
					"vieweeProfileId":                 vieweeProfileID,
					"isSelfView":                      false,
					"isSelfViewResolved":              false,
				},
			},
			"states":           []interface{}{},
			"screenId":         "profile.profile-v2",
			"knownTemplateIds": []interface{}{},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("linkedin: encoding component payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("linkedin: creating component request: %w", err)
	}

	c.setBrowserHeaders(req, http.MethodPost, referer)
	req.Header.Set("Origin", "https://www.linkedin.com")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/octet-stream")
	req.Header.Set("x-li-rsc-stream", "1")
	req.Header.Set("csrf-token", c.session.CSRFToken())

	resp, body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := c.session.InspectResponse(resp); err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return body, nil
	case http.StatusNotFound:
		return nil, ErrProfileNotFound
	case http.StatusTooManyRequests:
		return nil, ErrRateLimited
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrSessionExpired
	default:
		if resp.StatusCode >= 500 {
			return nil, ErrLinkedInUnavailable
		}
		return nil, fmt.Errorf("%w: status %d", ErrInvalidLinkedInResponse, resp.StatusCode)
	}
}

func (c *Client) doRequest(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	c.pacingLock.Lock()
	elapsed := time.Since(c.lastRequestTime)
	if elapsed < c.pacingInterval {
		time.Sleep(c.pacingInterval - elapsed)
	}
	c.lastRequestTime = time.Now()
	c.pacingLock.Unlock()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
			return nil, nil, fmt.Errorf("linkedin: network error: %w", err)
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, nil, fmt.Errorf("linkedin: reading body: %w", err)
	}

	slog.Debug("linkedin upstream call",
		"method", req.Method,
		"url", req.URL.Path,
		"status", resp.StatusCode,
		"body_bytes", len(body),
	)

	return resp, body, nil
}
