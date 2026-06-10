package httpapi_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mist941/b3s23-engine/engine/internal/config"
	"github.com/mist941/b3s23-engine/engine/internal/engine"
	"github.com/mist941/b3s23-engine/engine/internal/httpapi"
)

func newServerWithStatic(t *testing.T, staticDir string) (*httptest.Server, *engine.Simulation) {
	t.Helper()
	cfg := config.Default()
	cfg.Width, cfg.Height = 64, 64
	cfg.StaticDir = staticDir
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	sim, err := engine.New(cfg, &fakeStore{}, nopBroadcaster{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	sim.Start()
	ws := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUpgradeRequired) })
	srv := httptest.NewServer(httpapi.NewRouter(sim, ws, cfg, nil))
	t.Cleanup(func() {
		srv.Close()
		sim.Stop(context.Background())
	})
	return srv, sim
}

func TestStaticServing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>ui</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, _ := newServerWithStatic(t, dir)

	for _, tc := range []struct {
		path     string
		wantCode int
		wantBody string
	}{
		{"/", http.StatusOK, "<html>ui</html>"},
		{"/app.js", http.StatusOK, "console.log(1)"},
		{"/some/spa/route", http.StatusOK, "<html>ui</html>"}, // fallback
		{"/api/v1/nope", http.StatusNotFound, ""},             // no fallback for API
	} {
		resp := do(t, srv, http.MethodGet, tc.path, nil)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tc.wantCode {
			t.Errorf("GET %s: status %d, want %d", tc.path, resp.StatusCode, tc.wantCode)
		}
		if tc.wantBody != "" && string(body) != tc.wantBody {
			t.Errorf("GET %s: body %q, want %q", tc.path, body, tc.wantBody)
		}
	}
}

func TestNoStaticDir(t *testing.T) {
	srv, _ := newServer(t)
	resp := do(t, srv, http.MethodGet, "/", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /: status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
