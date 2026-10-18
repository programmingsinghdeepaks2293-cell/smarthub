package tapo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"smarthub/internal/core"
)

func init() { core.Register("tapo.plug", New) }

type config struct {
	Host string `json:"host"`
}

type plug struct {
	info   core.Info
	host   string
	bridge string
	http   *http.Client
}

func New(spec core.Spec) (core.Device, error) {
	var cfg config
	if err := json.Unmarshal(spec.Config, &cfg); err != nil {
		return nil, err
	}
	if cfg.Host == "" {
		return nil, fmt.Errorf("tapo: host is required")
	}
	bridge := os.Getenv("TAPO_BRIDGE_URL")
	if bridge == "" {
		bridge = "http://127.0.0.1:8765"
	}
	return &plug{
		info: core.Info{
			ID: spec.ID, Name: spec.Name, Kind: spec.Kind,
			Vendor: "TP-Link Tapo", Transport: "wifi",
			Capabilities: []core.Capability{core.CapSwitch, core.CapPowerMeter},
		},
		host:   cfg.Host,
		bridge: bridge,
		http:   &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (p *plug) Info() core.Info { return p.info }

type bridgeState struct {
	On     bool    `json:"on"`
	PowerW float64 `json:"power_w"`
}

func (p *plug) State(ctx context.Context) (core.State, error) {
	var bs bridgeState
	if err := p.call(ctx, http.MethodGet, "state", &bs); err != nil {
		return core.State{Online: false, Error: err.Error(), UpdatedAt: time.Now()}, err
	}
	return core.State{Online: true, On: bs.On, PowerW: &bs.PowerW, UpdatedAt: time.Now()}, nil
}

func (p *plug) Execute(ctx context.Context, cmd core.Command) error {
	action := cmd.Action
	if action == "toggle" {
		st, err := p.State(ctx)
		if err != nil {
			return err
		}
		action = "on"
		if st.On {
			action = "off"
		}
	}
	if action != "on" && action != "off" {
		return fmt.Errorf("tapo: unsupported action %q", cmd.Action)
	}
	return p.call(ctx, http.MethodPost, action, nil)
}

func (p *plug) call(ctx context.Context, method, op string, out any) error {
	url := fmt.Sprintf("%s/%s/%s", p.bridge, p.host, op)
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return err
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("tapo bridge: %s", resp.Status)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
