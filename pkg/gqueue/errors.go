package gqueue

import (
	"errors"
	"fmt"
)

const maxErrBodySnippet = 2048

// APIError is returned when the gqueue API responds with a non-success status.
type APIError struct {
	StatusCode int
	Body       []byte
}

func (e *APIError) Error() string {
	if e == nil {
		return "gqueue: <nil APIError>"
	}
	snippet := string(e.Body)
	if len(snippet) > maxErrBodySnippet {
		snippet = snippet[:maxErrBodySnippet] + "…"
	}
	return fmt.Sprintf("gqueue: API status %d: %s", e.StatusCode, snippet)
}

// IsAPIError reports whether err is or wraps *APIError.
func IsAPIError(err error) bool {
	var ae *APIError
	return err != nil && errors.As(err, &ae)
}
