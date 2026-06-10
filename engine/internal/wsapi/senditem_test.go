package wsapi

import (
	"testing"

	"github.com/mist941/b3s23-engine/engine/internal/engine"
)

func gridItem(gen uint64) sendItem {
	return sendItem{kind: engine.FrameGrid, gen: gen}
}

func controlItem() sendItem {
	return sendItem{kind: engine.FrameControl}
}

func TestPushGridCoalescesToNewest(t *testing.T) {
	q := newSendQueue(4)
	q.pushGrid(gridItem(1))
	q.pushGrid(gridItem(2))
	q.pushGrid(gridItem(3))

	it, ok := q.pop()
	if !ok || it.gen != 3 {
		t.Fatalf("coalesced grid = %d ok=%v, want gen 3", it.gen, ok)
	}
	if _, ok := q.pop(); ok {
		t.Fatal("expected exactly one coalesced grid item")
	}
}

func TestPushGridMonotonic(t *testing.T) {
	q := newSendQueue(4)
	q.pushGrid(gridItem(5))
	q.pushGrid(gridItem(3))
	it, _ := q.pop()
	if it.gen != 5 {
		t.Fatalf("gen = %d, want 5 (older frame overwrote newer)", it.gen)
	}
}

func TestControlNeverDroppedThenOverflowCloses(t *testing.T) {
	q := newSendQueue(2)
	for i := 0; i < 18; i++ {
		if !q.pushControl(controlItem()) {
			t.Fatalf("control %d rejected before hard cap", i)
		}
	}
	if q.pushControl(controlItem()) {
		t.Fatal("expected control beyond hard cap to be rejected")
	}
	select {
	case <-q.done:
	default:
		t.Fatal("queue should be closed after control overflow")
	}
}

func TestControlAndGridFIFOOrder(t *testing.T) {
	q := newSendQueue(8)
	q.pushControl(sendItem{kind: engine.FrameControl, gen: 10})
	q.pushGrid(gridItem(11))
	q.pushControl(sendItem{kind: engine.FrameControl, gen: 12})

	first, _ := q.pop()
	second, _ := q.pop()
	third, _ := q.pop()
	if first.kind != engine.FrameControl || first.gen != 10 {
		t.Fatalf("first = %+v", first)
	}
	if second.kind != engine.FrameGrid || second.gen != 11 {
		t.Fatalf("second = %+v", second)
	}
	if third.kind != engine.FrameControl || third.gen != 12 {
		t.Fatalf("third = %+v", third)
	}
}

func TestPushGridDropsWhenFullOfControl(t *testing.T) {
	q := newSendQueue(2)
	q.pushControl(controlItem())
	q.pushControl(controlItem())
	q.pushGrid(gridItem(1))

	for {
		it, ok := q.pop()
		if !ok {
			break
		}
		if it.kind == engine.FrameGrid {
			t.Fatal("grid should have been dropped when queue was full of control")
		}
	}
}
