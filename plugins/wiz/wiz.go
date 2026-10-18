package wiz

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"smarthub/internal/core"
)

func init() { core.Register("wiz.bulb", New) }

// Scene IDs from the WiZ local protocol, grouped like the WiZ app.
// If a name triggers a different effect on your firmware, swap its ID.
var scenes = []core.Scene{
	{ID: 11, Name: "Warm white", Group: "White"},
	{ID: 12, Name: "Daylight", Group: "White"},
	{ID: 13, Name: "Cool white", Group: "White"},
	{ID: 14, Name: "Night light", Group: "Functional"},
	{ID: 6, Name: "Cozy", Group: "Functional"},
	{ID: 17, Name: "True colors", Group: "Functional"},
	{ID: 16, Name: "Relax", Group: "Functional"},
	{ID: 15, Name: "Focus", Group: "Functional"},
	{ID: 18, Name: "TV time", Group: "Functional"},
	{ID: 19, Name: "Plant growth", Group: "Functional"},
	{ID: 9, Name: "Wake-up", Group: "Progressive"},
	{ID: 10, Name: "Bedtime", Group: "Progressive"},
	{ID: 5, Name: "Fireplace", Group: "Dynamic"},
	{ID: 22, Name: "Fall", Group: "Dynamic"},
	{ID: 26, Name: "Club", Group: "Dynamic"},
	{ID: 3, Name: "Sunset", Group: "Dynamic"},
	{ID: 2, Name: "Romance", Group: "Dynamic"},
	{ID: 4, Name: "Party", Group: "Dynamic"},
	{ID: 8, Name: "Pastel colors", Group: "Dynamic"},
	{ID: 20, Name: "Spring", Group: "Dynamic"},
	{ID: 21, Name: "Summer", Group: "Dynamic"},
	{ID: 7, Name: "Forest", Group: "Dynamic"},
	{ID: 24, Name: "Jungle", Group: "Dynamic"},
	{ID: 25, Name: "Mojito", Group: "Dynamic"},
	{ID: 1, Name: "Ocean", Group: "Dynamic"},
	{ID: 23, Name: "Deep dive", Group: "Dynamic"},
	{ID: 27, Name: "Christmas", Group: "Dynamic"},
	{ID: 33, Name: "Diwali", Group: "Dynamic"},
	{ID: 28, Name: "Halloween", Group: "Dynamic"},
	{ID: 29, Name: "Candlelight", Group: "Dynamic"},
	{ID: 31, Name: "Pulse", Group: "Dynamic"},
	{ID: 30, Name: "Golden white", Group: "Dynamic"},
}

type config struct {
	Host       string  `json:"host"`
	RatedWatts float64 `json:"rated_watts"`
}

type bulb struct {
	info core.Info
	cfg  config
}

func New(spec core.Spec) (core.Device, error) {
	var cfg config
	if err := json.Unmarshal(spec.Config, &cfg); err != nil {
		return nil, err
	}
	if cfg.Host == "" {
		return nil, fmt.Errorf("wiz: host is required")
	}
	if cfg.RatedWatts <= 0 {
		cfg.RatedWatts = 9
	}
	return &bulb{cfg: cfg, info: core.Info{
		ID: spec.ID, Name: spec.Name, Kind: spec.Kind,
		Vendor: "WiZ (Philips)", Transport: "wifi (udp)",
		Capabilities: []core.Capability{
			core.CapSwitch, core.CapBrightness, core.CapPowerMeter,
			core.CapScenes, core.CapColor, core.CapTemperature,
		},
		Scenes: scenes,
	}}, nil
}

func (b *bulb) Info() core.Info { return b.info }

// send writes one UDP datagram to the bulb and, if out is set, decodes the reply.
func (b *bulb) send(ctx context.Context, msg map[string]any, out any) error {
	conn, err := net.Dial("udp", net.JoinHostPort(b.cfg.Host, "38899"))
	if err != nil {
		return err
	}
	defer conn.Close()

	deadline := time.Now().Add(5 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	payload, _ := json.Marshal(msg)
	if _, err := conn.Write(payload); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf[:n], out)
}

type pilotReply struct {
	Result struct {
		State   *bool `json:"state"`
		Dimming int   `json:"dimming"`
		SceneID int   `json:"sceneId"`
	} `json:"result"`
}

func (b *bulb) State(ctx context.Context) (core.State, error) {
	var r pilotReply
	if err := b.send(ctx, map[string]any{"method": "getPilot"}, &r); err != nil {
		return core.State{Online: false, Error: err.Error(), UpdatedAt: time.Now()}, err
	}
	on := r.Result.State != nil && *r.Result.State
	bri := r.Result.Dimming
	watts := 0.0
	if on {
		// WiZ bulbs don't report power, so estimate from dimming level.
		watts = b.cfg.RatedWatts * float64(bri) / 100
	}
	var scene *int
	if r.Result.SceneID != 0 {
		id := r.Result.SceneID
		scene = &id
	}
	return core.State{
		Online: true, On: on, Brightness: &bri, Scene: scene,
		PowerW: &watts, PowerEstimated: true, UpdatedAt: time.Now(),
	}, nil
}

type rgb struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

func clamp[T int | float64](v, lo, hi T) T {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func validScene(id int) bool {
	for _, s := range scenes {
		if s.ID == id {
			return true
		}
	}
	return false
}

func (b *bulb) Execute(ctx context.Context, cmd core.Command) error {
	switch cmd.Action {
	case "on":
		return b.set(ctx, map[string]any{"state": true})
	case "off":
		return b.set(ctx, map[string]any{"state": false})
	case "toggle":
		st, err := b.State(ctx)
		if err != nil {
			return err
		}
		return b.set(ctx, map[string]any{"state": !st.On})

	case "brightness":
		var pct float64
		if err := json.Unmarshal(cmd.Value, &pct); err != nil {
			return fmt.Errorf("wiz: brightness must be a number")
		}
		pct = clamp(pct, 10, 100) // WiZ minimum dimming is 10%
		return b.set(ctx, map[string]any{"state": true, "dimming": int(pct)})

	case "scene":
		var id int
		if err := json.Unmarshal(cmd.Value, &id); err != nil || !validScene(id) {
			return fmt.Errorf("wiz: unknown scene")
		}
		return b.set(ctx, map[string]any{"state": true, "sceneId": id})

	case "color":
		var c rgb
		if err := json.Unmarshal(cmd.Value, &c); err != nil {
			return fmt.Errorf("wiz: color must be {r,g,b}")
		}
		return b.set(ctx, map[string]any{
			"state": true,
			"r": clamp(c.R, 0, 255), "g": clamp(c.G, 0, 255), "b": clamp(c.B, 0, 255),
		})

	case "temp":
		var k float64
		if err := json.Unmarshal(cmd.Value, &k); err != nil {
			return fmt.Errorf("wiz: temp must be a number")
		}
		return b.set(ctx, map[string]any{"state": true, "temp": int(clamp(k, 2200, 6500))})
	}
	return fmt.Errorf("wiz: unsupported action %q", cmd.Action)
}

func (b *bulb) set(ctx context.Context, params map[string]any) error {
	var r struct {
		Result struct {
			Success bool `json:"success"`
		} `json:"result"`
	}
	msg := map[string]any{"method": "setPilot", "params": params}
	if err := b.send(ctx, msg, &r); err != nil {
		return err
	}
	if !r.Result.Success {
		return fmt.Errorf("wiz: bulb rejected command")
	}
	return nil
}
