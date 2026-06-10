package wsapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/mist941/b3s23-engine/engine/internal/engine"
)

const (
	pingInterval = 20 * time.Second
	writeTimeout = 10 * time.Second
)

type client struct {
	conn        *websocket.Conn
	hub         *Hub
	q           *sendQueue
	logger      *slog.Logger
	cleanupOnce sync.Once
}

func (c *client) serve(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.writePump(ctx)
		cancel()
		c.cleanup()
	}()

	c.readPump(ctx)
	cancel()
	c.cleanup()
	wg.Wait()
}

func (c *client) writePump(ctx context.Context) {
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.q.done:
			return
		case <-c.q.notify:
			for {
				it, ok := c.q.pop()
				if !ok {
					break
				}
				if err := c.writeItem(ctx, it); err != nil {
					c.logger.Debug("ws write failed", "err", err)
					return
				}
			}
		case <-ping.C:
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Ping(wctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (c *client) writeItem(ctx context.Context, it sendItem) error {
	if len(it.header) > 0 {
		if err := c.write(ctx, websocket.MessageText, it.header); err != nil {
			return err
		}
	}
	if it.kind == engine.FrameGrid && len(it.payload) > 0 {
		if err := c.write(ctx, websocket.MessageBinary, it.payload); err != nil {
			return err
		}
	}
	return nil
}

func (c *client) write(ctx context.Context, typ websocket.MessageType, data []byte) error {
	wctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return c.conn.Write(wctx, typ, data)
}

func (c *client) readPump(ctx context.Context) {
	for {
		typ, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}
		if typ == websocket.MessageText {
			c.handleControl(ctx, data)
		}
	}
}

type clientMsg struct {
	Type         string  `json:"type"`
	TickHz       float64 `json:"tickHz"`
	StreamEveryN int     `json:"streamEveryN"`
}

func (c *client) handleControl(ctx context.Context, data []byte) {
	if c.hub.ctrl == nil {
		return
	}
	var m clientMsg
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	var err error
	switch m.Type {
	case "play":
		err = c.hub.ctrl.Play(ctx)
	case "pause":
		err = c.hub.ctrl.Pause(ctx)
	case "step":
		err = c.hub.ctrl.Step(ctx)
	case "setRate":
		if m.TickHz > 0 {
			err = c.hub.ctrl.SetTickRate(ctx, m.TickHz)
		}
	case "setStreamRate":
		if m.StreamEveryN >= 1 {
			err = c.hub.ctrl.SetStreamEveryN(ctx, m.StreamEveryN)
		}
	}
	if err != nil {
		c.logger.Debug("ws control failed", "type", m.Type, "err", err)
	}
}

func (c *client) cleanup() {
	c.cleanupOnce.Do(func() {
		c.hub.unregister(c)
		c.q.close()
		c.conn.Close(websocket.StatusNormalClosure, "")
	})
}
