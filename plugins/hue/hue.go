package hue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"smarthub/internal/core"
)

func init() { core.Register("hue.light", New) }

type config struct {
	Bridge     string  `json:"bridge"`
	Username   string  `json:"username"`
	LightID    string  `json:"light_id"`
	RatedWatts float64 `json:"rated_watts"`
}

type light struct {
	info core.Info
	cfg  config
	http *http.Client
}

func New(spec core.Spec) (core.Device, error) {
	var cfg config
	if err := json.Unmarshal(spec.Config, &cfg); err != nil {
		return nil, err
	}
	if cfg.Bridge == "" || cfg.Username == "" || cfg.LightID == "" {
		return nil, fmt.Errorf("hue: bridge, username and light_id are required")
	}
	if cfg.RatedWatts <= 0 {
		cfg.RatedWatts = 9
	}
	return &light{
		info: core.Info{
			ID: spec.ID, Name: spec.Name, Kind: spec.Kind,
			Vendor: "Philips Hue", Transport: "zigbee (hue bridge)",
			Capabilities: []core.Capability{core.CapSwitch, core.CapBrightness, core.CapPowerMeter},
		},
		cfg:  cfg,
		http: &http.Client{Timeout: 5 * time.Second},
	}, nil
}

func (l *light) Info() core.Info { return l.info }

type hueLight struct {
	State struct {
		On        bool `json:"on"`
		Bri       int  `json:"bri"`
		Reachable bool `json:"reachable"`
	} `json:"state"`
}

func (l *light) endpoint(suffix string) string {
	return fmt.Sprintf("http://%s/api/%s/lights/%s%s", l.cfg.Bridge, l.cfg.Username, l.cfg.LightID, suffix)
}

func (l *light) State(ctx context.Context) (core.State, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.endpoint(""), nil)
	if err != nil {
		return core.State{}, err
	}
	resp, err := l.http.Do(req)
	if err != nil {
		return core.State{Online: false, Error: err.Error(), UpdatedAt: time.Now()}, err
	}
	defer resp.Body.Close()

	var hl hueLight
	if err := json.NewDecoder(resp.Body).Decode(&hl); err != nil {
		return core.State{Online: false, Error: err.Error(), UpdatedAt: time.Now()}, err
	}
	if !hl.State.Reachable {
		return core.State{Online: false, Error: "bulb not reachable from bridge", UpdatedAt: time.Now()}, nil
	}

	bri := int(math.Round(float64(hl.State.Bri) / 254 * 100))
	watts := 0.0
	if hl.State.On {
		// Hue bulbs don't measure power: estimate rated watts x brightness fraction.
		watts = l.cfg.RatedWatts * float64(hl.State.Bri) / 254
	}
	return core.State{
		Online: true, On: hl.State.On, Brightness: &bri,
		PowerW: &watts, PowerEstimated: true, UpdatedAt: time.Now(),
	}, nil
}

func (l *light) Execute(ctx context.Context, cmd core.Command) error {
	switch cmd.Action {
	case "on":
		return l.put(ctx, map[string]any{"on": true})
	case "off":
		return l.put(ctx, map[string]any{"on": false})
	case "toggle":
		st, err := l.State(ctx)
		if err != nil {
			return err
		}
		return l.put(ctx, map[string]any{"on": !st.On})
	case "brightness":
		var pct float64
		if err := json.Unmarshal(cmd.Value, &pct); err != nil {
			return fmt.Errorf("hue: brightness must be a number")
		}
		pct = math.Max(1, math.Min(100, pct))
		bri := int(math.Round(pct / 100 * 254))
		return l.put(ctx, map[string]any{"on": true, "bri": bri})
	}
	return fmt.Errorf("hue: unsupported action %q", cmd.Action)
}

func (l *light) put(ctx context.Context, body map[string]any) error {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, l.endpoint("/state"), bytes.NewReader(b))
	if err != nil {
		return err
	}
	resp, err := l.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var res []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}
	for _, item := range res {
		if e, ok := item["error"].(map[string]any); ok {
			return fmt.Errorf("hue: %v", e["description"])
		}
	}
	return nil
}
