package wsapi

import (
	"sync"

	"github.com/mist941/b3s23-engine/engine/internal/engine"
)

type sendItem struct {
	kind    engine.FrameKind
	gen     uint64
	header  []byte
	payload []byte
}

type sendQueue struct {
	mu      sync.Mutex
	items   []sendItem
	softCap int
	hardCap int
	closed  bool

	notify chan struct{}
	done   chan struct{}
}

func newSendQueue(softCap int) *sendQueue {
	if softCap < 1 {
		softCap = 1
	}
	return &sendQueue{
		softCap: softCap,
		hardCap: softCap + 16,
		notify:  make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
}

func (q *sendQueue) pushGrid(it sendItem) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	for i := range q.items {
		if q.items[i].kind == engine.FrameGrid {
			
			if it.gen >= q.items[i].gen {
				q.items[i] = it
			}
			q.mu.Unlock()
			q.signal()
			return
		}
	}
	if len(q.items) >= q.softCap {
		q.mu.Unlock()
		return
	}
	q.items = append(q.items, it)
	q.mu.Unlock()
	q.signal()
}

func (q *sendQueue) pushControl(it sendItem) bool {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return false
	}
	if len(q.items) >= q.hardCap {
		q.closeLocked()
		q.mu.Unlock()
		return false
	}
	q.items = append(q.items, it)
	q.mu.Unlock()
	q.signal()
	return true
}

func (q *sendQueue) pop() (sendItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return sendItem{}, false
	}
	it := q.items[0]
	q.items[0] = sendItem{}
	q.items = q.items[1:]
	return it, true
}

func (q *sendQueue) close() {
	q.mu.Lock()
	q.closeLocked()
	q.mu.Unlock()
}

func (q *sendQueue) closeLocked() {
	if !q.closed {
		q.closed = true
		close(q.done)
	}
}

func (q *sendQueue) signal() {
	select {
	case q.notify <- struct{}{}:
	default:
	}
}
