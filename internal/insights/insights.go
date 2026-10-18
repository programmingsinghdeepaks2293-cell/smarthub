// Package insights finds usage patterns in stored power data using rules.
// Every tip is traceable to the samples that produced it.
package insights

import (
	"fmt"
	"sort"
	"time"

	"smarthub/internal/store"
	"smarthub/internal/tariff"
)

const defaultOnW = 10.0

const (
	minRunSec = 120  // ignore blips shorter than 2 minutes
	minRunWh  = 20.0 // ignore runs that used less than 20 Wh
)

type Device struct {
	ID   string
	Name string
	OnW  float64 // watts above which the device counts as running; 0 uses the default
}

type Tip struct {
	Severity string `json:"severity"` // "info", "tip" or "warn"
	DeviceID string `json:"device_id,omitempty"`
	Device   string `json:"device,omitempty"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
}

type run struct {
	start, end int64
	n          int
	wh         float64
}

// runs groups consecutive samples at or above onW into runs of at least two samples.
func runs(samples []store.Sample, onW float64, intervalSec int64) []run {
	var out []run
	var cur *run
	flush := func() {
		if cur != nil && cur.n >= 2 && cur.end-cur.start >= minRunSec && cur.wh >= minRunWh {
			out = append(out, *cur)
		}
		cur = nil
	}
	for _, s := range samples {
		if s.W < onW {
			flush()
			continue
		}
		if cur == nil {
			cur = &run{start: s.T}
		}
		cur.end = s.T
		cur.n++
		cur.wh += s.W * float64(intervalSec) / 3600
	}
	flush()
	return out
}

// Analyze runs every rule and returns the tips for the current state.
func Analyze(st *store.Store, devs []Device, now time.Time, intervalSec int64) ([]Tip, error) {
	tips := []Tip{}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	dayAgo := now.Add(-24 * time.Hour).Unix()
	weekAgo := now.AddDate(0, 0, -7).Unix()

	for _, d := range devs {
		onW := d.OnW
		if onW <= 0 {
			onW = defaultOnW
		}
		week, err := st.Samples(d.ID, weekAgo, now.Unix())
		if err != nil {
			return nil, err
		}
		if len(week) == 0 {
			continue
		}
		all := runs(week, onW, intervalSec)

		// Repeated cycles today.
		var today []run
		var todayWh float64
		for _, r := range all {
			if r.start >= dayStart {
				today = append(today, r)
				todayWh += r.wh
			}
		}
		if len(today) >= 2 {
			tips = append(tips, Tip{
				Severity: "tip", DeviceID: d.ID, Device: d.Name,
				Title: fmt.Sprintf("%s ran %d times today", d.Name, len(today)),
				Detail: fmt.Sprintf("Those runs used %.3f kWh in total. If the load allows, combining them into fewer, fuller cycles saves energy.",
					todayWh/1000),
			})
		}

		// Short cycling: four or more starts within one hour.
		for i := range all {
			if all[i].start < dayAgo {
				continue
			}
			n := 0
			for j := i; j < len(all) && all[j].start < all[i].start+3600; j++ {
				n++
			}
			if n >= 4 {
				tips = append(tips, Tip{
					Severity: "warn", DeviceID: d.ID, Device: d.Name,
					Title:  "Turning on and off often",
					Detail: fmt.Sprintf("It started %d times within an hour. Frequent cycling wastes energy and wears the device. Check its thermostat or timer settings.", n),
				})
				break
			}
		}

		// Long continuous run in the last day.
		for _, r := range all {
			if r.start >= dayAgo && r.end-r.start >= 4*3600 {
				tips = append(tips, Tip{
					Severity: "tip", DeviceID: d.ID, Device: d.Name,
					Title:  "Ran for a long time",
					Detail: fmt.Sprintf("It ran continuously for %.1f hours. An auto-off timer stops idle running.", float64(r.end-r.start)/3600),
				})
				break
			}
		}

		// Standby: most samples are below the on threshold but show a steady trickle.
		var idle []float64
		for _, s := range week {
			if s.W < onW {
				idle = append(idle, s.W)
			}
		}
		if len(idle)*2 > len(week) {
			sort.Float64s(idle)
			med := idle[len(idle)/2]
			if med >= 1 {
				tips = append(tips, Tip{
					Severity: "info", DeviceID: d.ID, Device: d.Name,
					Title: "Standby power",
					Detail: fmt.Sprintf("It draws about %.1f W while idle, roughly %.0f kWh a year. Switching it off at the wall stops this.",
						med, med*8760/1000),
				})
			}
		}
	}

	// Warn when this month's total is close to a higher slab.
	ms := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	rows, err := st.EnergyBetween(ms.Unix(), now.Unix()+1)
	if err != nil {
		return nil, err
	}
	var units float64
	for _, r := range rows {
		units += r.Wh / 1000
	}
	for _, s := range tariff.Slabs {
		if s.UpTo == 0 {
			continue
		}
		if units < s.UpTo && s.UpTo-units <= 20 {
			tips = append(tips, Tip{
				Severity: "warn",
				Title:    "Close to a higher slab",
				Detail: fmt.Sprintf("SmartHub's devices have used %.1f units this month, %.1f short of %.0f. Units beyond that are billed at a higher rate.",
					units, s.UpTo-units, s.UpTo),
			})
			break
		}
	}
	return tips, nil
}
