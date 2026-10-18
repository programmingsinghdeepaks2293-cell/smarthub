// Template for your own device. Copy this folder, rename the package and kind,
// then replace the logic with calls to your hardware.
package example

import (
	"context"
	"fmt"
	"sync"
	"time"

	"smarthub/internal/core"
)

func init() { core.Register("example.virtual", New) }

type virtual struct {
	info core.Info
	mu   sync.Mutex
	on   bool
}

func New(spec core.Spec) (core.Device, error) {
	return &virtual{info: core.Info{
		ID: spec.ID, Name: spec.Name, Kind: spec.Kind,
		Vendor: "Virtual", Transport: "none",
		Capabilities: []core.Capability{core.CapSwitch, core.CapPowerMeter},
	}}, nil
}

func (v *virtual) Info() core.Info { return v.info }

func (v *virtual) State(ctx context.Context) (core.State, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	w := 0.0
	if v.on {
		w = 5
	}
	return core.State{Online: true, On: v.on, PowerW: &w, PowerEstimated: true, UpdatedAt: time.Now()}, nil
}

func (v *virtual) Execute(ctx context.Context, cmd core.Command) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	switch cmd.Action {
	case "on":
		v.on = true
	case "off":
		v.on = false
	case "toggle":
		v.on = !v.on
	default:
		return fmt.Errorf("example: unsupported action %q", cmd.Action)
	}
	return nil
}
