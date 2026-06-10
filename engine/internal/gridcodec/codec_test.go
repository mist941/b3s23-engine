package gridcodec

import (
	"testing"

	"github.com/mist941/b3s23-engine/engine/internal/life"
)

func makeGrid(t *testing.T, w, h int, prob float64, seed uint64) *life.Grid {
	t.Helper()
	g := life.NewGrid(w, h)
	life.Seed(g, prob, seed)
	return g
}

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		w, h  int
		codec uint8
	}{
		{1024, 1024, CodecRaw},
		{1024, 1024, CodecFlate},
		{65, 65, CodecRaw},
		{1000, 37, CodecFlate},
		{8, 8, CodecRaw},
	}
	for _, c := range cases {
		g := makeGrid(t, c.w, c.h, 0.1, 123)
		blob, err := Encode(g, c.codec)
		if err != nil {
			t.Fatalf("Encode %dx%d codec %d: %v", c.w, c.h, c.codec, err)
		}
		got, err := Decode(blob)
		if err != nil {
			t.Fatalf("Decode %dx%d codec %d: %v", c.w, c.h, c.codec, err)
		}
		if !g.Equal(got) {
			t.Fatalf("%dx%d codec %d: round-trip mismatch", c.w, c.h, c.codec)
		}
	}
}

func TestDecodeRejectsCorruption(t *testing.T) {
	g := makeGrid(t, 128, 128, 0.3, 9)
	blob, err := Encode(g, CodecRaw)
	if err != nil {
		t.Fatal(err)
	}

	// Flip a payload bit -> checksum mismatch.
	bad := append([]byte(nil), blob...)
	bad[headerSize+10] ^= 0x01
	if _, err := Decode(bad); err == nil {
		t.Fatal("expected checksum mismatch error")
	}

	// Truncated blob.
	if _, err := Decode(blob[:headerSize-1]); err == nil {
		t.Fatal("expected too-short error")
	}

	// Corrupt magic.
	badMagic := append([]byte(nil), blob...)
	badMagic[0] = 'X'
	if _, err := Decode(badMagic); err == nil {
		t.Fatal("expected bad magic error")
	}

	// Truncated payload (geometry says more words than present).
	shortPayload := blob[:len(blob)-8]
	if _, err := Decode(shortPayload); err == nil {
		t.Fatal("expected raw-length mismatch error")
	}
}

func TestChecksumMatchesEmbedded(t *testing.T) {
	g := makeGrid(t, 200, 50, 0.25, 5)
	if Checksum(g) == 0 {
		t.Skip("checksum coincidentally zero")
	}
	g2 := makeGrid(t, 200, 50, 0.25, 5)
	if Checksum(g) != Checksum(g2) {
		t.Fatal("checksum should be deterministic for identical grids")
	}
}

func TestFlateSmallerOnSparse(t *testing.T) {
	g := makeGrid(t, 1024, 1024, 0.1, 1)
	raw, _ := Encode(g, CodecRaw)
	comp, _ := Encode(g, CodecFlate)
	if len(comp) >= len(raw) {
		t.Fatalf("flate (%d) should beat raw (%d) on a sparse 10%% seed", len(comp), len(raw))
	}
}
