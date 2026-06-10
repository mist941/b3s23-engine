package gridcodec

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"

	"github.com/mist941/b3s23-engine/engine/internal/life"
)

const (
	CodecRaw   uint8 = 1 // packed LE-uint64 words, uncompressed
	CodecFlate uint8 = 2 // CodecRaw bytes wrapped in compress/flate
)

const (
	formatVersion = 1
	headerSize    = 24
)

var (
	magic       = [4]byte{'B', '3', 'S', '2'}
	castagnoli  = crc32.MakeTable(crc32.Castagnoli)
	errTooShort = errors.New("gridcodec: blob shorter than header")
)

func Checksum(g *life.Grid) uint32 {
	return crc32.Checksum(rawBytes(g), castagnoli)
}

func Encode(g *life.Grid, codec uint8) ([]byte, error) {
	raw := rawBytes(g)
	crc := crc32.Checksum(raw, castagnoli)

	var payload []byte
	switch codec {
	case CodecRaw:
		payload = raw
	case CodecFlate:
		var err error
		if payload, err = flateCompress(raw); err != nil {
			return nil, fmt.Errorf("gridcodec: compress: %w", err)
		}
	default:
		return nil, fmt.Errorf("gridcodec: unknown codec %d", codec)
	}

	out := make([]byte, headerSize+len(payload))
	copy(out[0:4], magic[:])
	out[4] = formatVersion
	out[5] = codec
	// out[6:8] reserved, already zero.
	binary.LittleEndian.PutUint32(out[8:12], uint32(g.W))
	binary.LittleEndian.PutUint32(out[12:16], uint32(g.H))
	binary.LittleEndian.PutUint32(out[16:20], uint32(g.WordsPerRow()))
	binary.LittleEndian.PutUint32(out[20:24], crc)
	copy(out[headerSize:], payload)
	return out, nil
}

func Decode(data []byte) (*life.Grid, error) {
	if len(data) < headerSize {
		return nil, errTooShort
	}
	if !bytes.Equal(data[0:4], magic[:]) {
		return nil, errors.New("gridcodec: bad magic")
	}
	if v := data[4]; v != formatVersion {
		return nil, fmt.Errorf("gridcodec: unsupported version %d", v)
	}
	codec := data[5]
	w := int(binary.LittleEndian.Uint32(data[8:12]))
	h := int(binary.LittleEndian.Uint32(data[12:16]))
	wpr := int(binary.LittleEndian.Uint32(data[16:20]))
	wantCRC := binary.LittleEndian.Uint32(data[20:24])

	if w <= 0 || h <= 0 {
		return nil, life.ErrBadDimensions
	}
	if wpr != (w+63)/64 {
		return nil, fmt.Errorf("gridcodec: wordsPerRow %d inconsistent with width %d", wpr, w)
	}
	expectRaw := wpr * h * 8

	payload := data[headerSize:]
	var raw []byte
	switch codec {
	case CodecRaw:
		raw = payload
	case CodecFlate:
		var err error
		if raw, err = flateDecompress(payload, expectRaw); err != nil {
			return nil, fmt.Errorf("gridcodec: decompress: %w", err)
		}
	default:
		return nil, fmt.Errorf("gridcodec: unknown codec %d", codec)
	}

	if len(raw) != expectRaw {
		return nil, fmt.Errorf("gridcodec: raw length %d, want %d", len(raw), expectRaw)
	}
	if got := crc32.Checksum(raw, castagnoli); got != wantCRC {
		return nil, fmt.Errorf("gridcodec: checksum mismatch (got %08x want %08x)", got, wantCRC)
	}

	words := make([]uint64, wpr*h)
	for i := range words {
		words[i] = binary.LittleEndian.Uint64(raw[i*8 : i*8+8])
	}
	return life.NewGridFromWords(w, h, words)
}

func rawBytes(g *life.Grid) []byte {
	words := g.Words()
	raw := make([]byte, len(words)*8)
	for i, w := range words {
		binary.LittleEndian.PutUint64(raw[i*8:i*8+8], w)
	}
	return raw
}

func flateCompress(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func flateDecompress(payload []byte, expectRaw int) ([]byte, error) {
	r := flate.NewReader(bytes.NewReader(payload))
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, int64(expectRaw)+1))
}
