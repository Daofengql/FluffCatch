package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
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

type routeParamsKey struct{}

func (server *Server) ginHandler(handler stdhttp.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		params := map[string]string{}
		for _, param := range c.Params {
			params[param.Key] = param.Value
		}
		request := c.Request.WithContext(context.WithValue(c.Request.Context(), routeParamsKey{}, params))
		handler(c.Writer, request)
	}
}

func (server *Server) requireAdminGin(c *gin.Context) {
	request := c.Request.WithContext(context.WithValue(c.Request.Context(), routeParamsKey{}, ginParams(c)))
	_, ok, err := server.currentAdmin(request)
	if err != nil {
		writeError(c.Writer, stdhttp.StatusInternalServerError, "failed to authenticate session")
		c.Abort()
		return
	}
	if !ok {
		writeError(c.Writer, stdhttp.StatusUnauthorized, "admin authentication required")
		c.Abort()
		return
	}
	c.Request = request
	c.Next()
}

func ginParams(c *gin.Context) map[string]string {
	params := map[string]string{}
	for _, param := range c.Params {
		params[param.Key] = param.Value
	}
	return params
}

func routeParam(r *stdhttp.Request, name string) string {
	params, _ := r.Context().Value(routeParamsKey{}).(map[string]string)
	return params[name]
}
