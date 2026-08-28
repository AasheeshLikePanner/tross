package linkedin

import "errors"

var (
	ErrInvalidProfileURL       = errors.New("linkedin: invalid profile url")
	ErrProfileNotFound         = errors.New("linkedin: profile not found or private")
	ErrSessionExpired          = errors.New("linkedin: session expired or invalid")
	ErrAuthChallenge           = errors.New("linkedin: auth challenge or checkpoint triggered")
	ErrRateLimited             = errors.New("linkedin: rate limited by upstream")
	ErrLinkedInUnavailable     = errors.New("linkedin: upstream service unavailable")
	ErrInvalidLinkedInResponse = errors.New("linkedin: invalid or malformed response")
)
