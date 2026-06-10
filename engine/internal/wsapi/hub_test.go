package wsapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/mist941/b3s23-engine/engine/internal/engine"
	"github.com/mist941/b3s23-engine/engine/internal/gridcodec"
	"github.com/mist941/b3s23-engine/engine/internal/life"
	"github.com/mist941/b3s23-engine/engine/internal/wsapi"
)

type fakeController struct {
	mu    sync.Mutex
	plays int
}

func (f *fakeController) Play(context.Context) error {
	f.mu.Lock()
	f.plays++
	f.mu.Unlock()
	return nil
}
func (f *fakeController) Pause(context.Context) error                { return nil }
func (f *fakeController) Step(context.Context) error                 { return nil }
func (f *fakeController) SetTickRate(context.Context, float64) error { return nil }
func (f *fakeController) SetStreamEveryN(context.Context, int) error { return nil }

func (f *fakeController) playCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.plays
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func gridFrame(t *testing.T, gen uint64) engine.BroadcastItem {
	g := life.NewGrid(16, 16)
	life.Seed(g, 0.3, gen)
	blob, err := gridcodec.Encode(g, gridcodec.CodecRaw)
	if err != nil {
		t.Fatal(err)
	}
	return engine.BroadcastItem{
		Kind:       engine.FrameGrid,
		Generation: gen,
		Header:     mustJSON(t, map[string]any{"type": "gen", "generation": gen}),
		Payload:    blob,
	}
}

func wsURL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func dial(t *testing.T, srv *httptest.Server) (*websocket.Conn, context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	conn, _, err := websocket.Dial(ctx, wsURL(srv), nil)
	if err != nil {
		cancel()
		t.Fatalf("dial: %v", err)
	}
	return conn, ctx, cancel
}

func readGridFrame(t *testing.T, ctx context.Context, conn *websocket.Conn) uint64 {
	t.Helper()
	typ, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read gen header: %v", err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("expected text gen header, got %v", typ)
	}
	var hdr struct {
		Type       string `json:"type"`
		Generation uint64 `json:"generation"`
	}
	if err := json.Unmarshal(data, &hdr); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if hdr.Type != "gen" {
		t.Fatalf("header type = %q, want gen", hdr.Type)
	}
	btyp, bin, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if btyp != websocket.MessageBinary {
		t.Fatalf("expected binary frame after gen header, got %v", btyp)
	}
	if len(bin) == 0 {
		t.Fatal("empty binary frame")
	}
	return hdr.Generation
}

func newHubServer(ctrl wsapi.Controller) (*wsapi.Hub, *httptest.Server) {
	hub := wsapi.NewHub(ctrl, wsapi.Options{MaxSubscribers: 4, SendQueueSize: 8, AllowAllOrigins: true})
	srv := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	return hub, srv
}

func TestHubHelloAndFrameAtomicity(t *testing.T) {
	hub, srv := newHubServer(&fakeController{})
	defer srv.Close()

	hub.Publish(engine.BroadcastItem{Kind: engine.FrameControl, Header: mustJSON(t, map[string]any{"type": "status", "generation": 0})})
	hub.Publish(gridFrame(t, 0))

	conn, ctx, cancel := dial(t, srv)
	defer cancel()
	defer conn.Close(websocket.StatusNormalClosure, "")

	typ, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("hello type = %v", typ)
	}
	var hello struct {
		Type string `json:"type"`
	}
	json.Unmarshal(data, &hello)
	if hello.Type != "status" {
		t.Fatalf("hello type = %q, want status", hello.Type)
	}

	if g := readGridFrame(t, ctx, conn); g != 0 {
		t.Fatalf("initial grid gen = %d, want 0", g)
	}

	for gen := uint64(1); gen <= 5; gen++ {
		hub.Publish(gridFrame(t, gen))
		if got := readGridFrame(t, ctx, conn); got != gen {
			t.Fatalf("frame gen = %d, want %d", got, gen)
		}
	}
}

func TestHubRoutesControlToController(t *testing.T) {
	ctrl := &fakeController{}
	_, srv := newHubServer(ctrl)
	defer srv.Close()

	conn, ctx, cancel := dial(t, srv)
	defer cancel()
	defer conn.Close(websocket.StatusNormalClosure, "")

	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"play"}`)); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for ctrl.playCount() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("play control was not routed to the controller")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHubReapsDisconnectedClient(t *testing.T) {
	hub, srv := newHubServer(&fakeController{})
	defer srv.Close()

	conn, _, cancel := dial(t, srv)
	defer cancel()

	deadline := time.Now().Add(2 * time.Second)
	for hub.ClientCount() != 1 {
		if time.Now().After(deadline) {
			t.Fatal("client never registered")
		}
		time.Sleep(5 * time.Millisecond)
	}

	conn.Close(websocket.StatusNormalClosure, "")

	for hub.ClientCount() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("client not reaped, count = %d", hub.ClientCount())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHubEnforcesSubscriberCap(t *testing.T) {
	hub := wsapi.NewHub(&fakeController{}, wsapi.Options{MaxSubscribers: 1, SendQueueSize: 4, AllowAllOrigins: true})
	srv := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer srv.Close()

	c1, _, cancel1 := dial(t, srv)
	defer cancel1()
	defer c1.Close(websocket.StatusNormalClosure, "")

	deadline := time.Now().Add(2 * time.Second)
	for hub.ClientCount() != 1 {
		if time.Now().After(deadline) {
			t.Fatal("first client never registered")
		}
		time.Sleep(5 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, wsURL(srv), nil)
	if err == nil {
		t.Fatal("expected second dial to be rejected")
	}
	if resp != nil && resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("rejection status = %d, want 503", resp.StatusCode)
	}
}
