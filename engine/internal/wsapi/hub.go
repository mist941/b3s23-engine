package wsapi

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/coder/websocket"

	"github.com/mist941/b3s23-engine/engine/internal/engine"
)

type Controller interface {
	Play(context.Context) error
	Pause(context.Context) error
	Step(context.Context) error
	SetTickRate(context.Context, float64) error
	SetStreamEveryN(context.Context, int) error
}

type Options struct {
	MaxSubscribers  int
	SendQueueSize   int
	AllowAllOrigins bool
	OriginPatterns  []string
	Logger          *slog.Logger
}

type Hub struct {
	ctrl   Controller
	opts   Options
	logger *slog.Logger

	mu         sync.Mutex
	clients    map[*client]struct{}
	lastGrid   *sendItem
	lastStatus *sendItem
}

func NewHub(ctrl Controller, opts Options) *Hub {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.MaxSubscribers < 1 {
		opts.MaxSubscribers = 1
	}
	if opts.SendQueueSize < 1 {
		opts.SendQueueSize = 4
	}
	return &Hub{
		ctrl:    ctrl,
		opts:    opts,
		logger:  opts.Logger,
		clients: make(map[*client]struct{}),
	}
}

func (h *Hub) Publish(item engine.BroadcastItem) {
	it := sendItem{kind: item.Kind, gen: item.Generation, header: item.Header, payload: item.Payload}

	h.mu.Lock()
	if it.kind == engine.FrameGrid {
		cp := it
		h.lastGrid = &cp
	} else {
		cp := it
		h.lastStatus = &cp
	}
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, c := range clients {
		if it.kind == engine.FrameGrid {
			c.q.pushGrid(it)
		} else if !c.q.pushControl(it) {
			c.cleanup()
		}
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	n := len(h.clients)
	h.mu.Unlock()
	if n >= h.opts.MaxSubscribers {
		http.Error(w, "too many subscribers", http.StatusServiceUnavailable)
		return
	}

	acc := &websocket.AcceptOptions{CompressionMode: websocket.CompressionNoContextTakeover}
	if h.opts.AllowAllOrigins {
		acc.InsecureSkipVerify = true
	} else if len(h.opts.OriginPatterns) > 0 {
		acc.OriginPatterns = h.opts.OriginPatterns
	}

	conn, err := websocket.Accept(w, r, acc)
	if err != nil {
		h.logger.Debug("ws accept failed", "err", err)
		return
	}

	c := &client{conn: conn, hub: h, q: newSendQueue(h.opts.SendQueueSize), logger: h.logger}
	if !h.register(c) {
		conn.Close(websocket.StatusTryAgainLater, "too many subscribers")
		return
	}
	h.replay(c)
	c.serve(context.Background())
}

func (h *Hub) register(c *client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.clients) >= h.opts.MaxSubscribers {
		return false
	}
	h.clients[c] = struct{}{}
	return true
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

func (h *Hub) replay(c *client) {
	h.mu.Lock()
	status, grid := h.lastStatus, h.lastGrid
	h.mu.Unlock()

	if status != nil {
		c.q.pushControl(*status)
	}
	if grid != nil {
		c.q.pushGrid(*grid)
	}
}

func (h *Hub) SetController(c Controller) { h.ctrl = c }

func (h *Hub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
