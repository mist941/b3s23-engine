package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mist941/b3s23-engine/engine/internal/config"
	"github.com/mist941/b3s23-engine/engine/internal/engine"
	"github.com/mist941/b3s23-engine/engine/internal/httpapi"
	"github.com/mist941/b3s23-engine/engine/internal/store"
	"github.com/mist941/b3s23-engine/engine/internal/wsapi"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cfg, logLevel, healthcheck, showVersion := parseFlags()

	if showVersion {
		fmt.Println("engined", version)
		return
	}

	// Health-probe mode: used by the container HEALTHCHECK. The distroless image
	// has no shell or curl, so the binary probes itself and exits 0/1.
	if healthcheck {
		os.Exit(probe(cfg.Addr))
	}

	logger := newLogger(logLevel)
	slog.SetDefault(logger)

	if err := run(cfg, logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// probe issues a GET /api/v1/healthz against the local listener and returns a
// process exit code: 0 when the engine reports healthy, 1 otherwise.
func probe(addr string) int {
	// addr is a listen address like ":8050"; dial it on localhost.
	host := addr
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + host + "/api/v1/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func run(cfg config.Config, logger *slog.Logger) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	// Fail fast on a misconfigured UI directory instead of serving 404s.
	if cfg.StaticDir != "" {
		if _, err := os.Stat(filepath.Join(cfg.StaticDir, "index.html")); err != nil {
			return fmt.Errorf("static dir %q: %w", cfg.StaticDir, err)
		}
	}

	st, err := store.Open(cfg.DBPath, cfg.SnapshotRetain)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := st.Close(); cerr != nil {
			logger.Error("close store", "err", cerr)
		}
	}()

	// Break the hub<->engine construction cycle: the hub is the engine's
	// Broadcaster, and the engine is the hub's Controller. Create the hub first
	// (no controller), then the engine, then wire the controller back.
	hub := wsapi.NewHub(nil, wsapi.Options{
		MaxSubscribers:  cfg.MaxSubscribers,
		SendQueueSize:   cfg.SendQueueSize,
		AllowAllOrigins: cfg.CORSAllowOrigin == "*",
		Logger:          logger,
	})

	sim, err := engine.New(cfg, st, hub, logger)
	if err != nil {
		return err
	}
	hub.SetController(sim)

	sim.Start()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewRouter(sim, hub, cfg, logger),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("engine listening", "addr", cfg.Addr, "db", cfg.DBPath, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serveErr:
		if err != nil {
			sim.Stop(context.Background())
			return err
		}
	}

	// Graceful shutdown: stop accepting HTTP/WS first, then finalize the
	// simulation (pause + final snapshot through the persistence goroutine),
	// then the deferred store Close checkpoints and clears the WAL sidecars.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown", "err", err)
	}
	sim.Stop(shutdownCtx)
	logger.Info("shutdown complete")
	return nil
}

func parseFlags() (config.Config, slog.Level, bool, bool) {
	cfg := config.Default()
	var (
		seed        int64
		logLevel    string
		healthcheck bool
		showVersion bool
	)
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&healthcheck, "healthcheck", false, "probe /api/v1/healthz and exit 0 (healthy) or 1")
	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "HTTP listen address")
	flag.StringVar(&cfg.DBPath, "db", cfg.DBPath, "SQLite database path")
	flag.IntVar(&cfg.Width, "width", cfg.Width, "initial grid width")
	flag.IntVar(&cfg.Height, "height", cfg.Height, "initial grid height")
	flag.Float64Var(&cfg.Probability, "prob", cfg.Probability, "seed life probability [0,1]")
	flag.Int64Var(&seed, "seed", int64(cfg.RngSeed), "RNG seed for the initial cold board")
	flag.Float64Var(&cfg.TickHz, "tick", cfg.TickHz, "simulation generations per second")
	flag.IntVar(&cfg.StreamEveryN, "stream", cfg.StreamEveryN, "broadcast every Nth generation")
	flag.Float64Var(&cfg.MaxWSFps, "maxfps", cfg.MaxWSFps, "cap WebSocket frame rate (0 = no cap)")
	flag.IntVar(&cfg.SnapshotEveryN, "snapshot", cfg.SnapshotEveryN, "persist a snapshot every N generations")
	flag.IntVar(&cfg.SnapshotRetain, "retain", cfg.SnapshotRetain, "snapshots kept per epoch")
	flag.IntVar(&cfg.MaxCells, "maxcells", cfg.MaxCells, "hard cap on width*height")
	flag.IntVar(&cfg.MaxSubscribers, "maxsubs", cfg.MaxSubscribers, "max concurrent WebSocket clients")
	flag.BoolVar(&cfg.FlateBlobs, "flate", cfg.FlateBlobs, "compress snapshot blobs with flate")
	flag.StringVar(&cfg.CORSAllowOrigin, "cors", cfg.CORSAllowOrigin, "Access-Control-Allow-Origin value")
	flag.StringVar(&cfg.StaticDir, "static", cfg.StaticDir, "directory with the built UI to serve at / (empty = API only)")
	flag.StringVar(&logLevel, "log", "info", "log level: debug|info|warn|error")
	flag.Parse()

	cfg.RngSeed = uint64(seed)
	return cfg, parseLevel(logLevel), healthcheck, showVersion
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
