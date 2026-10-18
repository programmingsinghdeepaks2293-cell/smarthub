// Package tariff prices energy using Indian residential slab rates.
// Each slab's rate applies only to units inside that slab, so the cost of
// extra energy depends on how much was already used this month.
package tariff

import (
	"fmt"
	"math"
)

type Slab struct {
	UpTo float64 `json:"up_to"` // upper limit in units (kWh); 0 means no limit
	Rate float64 `json:"rate"`  // ₹ per unit
}

// FY 2026-27 residential slabs. Verify against your bill; utilities revise these.
var Slabs = []Slab{
	{UpTo: 100, Rate: 5.56},
	{UpTo: 300, Rate: 12.40},
	{UpTo: 500, Rate: 16.64},
	{UpTo: 0, Rate: 19.13},
}

type Line struct {
	Label  string  `json:"label"`
	Units  float64 `json:"units"`
	Rate   float64 `json:"rate"`
	Amount float64 `json:"amount"`
}

// Billed returns the charge for a total of `units` in one month.
func Billed(units float64) float64 {
	total, prev := 0.0, 0.0
	for _, s := range Slabs {
		if units <= prev {
			break
		}
		top := slabTop(s)
		total += (math.Min(units, top) - prev) * s.Rate
		prev = top
	}
	return total
}

// Cost returns the charge for `kwh` added on top of `monthToDate` units.
func Cost(monthToDate, kwh float64) float64 {
	return Billed(monthToDate+kwh) - Billed(monthToDate)
}

// Breakdown lists how `units` fills each slab, for display.
func Breakdown(units float64) []Line {
	var out []Line
	prev := 0.0
	for _, s := range Slabs {
		if units <= prev {
			break
		}
		top := slabTop(s)
		label := fmt.Sprintf("%.0f–%.0f units", prev, top)
		if math.IsInf(top, 1) {
			label = fmt.Sprintf("above %.0f units", prev)
		}
		u := math.Min(units, top) - prev
		out = append(out, Line{Label: label, Units: u, Rate: s.Rate, Amount: u * s.Rate})
		prev = top
	}
	return out
}

func slabTop(s Slab) float64 {
	if s.UpTo == 0 {
		return math.Inf(1)
	}
	return s.UpTo
}
