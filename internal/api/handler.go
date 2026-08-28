package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tross/linkedin-profile-api/internal/linkedin"
	"github.com/tross/linkedin-profile-api/internal/profile"
)

const requestTimeout = 25 * time.Second

type LinkedInFetcher interface {
	FetchProfile(ctx context.Context, publicIdentifier string) (*linkedin.InternalProfile, error)
}

type Handler struct {
	li LinkedInFetcher
}

func NewHandler(li LinkedInFetcher) *Handler {
	return &Handler{li: li}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.handleHealth)
	mux.HandleFunc("POST /v1/profiles", h.handleProfile)

	var handler http.Handler = mux
	handler = bodySizeMiddleware(handler)
	handler = loggingMiddleware(handler)
	handler = requestIDMiddleware(handler)
	return handler
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleProfile(w http.ResponseWriter, r *http.Request) {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, newErrResponse("INVALID_REQUEST", "Content-Type must be application/json"))
		return
	}

	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, newErrResponse("INVALID_REQUEST", "Invalid JSON body"))
		return
	}

	if body.URL == "" {
		writeJSON(w, http.StatusBadRequest, newErrResponse("INVALID_PROFILE_URL", "Expected a LinkedIn /in/ profile URL."))
		return
	}

	slug, normalizedURL, err := linkedin.ParseProfileURL(body.URL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, newErrResponse("INVALID_PROFILE_URL", "Expected a LinkedIn /in/ profile URL."))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	slog.Info("fetching profile", "identifier", slug, "request_id", requestIDFromCtx(ctx))

	raw, err := h.li.FetchProfile(ctx, slug)
	if err != nil {
		status, errResp := httpStatusForError(err)
		slog.Warn("profile fetch failed", "identifier", slug, "err", err, "request_id", requestIDFromCtx(ctx))
		writeJSON(w, status, errResp)
		return
	}

	p, err := profile.Normalize(slug, normalizedURL, raw)
	if err != nil || p == nil {
		slog.Warn("normalization returned nil", "identifier", slug)
		writeJSON(w, http.StatusNotFound, newErrResponse("PROFILE_NOT_FOUND", "No LinkedIn profile found for this identifier."))
		return
	}

	writeJSON(w, http.StatusOK, p)
}
