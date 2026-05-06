package http

import (
	"encoding/json"
	stdhttp "net/http"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w stdhttp.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w stdhttp.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}

func decodeJSON(r *stdhttp.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (server *Server) verifyCaptchaHeader(w stdhttp.ResponseWriter, r *stdhttp.Request) bool {
	id := r.Header.Get("X-Captcha-Id")
	answer := r.Header.Get("X-Captcha-Answer")
	if !server.captchaStore.Verify(r.Context(), id, answer) {
		writeError(w, stdhttp.StatusBadRequest, "invalid captcha")
		return false
	}
	return true
}
