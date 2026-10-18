package manager

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"smarthub/internal/core"
	"smarthub/internal/store"
)

var ErrNotFound = errors.New("device not found")

// View is what the API sends: static info plus live state.
type View struct {
	core.Info
	State core.State `json:"state"`
}

type entry struct {
	dev   core.Device
	info  core.Info
	state core.State
}

type Manager struct {
	store    *store.Store
	interval time.Duration

	mu      sync.RWMutex
	order   []string
	entries map[string]*entry
	subs    map[chan View]struct{}
}

func New(devs []core.Device, st *store.Store, interval time.Duration) *Manager {
	m := &Manager{
		store:    st,
		interval: interval,
		entries:  map[string]*entry{},
		subs:     map[chan View]struct{}{},
	}
	for _, d := range devs {
		info := d.Info()
		m.order = append(m.order, info.ID)
		m.entries[info.ID] = &entry{dev: d, info: info}
	}
	return m
}

// Run polls every device in its own goroutine until ctx is cancelled.
func (m *Manager) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, id := range m.order {
		wg.Add(1)
		go func(d core.Device) {
			defer wg.Done()
			m.loop(ctx, d)
		}(m.entries[id].dev)
	}
	wg.Wait()
}

func (m *Manager) loop(ctx context.Context, d core.Device) {
	t := time.NewTicker(m.interval)
	defer t.Stop()
	for {
		m.poll(ctx, d)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (m *Manager) poll(ctx context.Context, d core.Device) {
	pctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	id := d.Info().ID
	st, err := d.State(pctx)
	if err != nil {
		st = core.State{Online: false, Error: err.Error(), UpdatedAt: time.Now()}
	}
	m.update(id, st)

	if st.Online && st.PowerW != nil {
		if err := m.store.RecordPower(id, time.Now().Unix(), *st.PowerW, int64(m.interval.Seconds())); err != nil {
			log.Printf("record power %s: %v", id, err)
		}
	}
}

func (m *Manager) update(id string, st core.State) {
	m.mu.Lock()
	e := m.entries[id]
	e.state = st
	v := View{Info: e.info, State: st}
	m.mu.Unlock()
	m.publish(v)
}

func (m *Manager) publish(v View) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for ch := range m.subs {
		select {
		case ch <- v:
		default:
		}
	}
}

func (m *Manager) Subscribe() chan View {
	ch := make(chan View, 32)
	m.mu.Lock()
	m.subs[ch] = struct{}{}
	m.mu.Unlock()
	return ch
}

func (m *Manager) Unsubscribe(ch chan View) {
	m.mu.Lock()
	delete(m.subs, ch)
	m.mu.Unlock()
}

func (m *Manager) Snapshot() []View {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]View, 0, len(m.order))
	for _, id := range m.order {
		e := m.entries[id]
		out = append(out, View{Info: e.info, State: e.state})
	}
	return out
}

func (m *Manager) Execute(ctx context.Context, id string, cmd core.Command) error {
	m.mu.RLock()
	e, ok := m.entries[id]
	m.mu.RUnlock()
	if !ok {
		return ErrNotFound
	}
	if err := e.dev.Execute(ctx, cmd); err != nil {
		return err
	}
	m.poll(ctx, e.dev)
	return nil
}
