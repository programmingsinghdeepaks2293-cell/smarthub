package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"smarthub/internal/core"
	"smarthub/internal/insights"
	"smarthub/internal/manager"
	"smarthub/internal/store"
	"smarthub/internal/tariff"
)

type Options struct {
	Poll    time.Duration
	OnWatts map[string]float64
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
	mux.HandleFunc("GET /api/energy/current", s.current)
	mux.HandleFunc("GET /api/energy/monthly", s.monthly)
	mux.HandleFunc("GET /api/tariff", s.tariffInfo)
	mux.HandleFunc("GET /api/retention", s.retention)
	mux.HandleFunc("GET /api/insights", s.insights)
	mux.HandleFunc("DELETE /api/data", s.deleteAll)
	mux.HandleFunc("DELETE /api/data/devices/{id}", s.deleteDevice)
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

// meta returns the current device views keyed by ID.
func (s *Server) meta() map[string]manager.View {
	out := map[string]manager.View{}
	for _, v := range s.mgr.Snapshot() {
		out[v.ID] = v
	}
	return out
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

// monthStart returns the first instant of t's month in t's location.
func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func totalKWh(rows []store.DeviceEnergy) float64 {
	var wh float64
	for _, r := range rows {
		wh += r.Wh
	}
	return wh / 1000
}

// avgW converts summed watt-hours into average watts from the sample count.
func avgW(wh float64, samples int64, poll time.Duration) float64 {
	hours := float64(samples) * poll.Hours()
	if hours == 0 {
		return 0
	}
	return wh / hours
}

func httpErr(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

type energyRow struct {
	DeviceID  string  `json:"device_id"`
	Name      string  `json:"name"`
	KWh       float64 `json:"kwh"`
	AvgW      float64 `json:"avg_w"`
	Cost      float64 `json:"cost"`
	Estimated bool    `json:"estimated"`
}

// summary covers the selected range. Its cost is priced on top of the units
// already used earlier this month, so it follows the slab rates.
func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	win := parseRange(r.URL.Query().Get("range"))
	now := time.Now()
	since := now.Add(-win.d)

	rows, err := s.st.EnergyBetween(since.Unix(), now.Unix()+1)
	if err != nil {
		httpErr(w, err)
		return
	}
	var prior float64
	if ms := monthStart(now); since.After(ms) {
		pre, err := s.st.EnergyBetween(ms.Unix(), since.Unix())
		if err != nil {
			httpErr(w, err)
			return
		}
		prior = totalKWh(pre)
	}

	total := totalKWh(rows)
	cost := tariff.Cost(prior, total)
	meta := s.meta()
	out := make([]energyRow, 0, len(rows))
	for _, row := range rows {
		kwh := row.Wh / 1000
		share := 0.0
		if total > 0 {
			share = kwh / total
		}
		m := meta[row.DeviceID]
		out = append(out, energyRow{
			DeviceID:  row.DeviceID,
			Name:      m.Name,
			KWh:       kwh,
			AvgW:      avgW(row.Wh, row.Samples, s.opts.Poll),
			Cost:      cost * share,
			Estimated: m.State.PowerEstimated,
		})
	}
	writeJSON(w, out)
}

type devMonth struct {
	DeviceID string  `json:"device_id"`
	Name     string  `json:"name"`
	KWh      float64 `json:"kwh"`
	Cost     float64 `json:"cost"`
}

// devMonths splits a month's bill across devices in proportion to their energy.
func (s *Server) devMonths(rows []store.DeviceEnergy, total, bill float64) []devMonth {
	meta := s.meta()
	out := make([]devMonth, 0, len(rows))
	for _, r := range rows {
		kwh := r.Wh / 1000
		cost := 0.0
		if total > 0 {
			cost = bill * kwh / total
		}
		out = append(out, devMonth{r.DeviceID, meta[r.DeviceID].Name, kwh, cost})
	}
	return out
}

func (s *Server) current(w http.ResponseWriter, _ *http.Request) {
	ms := monthStart(time.Now())
	rows, err := s.st.EnergyBetween(ms.Unix(), ms.AddDate(0, 1, 0).Unix())
	if err != nil {
		httpErr(w, err)
		return
	}
	total := totalKWh(rows)
	bill := tariff.Billed(total)
	writeJSON(w, map[string]any{
		"month":   ms.Format("2006-01"),
		"kwh":     total,
		"bill":    bill,
		"slabs":   tariff.Breakdown(total),
		"devices": s.devMonths(rows, total, bill),
	})
}

type monthRow struct {
	Month   string     `json:"month"`
	KWh     float64    `json:"kwh"`
	Bill    float64    `json:"bill"`
	Devices []devMonth `json:"devices"`
}

func (s *Server) monthly(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.URL.Query().Get("months"))
	if err != nil || n < 1 {
		n = 12
	}
	if n > 120 {
		n = 120
	}
	base := monthStart(time.Now())
	out := make([]monthRow, 0, n)
	for i := n - 1; i >= 0; i-- {
		ms := base.AddDate(0, -i, 0)
		rows, err := s.st.EnergyBetween(ms.Unix(), ms.AddDate(0, 1, 0).Unix())
		if err != nil {
			httpErr(w, err)
			return
		}
		total := totalKWh(rows)
		bill := tariff.Billed(total)
		out = append(out, monthRow{ms.Format("2006-01"), total, bill, s.devMonths(rows, total, bill)})
	}
	writeJSON(w, out)
}

func (s *Server) tariffInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"slabs": tariff.Slabs})
}

func (s *Server) retention(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]int{
		"raw_days":     store.RawRetentionDays,
		"hourly_years": store.HourlyRetentionYears,
	})
}

// Deletes require ?confirm=DELETE so a stray request can't erase history.
func confirmed(r *http.Request) bool {
	return r.URL.Query().Get("confirm") == "DELETE"
}

func (s *Server) deleteAll(w http.ResponseWriter, r *http.Request) {
	if !confirmed(r) {
		http.Error(w, "add ?confirm=DELETE to erase energy data", http.StatusBadRequest)
		return
	}
	if err := s.st.DeleteAll(); err != nil {
		httpErr(w, err)
		return
	}
	log.Printf("user deleted all energy data")
	writeJSON(w, map[string]string{"status": "deleted", "scope": "all"})
}

func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	if !confirmed(r) {
		http.Error(w, "add ?confirm=DELETE to erase device data", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if err := s.st.DeleteDevice(id); err != nil {
		httpErr(w, err)
		return
	}
	log.Printf("user deleted data for device %s", id)
	writeJSON(w, map[string]string{"status": "deleted", "scope": id})
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

func (s *Server) insights(w http.ResponseWriter, _ *http.Request) {
	var devs []insights.Device
	for _, v := range s.mgr.Snapshot() {
		if !v.Has(core.CapPowerMeter) {
			continue
		}
		devs = append(devs, insights.Device{ID: v.ID, Name: v.Name, OnW: s.opts.OnWatts[v.ID]})
	}
	tips, err := insights.Analyze(s.st, devs, time.Now(), int64(s.opts.Poll.Seconds()))
	if err != nil {
		httpErr(w, err)
		return
	}
	writeJSON(w, tips)
}
