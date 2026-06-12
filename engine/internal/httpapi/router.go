package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/mist941/b3s23-engine/engine/internal/config"
	"github.com/mist941/b3s23-engine/engine/internal/engine"
)

func NewRouter(sim *engine.Simulation, wsHandler http.Handler, cfg config.Config, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	api := &API{sim: sim, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", api.handleStatus)
	mux.HandleFunc("GET /api/v1/grid", api.handleGrid)
	mux.HandleFunc("POST /api/v1/play", api.handlePlay)
	mux.HandleFunc("POST /api/v1/pause", api.handlePause)
	mux.HandleFunc("POST /api/v1/step", api.handleStep)
	mux.HandleFunc("POST /api/v1/reset", api.handleReset)
	mux.HandleFunc("POST /api/v1/reseed", api.handleReseed)
	mux.HandleFunc("PUT /api/v1/config/probability", api.handleProbability)
	mux.HandleFunc("PUT /api/v1/config/size", api.handleSize)
	mux.HandleFunc("PUT /api/v1/config/tickrate", api.handleTickRate)
	mux.HandleFunc("PUT /api/v1/config/streamrate", api.handleStreamRate)
	mux.HandleFunc("POST /api/v1/cells", api.handleSetCells)
	mux.HandleFunc("POST /api/v1/snapshot", api.handleSnapshot)
	mux.HandleFunc("GET /api/v1/healthz", api.handleHealth)
	mux.Handle("GET /api/v1/ws", wsHandler)

	if cfg.StaticDir != "" {
		mux.Handle("/", spaHandler(cfg.StaticDir))
	}

	return withMiddleware(mux, cfg, logger)
}
