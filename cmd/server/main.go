//go:generate go run ../../tools/genplugins

package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"smarthub/internal/api"
	"smarthub/internal/core"
	"smarthub/internal/manager"
	"smarthub/internal/store"
)

func main() {
	addr := getenv("ADDR", ":8080")
	dbPath := getenv("DB_PATH", "smarthub.db")
	cfgPath := getenv("CONFIG", "../../devices.json")
	poll := 15 * time.Second

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		log.Fatalf("read config: %v", err)
	}
	var cfg struct {
		Devices []core.Spec `json:"devices"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		log.Fatalf("parse config: %v", err)
	}

	onW := map[string]float64{}
	for _, s := range cfg.Devices {
		if s.OnWatts > 0 {
			onW[s.ID] = s.OnWatts
		}
	}

	var devs []core.Device
	for _, spec := range cfg.Devices {
		d, err := core.Build(spec)
		if err != nil {
			log.Printf("skipping %s: %v", spec.ID, err)
			continue
		}
		devs = append(devs, d)
	}

	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	mgr := manager.New(devs, st, poll)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go mgr.Run(ctx)
	go retentionLoop(ctx, st)

	srv := &http.Server{
		Addr:    addr,
		Handler: api.New(mgr, st, api.Options{Poll: poll, OnWatts: onW}),
	}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	log.Printf("SmartHub on %s with %d devices (plugins: %v)", addr, len(devs), core.Kinds())
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// retentionLoop enforces the retention policy on startup and then hourly.
func retentionLoop(ctx context.Context, st *store.Store) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if err := st.Prune(time.Now()); err != nil {
			log.Printf("retention: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
