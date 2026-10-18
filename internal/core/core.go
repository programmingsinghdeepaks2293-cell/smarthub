package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

type Capability string

const (
	CapSwitch      Capability = "switch"
	CapBrightness  Capability = "brightness"
	CapPowerMeter  Capability = "power_meter"
	CapScenes      Capability = "scenes"
	CapColor       Capability = "color"
	CapTemperature Capability = "temperature"
)

// Scene is a preset a device supports, grouped the way the vendor app shows it.
type Scene struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

// Info describes a device. The frontend renders cards from this alone.
type Info struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Kind         string       `json:"kind"`
	Vendor       string       `json:"vendor"`
	Transport    string       `json:"transport"`
	Capabilities []Capability `json:"capabilities"`
	Scenes       []Scene      `json:"scenes,omitempty"`
}

func (i Info) Has(c Capability) bool {
	for _, x := range i.Capabilities {
		if x == c {
			return true
		}
	}
	return false
}

type State struct {
	Online         bool      `json:"online"`
	On             bool      `json:"on"`
	Brightness     *int      `json:"brightness,omitempty"`
	PowerW         *float64  `json:"power_w,omitempty"`
	PowerEstimated bool      `json:"power_estimated,omitempty"`
	Scene          *int      `json:"scene,omitempty"`
	Error          string    `json:"error,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Command struct {
	Action string          `json:"action"`
	Value  json.RawMessage `json:"value,omitempty"`
}

// Device is the only thing a plugin must implement.
type Device interface {
	Info() Info
	State(ctx context.Context) (State, error)
	Execute(ctx context.Context, cmd Command) error
}

// Spec is one entry from devices.json.
type Spec struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Kind   string          `json:"kind"`
	Config json.RawMessage `json:"config"`
}

type Factory func(spec Spec) (Device, error)

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
)

// Register is called from each plugin's init().
func Register(kind string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := factories[kind]; dup {
		panic("duplicate plugin kind: " + kind)
	}
	factories[kind] = f
}

func Build(spec Spec) (Device, error) {
	mu.RLock()
	f, ok := factories[spec.Kind]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown device kind %q", spec.Kind)
	}
	return f(spec)
}

func Kinds() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(factories))
	for k := range factories {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
