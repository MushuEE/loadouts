package core

import "errors"

// Sentinel errors the service layer returns so HTTP handlers can map them to status codes
// without string matching.
var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrInvalid   = errors.New("invalid request")
	ErrConflict  = errors.New("conflict")
)
