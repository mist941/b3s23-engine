package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/mist941/b3s23-engine/engine/internal/engine"
	"github.com/mist941/b3s23-engine/engine/internal/gridcodec"
)

type API struct {
	sim    *engine.Simulation
	logger *slog.Logger
}

func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, statusFromView(a.sim.CurrentView()))
}

func (a *API) handleGrid(w http.ResponseWriter, r *http.Request) {
	v := a.sim.CurrentView()
	if v == nil || v.Grid == nil {
		writeError(w, http.StatusServiceUnavailable, "state not initialized")
		return
	}
	blob, err := gridcodec.Encode(v.Grid, gridcodec.CodecRaw)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode grid: "+err.Error())
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		writeJSON(w, http.StatusOK, gridJSONFromView(v, blob))
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Generation", strconv.FormatUint(v.Generation, 10))
	w.Header().Set("X-Epoch", strconv.FormatInt(v.Epoch, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(blob)
}

func (a *API) handlePlay(w http.ResponseWriter, r *http.Request) {
	a.runControl(w, r, a.sim.Play)
}

func (a *API) handlePause(w http.ResponseWriter, r *http.Request) {
	a.runControl(w, r, a.sim.Pause)
}

func (a *API) handleStep(w http.ResponseWriter, r *http.Request) {
	a.runControl(w, r, a.sim.Step)
}

func (a *API) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	a.runControl(w, r, a.sim.ForceSnapshot)
}

func (a *API) runControl(w http.ResponseWriter, r *http.Request, fn func(context.Context) error) {
	if err := fn(r.Context()); err != nil {
		writeCommandError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, statusFromView(a.sim.CurrentView()))
}

func (a *API) handleReset(w http.ResponseWriter, r *http.Request) {
	var req ResetRequest
	if !decodeOptional(w, r, &req) {
		return
	}
	var seed *uint64
	if req.RngSeed != nil {
		s := uint64(*req.RngSeed)
		seed = &s
	}
	if err := a.sim.Reset(r.Context(), seed); err != nil {
		writeCommandError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, statusFromView(a.sim.CurrentView()))
}

func (a *API) handleReseed(w http.ResponseWriter, r *http.Request) {
	var req ReseedRequest
	if !decodeOptional(w, r, &req) {
		return
	}
	var seed *uint64
	if req.RngSeed != nil {
		s := uint64(*req.RngSeed)
		seed = &s
	}
	if err := a.sim.Reseed(r.Context(), req.Probability, seed); err != nil {
		writeCommandError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, statusFromView(a.sim.CurrentView()))
}

func (a *API) handleProbability(w http.ResponseWriter, r *http.Request) {
	var req ProbabilityRequest
	if !decodeRequired(w, r, &req) {
		return
	}
	if err := a.sim.SetProbability(r.Context(), req.Probability, req.Reseed); err != nil {
		writeCommandError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, statusFromView(a.sim.CurrentView()))
}

func (a *API) handleSize(w http.ResponseWriter, r *http.Request) {
	var req SizeRequest
	if !decodeRequired(w, r, &req) {
		return
	}
	if err := a.sim.SetSize(r.Context(), req.Width, req.Height); err != nil {
		writeCommandError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, statusFromView(a.sim.CurrentView()))
}

func (a *API) handleTickRate(w http.ResponseWriter, r *http.Request) {
	var req TickRateRequest
	if !decodeRequired(w, r, &req) {
		return
	}
	if req.TickHz <= 0 {
		writeError(w, http.StatusBadRequest, "tickHz must be > 0")
		return
	}
	if err := a.sim.SetTickRate(r.Context(), req.TickHz); err != nil {
		writeCommandError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, statusFromView(a.sim.CurrentView()))
}

func (a *API) handleStreamRate(w http.ResponseWriter, r *http.Request) {
	var req StreamRateRequest
	if !decodeRequired(w, r, &req) {
		return
	}
	if req.StreamEveryN < 1 {
		writeError(w, http.StatusBadRequest, "streamEveryN must be >= 1")
		return
	}
	if err := a.sim.SetStreamEveryN(r.Context(), req.StreamEveryN); err != nil {
		writeCommandError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, statusFromView(a.sim.CurrentView()))
}

func (a *API) handleSetCells(w http.ResponseWriter, r *http.Request) {
	var req SetCellsRequest
	if !decodeRequired(w, r, &req) {
		return
	}
	if len(req.Cells) == 0 {
		writeError(w, http.StatusBadRequest, "cells must not be empty")
		return
	}
	cells := make([]engine.Cell, len(req.Cells))
	for i, t := range req.Cells {
		if t[2] != 0 && t[2] != 1 {
			writeError(w, http.StatusBadRequest, "alive flag must be 0 or 1")
			return
		}
		cells[i] = engine.Cell{X: t[0], Y: t[1], Alive: t[2] == 1}
	}
	if err := a.sim.SetCells(r.Context(), cells); err != nil {
		writeCommandError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, statusFromView(a.sim.CurrentView()))
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	err := a.sim.Healthy(r.Context())
	v := a.sim.CurrentView()
	resp := HealthResponse{OK: err == nil, DBOK: err == nil}
	if v != nil {
		resp.Generation = v.Generation
		resp.Epoch = v.Epoch
	}
	code := http.StatusOK
	if err != nil {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, resp)
}

func decodeRequired(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return false
	}
	return true
}

func decodeOptional(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return true // empty body: use defaults
		}
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return false
	}
	return true
}
