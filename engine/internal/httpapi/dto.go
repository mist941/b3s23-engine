package httpapi

import (
	"encoding/base64"
	"time"

	"github.com/mist941/b3s23-engine/engine/internal/engine"
	"github.com/mist941/b3s23-engine/engine/internal/gridcodec"
)

type StatusResponse struct {
	State        string  `json:"state"`
	Generation   uint64  `json:"generation"`
	Epoch        int64   `json:"epoch"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	Probability  float64 `json:"probability"`
	TickHz       float64 `json:"tickHz"`
	StreamEveryN int     `json:"streamEveryN"`
	Population   int     `json:"population"`
	StartTime    string  `json:"startTime"`
}

func statusFromView(v *engine.StateView) StatusResponse {
	if v == nil {
		return StatusResponse{State: engine.StateStopped.String()}
	}
	return StatusResponse{
		State:        v.State.String(),
		Generation:   v.Generation,
		Epoch:        v.Epoch,
		Width:        v.Width,
		Height:       v.Height,
		Probability:  v.Probability,
		TickHz:       v.TickHz,
		StreamEveryN: v.StreamEveryN,
		Population:   v.Population,
		StartTime:    v.StartTime.UTC().Format(time.RFC3339),
	}
}

type GridJSONResponse struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	WordsPerRow int    `json:"wordsPerRow"`
	Generation  uint64 `json:"generation"`
	Epoch       int64  `json:"epoch"`
	Codec       int    `json:"codec"`
	Checksum    uint32 `json:"checksum"`
	BlobBase64  string `json:"blobBase64"`
}

func gridJSONFromView(v *engine.StateView, blob []byte) GridJSONResponse {
	return GridJSONResponse{
		Width:       v.Width,
		Height:      v.Height,
		WordsPerRow: v.Grid.WordsPerRow(),
		Generation:  v.Generation,
		Epoch:       v.Epoch,
		Codec:       int(gridcodec.CodecRaw),
		Checksum:    gridcodec.Checksum(v.Grid),
		BlobBase64:  base64.StdEncoding.EncodeToString(blob),
	}
}

type HealthResponse struct {
	OK         bool   `json:"ok"`
	DBOK       bool   `json:"dbOk"`
	Generation uint64 `json:"generation"`
	Epoch      int64  `json:"epoch"`
}

type ResetRequest struct {
	RngSeed *int64 `json:"rngSeed"`
}

type ReseedRequest struct {
	Probability *float64 `json:"probability"`
	RngSeed     *int64   `json:"rngSeed"`
}

type ProbabilityRequest struct {
	Probability float64 `json:"probability"`
	Reseed      bool    `json:"reseed"`
}

type SizeRequest struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type TickRateRequest struct {
	TickHz float64 `json:"tickHz"`
}

type StreamRateRequest struct {
	StreamEveryN int `json:"streamEveryN"`
}

type SetCellsRequest struct {
	Cells [][3]int `json:"cells"` // [x, y, alive] triplets, alive is 0 or 1
}

type ErrorResponse struct {
	Error string `json:"error"`
}
