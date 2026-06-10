package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/mist941/b3s23-engine/engine/internal/config"
)

func withMiddleware(next http.Handler, cfg config.Config, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.CORSAllowOrigin != "" {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", cfg.CORSAllowOrigin)
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic in handler", "err", rec, "method", r.Method, "path", r.URL.Path)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		logger.Debug("http request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
