package api

import (
	"errors"
	"net/http"

	"github.com/tross/linkedin-profile-api/internal/linkedin"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

func newErrResponse(code, message string) errorResponse {
	return errorResponse{Error: apiError{Code: code, Message: message}}
}

func httpStatusForError(err error) (int, errorResponse) {
	switch {
	case errors.Is(err, linkedin.ErrSessionExpired):
		return http.StatusFailedDependency, newErrResponse("LINKEDIN_SESSION_EXPIRED", "The LinkedIn session has expired. Please update credentials.")
	case errors.Is(err, linkedin.ErrAuthChallenge):
		return http.StatusFailedDependency, newErrResponse("LINKEDIN_AUTH_CHALLENGE", "LinkedIn presented an authentication challenge or checkpoint.")
	case errors.Is(err, linkedin.ErrRateLimited):
		return http.StatusTooManyRequests, newErrResponse("LINKEDIN_RATE_LIMITED", "LinkedIn is rate limiting this service. Try again later.")
	case errors.Is(err, linkedin.ErrProfileNotFound):
		return http.StatusNotFound, newErrResponse("PROFILE_NOT_FOUND", "No LinkedIn profile found for this identifier.")
	case errors.Is(err, linkedin.ErrLinkedInUnavailable):
		return http.StatusBadGateway, newErrResponse("LINKEDIN_UPSTREAM_ERROR", "LinkedIn upstream is unavailable.")
	case errors.Is(err, linkedin.ErrInvalidLinkedInResponse):
		return http.StatusBadGateway, newErrResponse("LINKEDIN_UPSTREAM_ERROR", "Unexpected response from LinkedIn.")
	default:
		return http.StatusInternalServerError, newErrResponse("INTERNAL_ERROR", "An internal error occurred.")
	}
}
