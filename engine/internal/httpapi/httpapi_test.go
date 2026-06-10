package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mist941/b3s23-engine/engine/internal/config"
	"github.com/mist941/b3s23-engine/engine/internal/engine"
	"github.com/mist941/b3s23-engine/engine/internal/gridcodec"
	"github.com/mist941/b3s23-engine/engine/internal/httpapi"
)

type fakeStore struct {
	mu   sync.Mutex
	meta *engine.Meta
	snap *engine.Snapshot
}

func (f *fakeStore) LoadMeta(context.Context) (*engine.Meta, bool, error) { return nil, false, nil }
func (f *fakeStore) LoadLatest(context.Context, int64) (*engine.Snapshot, error) {
	return nil, nil
}
func (f *fakeStore) SaveSnapshot(_ context.Context, m engine.Meta, s engine.Snapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.meta, f.snap = &m, &s
	return nil
}
func (f *fakeStore) Ping(context.Context) error { return nil }
func (f *fakeStore) Close() error               { return nil }

type nopBroadcaster struct{}

func (nopBroadcaster) Publish(engine.BroadcastItem) {}

func newServer(t *testing.T) (*httptest.Server, *engine.Simulation) {
	t.Helper()
	cfg := config.Default()
	cfg.Width, cfg.Height = 64, 64
	cfg.MaxDim = 1024
	cfg.MaxCells = 1024 * 1024
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

func do(t *testing.T, srv *httptest.Server, method, path string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, srv.URL+path, r)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeStatus(t *testing.T, resp *http.Response) httpapi.StatusResponse {
	t.Helper()
	defer resp.Body.Close()
	var s httpapi.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	return s
}

func TestStatusAndStep(t *testing.T) {
	srv, _ := newServer(t)

	s := decodeStatus(t, do(t, srv, http.MethodGet, "/api/v1/status", nil))
	if s.State != "paused" || s.Generation != 0 {
		t.Fatalf("initial status = %+v", s)
	}

	s = decodeStatus(t, do(t, srv, http.MethodPost, "/api/v1/step", nil))
	if s.Generation != 1 {
		t.Fatalf("after step generation = %d, want 1", s.Generation)
	}
}

func TestStepConflictWhileRunning(t *testing.T) {
	srv, _ := newServer(t)
	if resp := do(t, srv, http.MethodPost, "/api/v1/play", nil); resp.StatusCode != 200 {
		t.Fatalf("play status %d", resp.StatusCode)
	}
	resp := do(t, srv, http.MethodPost, "/api/v1/step", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("step while running status = %d, want 409", resp.StatusCode)
	}
	do(t, srv, http.MethodPost, "/api/v1/pause", nil).Body.Close()
}

func TestConfigSizeValidation(t *testing.T) {
	srv, _ := newServer(t)

	// Too large -> 400.
	resp := do(t, srv, http.MethodPut, "/api/v1/config/size", httpapi.SizeRequest{Width: 99999, Height: 99999})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("oversize status = %d, want 400", resp.StatusCode)
	}

	// Valid -> 200, dimensions updated.
	s := decodeStatus(t, do(t, srv, http.MethodPut, "/api/v1/config/size", httpapi.SizeRequest{Width: 128, Height: 96}))
	if s.Width != 128 || s.Height != 96 {
		t.Fatalf("size = %dx%d, want 128x96", s.Width, s.Height)
	}
}

func TestSizeConflictWhileRunning(t *testing.T) {
	srv, _ := newServer(t)
	do(t, srv, http.MethodPost, "/api/v1/play", nil).Body.Close()
	resp := do(t, srv, http.MethodPut, "/api/v1/config/size", httpapi.SizeRequest{Width: 128, Height: 128})
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("resize while running = %d, want 409", resp.StatusCode)
	}
	do(t, srv, http.MethodPost, "/api/v1/pause", nil).Body.Close()
}

func TestGridBinaryAndJSON(t *testing.T) {
	srv, _ := newServer(t)

	// Binary form decodes back to a 64x64 grid.
	resp := do(t, srv, http.MethodGet, "/api/v1/grid", nil)
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "octet-stream") {
		t.Fatalf("content-type = %q", ct)
	}
	blob, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	g, err := gridcodec.Decode(blob)
	if err != nil {
		t.Fatalf("decode grid: %v", err)
	}
	if g.W != 64 || g.H != 64 {
		t.Fatalf("grid = %dx%d", g.W, g.H)
	}

	// JSON form via Accept.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/grid", nil)
	req.Header.Set("Accept", "application/json")
	jresp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer jresp.Body.Close()
	var gj httpapi.GridJSONResponse
	if err := json.NewDecoder(jresp.Body).Decode(&gj); err != nil {
		t.Fatalf("decode grid json: %v", err)
	}
	if gj.Width != 64 || gj.BlobBase64 == "" {
		t.Fatalf("grid json = %+v", gj)
	}
}

func TestProbabilityClampAndReseed(t *testing.T) {
	srv, _ := newServer(t)
	// Out-of-range probability is clamped, not rejected.
	s := decodeStatus(t, do(t, srv, http.MethodPut, "/api/v1/config/probability",
		httpapi.ProbabilityRequest{Probability: 1.5, Reseed: true}))
	if s.Probability != 1.0 {
		t.Fatalf("probability = %v, want clamped to 1.0", s.Probability)
	}
}

func TestResetBumpsEpochOverHTTP(t *testing.T) {
	srv, _ := newServer(t)
	before := decodeStatus(t, do(t, srv, http.MethodGet, "/api/v1/status", nil)).Epoch
	after := decodeStatus(t, do(t, srv, http.MethodPost, "/api/v1/reset", nil)).Epoch
	if after != before+1 {
		t.Fatalf("epoch %d -> %d, want +1", before, after)
	}
}

func TestHealthz(t *testing.T) {
	srv, _ := newServer(t)
	resp := do(t, srv, http.MethodGet, "/api/v1/healthz", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("healthz status = %d", resp.StatusCode)
	}
	var h httpapi.HealthResponse
	json.NewDecoder(resp.Body).Decode(&h)
	if !h.OK || !h.DBOK {
		t.Fatalf("healthz = %+v", h)
	}
}
