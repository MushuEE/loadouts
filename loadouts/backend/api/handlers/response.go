package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

// errorBody is the consistent error envelope for every endpoint.
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// writeJSON serializes a payload with the given status code.
func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// statusFor maps sentinel service errors onto HTTP status codes. It is separate from
// writeError so responses that are not JSON (the plugin sandbox serves HTML) can agree
// with the API about what an error means.
func statusFor(err error) int {
	status, _ := classify(err)
	return status
}

func classify(err error) (int, string) {
	switch {
	case errors.Is(err, core.ErrNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, core.ErrForbidden):
		return http.StatusForbidden, "forbidden"
	case errors.Is(err, core.ErrInvalid):
		return http.StatusBadRequest, "invalid"
	case errors.Is(err, core.ErrConflict):
		return http.StatusConflict, "conflict"
	}
	return http.StatusInternalServerError, "internal"
}

// writeError maps sentinel service errors onto HTTP status codes.
func writeError(w http.ResponseWriter, err error) {
	status, code := classify(err)

	var body errorBody
	body.Error.Code = code
	body.Error.Message = err.Error()
	writeJSON(w, status, body)
}

// decode reads a JSON body into dst, returning a friendly invalid-request error.
func decode(r *http.Request, dst interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return core.ErrInvalid
	}
	return nil
}

// queryInt reads an integer query parameter, falling back to a default.
func queryInt(r *http.Request, name string, fallback int) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
