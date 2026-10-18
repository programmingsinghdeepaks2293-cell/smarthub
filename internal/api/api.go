package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"smarthub/internal/core"
	"smarthub/internal/manager"
	"smarthub/internal/store"
)

type Options struct {
	Poll         time.Duration
	TariffPerKWh float64
}

type Server struct {
	mgr  *manager.Manager
	st   *store.Store
	opts Options
}

func New(m *manager.Manager, st *store.Store, opts Options) http.Handler {
	s := &Server{mgr: m, st: st, opts: opts}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/plugins", s.plugins)
	mux.HandleFunc("GET /api/devices", s.devices)
	mux.HandleFunc("POST /api/devices/{id}/command", s.command)
	mux.HandleFunc("GET /api/telemetry/series", s.series)
	mux.HandleFunc("GET /api/energy/summary", s.summary)
	mux.HandleFunc("GET /api/events", s.events)
	return mux
}

func (s *Server) plugins(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, core.Kinds())
}

func (s *Server) devices(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.mgr.Snapshot())
}

func (s *Server) command(w http.ResponseWriter, r *http.Request) {
	var cmd core.Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	err := s.mgr.Execute(r.Context(), r.PathValue("id"), cmd)
	switch {
	case errors.Is(err, manager.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case err != nil:
		http.Error(w, err.Error(), http.StatusBadGateway)
	default:
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

type window struct {
	d      time.Duration
	bucket int64
}

func parseRange(s string) window {
	switch s {
	case "24h":
		return window{24 * time.Hour, 900}
	case "7d":
		return window{7 * 24 * time.Hour, 3600}
	default:
		return window{time.Hour, 60}
	}
}

func (s *Server) series(w http.ResponseWriter, r *http.Request) {
	win := parseRange(r.URL.Query().Get("range"))
	now := time.Now().Unix()
	since := now - int64(win.d.Seconds())

	type out struct {
		DeviceID string        `json:"device_id"`
		Name     string        `json:"name"`
		Points   []store.Point `json:"points"`
	}
	res := []out{}
	for _, v := range s.mgr.Snapshot() {
		if !v.Has(core.CapPowerMeter) {
			continue
		}
		pts, err := s.st.Series(v.ID, since, now, win.bucket)
		if err != nil {
			log.Printf("series %s: %v", v.ID, err)
			continue
		}
		res = append(res, out{v.ID, v.Name, pts})
	}
	writeJSON(w, res)
}

type energyRow struct {
	DeviceID  string  `json:"device_id"`
	Name      string  `json:"name"`
	KWh       float64 `json:"kwh"`
	AvgW      float64 `json:"avg_w"`
	Cost      float64 `json:"cost"`
	Estimated bool    `json:"estimated"`
}

func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	win := parseRange(r.URL.Query().Get("range"))
	since := time.Now().Add(-win.d).Unix()

	rows, err := s.st.Energy(since, s.opts.Poll.Seconds())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	meta := map[string]manager.View{}
	for _, v := range s.mgr.Snapshot() {
		meta[v.ID] = v
	}
	out := make([]energyRow, 0, len(rows))
	for _, row := range rows {
		m := meta[row.DeviceID]
		out = append(out, energyRow{
			DeviceID:  row.DeviceID,
			Name:      m.Name,
			KWh:       row.KWh,
			AvgW:      row.AvgW,
			Cost:      row.KWh * s.opts.TariffPerKWh,
			Estimated: m.State.PowerEstimated,
		})
	}
	writeJSON(w, out)
}

// events streams device state to the browser using Server-Sent Events.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.mgr.Subscribe()
	defer s.mgr.Unsubscribe(ch)

	for _, v := range s.mgr.Snapshot() {
		writeEvent(w, "state", v)
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case v := <-ch:
			writeEvent(w, "state", v)
			flusher.Flush()
		}
	}
}

func writeEvent(w io.Writer, name string, v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
