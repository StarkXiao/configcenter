package transport

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"configcenter/internal/domain"
)

type contextKey string

const requestIDKey contextKey = "request-id"

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if id == "" || len(id) > 100 {
			data := make([]byte, 12)
			_, _ = rand.Read(data)
			id = hex.EncodeToString(data)
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("http request", slog.String("method", r.Method), slog.String("path", r.URL.Path),
			slog.Duration("duration", time.Since(started)), slog.String("request_id", requestID(r)))
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if value := recover(); value != nil {
				s.logger.Error("panic recovered", slog.Any("panic", value), slog.String("stack", string(debug.Stack())))
				writeError(w, r, domain.NewError(domain.CodeInternal, "internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) admin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" || len(token) != len(s.adminToken) || subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
			writeError(w, r, domain.NewError(domain.CodeUnauthorized, "invalid administrator token"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return domain.NewError(domain.CodeTooLarge, "request body is too large")
		}
		return domain.NewError(domain.CodeInvalid, "invalid JSON request: "+err.Error())
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return domain.NewError(domain.CodeInvalid, "request must contain one JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func respond(w http.ResponseWriter, r *http.Request, status int, value any, err error) {
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, status, value)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	code := domain.ErrorCodeOf(err)
	status := http.StatusInternalServerError
	switch code {
	case domain.CodeInvalid:
		status = http.StatusBadRequest
	case domain.CodeUnauthorized:
		status = http.StatusUnauthorized
	case domain.CodeForbidden:
		status = http.StatusForbidden
	case domain.CodeNotFound, domain.CodeNotPublished:
		status = http.StatusNotFound
	case domain.CodeConflict, domain.CodeRevisionConflict, domain.CodeVersionConflict:
		status = http.StatusConflict
	case domain.CodeTooLarge:
		status = http.StatusRequestEntityTooLarge
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{
		"code": string(code), "message": domain.ErrorMessage(err), "request_id": requestID(r),
	}})
}

func requestID(r *http.Request) string {
	value, _ := r.Context().Value(requestIDKey).(string)
	return value
}

func operator(r *http.Request) string { return strings.TrimSpace(r.Header.Get("X-Operator")) }
