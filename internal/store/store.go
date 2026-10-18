package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

const (
	// RawRetentionDays keeps per-poll samples for charts and tip detection.
	RawRetentionDays = 365
	// HourlyRetentionYears keeps hourly energy totals used for bills and history.
	HourlyRetentionYears = 10
)

type Store struct{ db *sql.DB }

type Point struct {
	T int64   `json:"t"`
	W float64 `json:"w"`
}

// Sample is one raw reading.
type Sample struct {
	T int64
	W float64
}

// DeviceEnergy is energy summed over a time window for one device.
type DeviceEnergy struct {
	DeviceID string
	Wh       float64
	Samples  int64
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS power_samples (
			device_id TEXT    NOT NULL,
			ts        INTEGER NOT NULL,
			watts     REAL    NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_power_dev_ts ON power_samples(device_id, ts);
		CREATE TABLE IF NOT EXISTS energy_hourly (
			device_id TEXT    NOT NULL,
			hour      INTEGER NOT NULL,
			wh        REAL    NOT NULL DEFAULT 0,
			samples   INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (device_id, hour)
		);`); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// RecordPower stores one raw sample and adds its energy to the hourly total.
func (s *Store) RecordPower(id string, ts int64, watts float64, intervalSec int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO power_samples(device_id, ts, watts) VALUES (?, ?, ?)`, id, ts, watts); err != nil {
		return err
	}
	hour := ts - ts%3600
	wh := watts * float64(intervalSec) / 3600
	if _, err := tx.Exec(`
		INSERT INTO energy_hourly(device_id, hour, wh, samples) VALUES (?, ?, ?, 1)
		ON CONFLICT(device_id, hour) DO UPDATE SET wh = wh + excluded.wh, samples = samples + 1`,
		id, hour, wh); err != nil {
		return err
	}
	return tx.Commit()
}

// Series returns average watts per bucket (bucket is in seconds).
func (s *Store) Series(id string, since, until, bucket int64) ([]Point, error) {
	rows, err := s.db.Query(`
		SELECT (ts / ?) * ? AS b, AVG(watts)
		FROM power_samples
		WHERE device_id = ? AND ts BETWEEN ? AND ?
		GROUP BY b ORDER BY b`, bucket, bucket, id, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pts := []Point{}
	for rows.Next() {
		var p Point
		if err := rows.Scan(&p.T, &p.W); err != nil {
			return nil, err
		}
		pts = append(pts, p)
	}
	return pts, rows.Err()
}

// EnergyBetween sums hourly energy per device for hours in [from, to).
func (s *Store) EnergyBetween(from, to int64) ([]DeviceEnergy, error) {
	rows, err := s.db.Query(`
		SELECT device_id, SUM(wh), SUM(samples)
		FROM energy_hourly
		WHERE hour >= ? AND hour < ?
		GROUP BY device_id ORDER BY SUM(wh) DESC`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []DeviceEnergy{}
	for rows.Next() {
		var e DeviceEnergy
		if err := rows.Scan(&e.DeviceID, &e.Wh, &e.Samples); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Prune applies the retention policy: raw samples older than RawRetentionDays
// and hourly totals older than HourlyRetentionYears are removed.
func (s *Store) Prune(now time.Time) error {
	rawCut := now.AddDate(0, 0, -RawRetentionDays).Unix()
	hourCut := now.AddDate(-HourlyRetentionYears, 0, 0).Unix()
	if _, err := s.db.Exec(`DELETE FROM power_samples WHERE ts < ?`, rawCut); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM energy_hourly WHERE hour < ?`, hourCut)
	return err
}

// DeleteAll erases every stored sample and total. Device configuration is not touched.
func (s *Store) DeleteAll() error {
	if _, err := s.db.Exec(`DELETE FROM power_samples`); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM energy_hourly`)
	return err
}

// DeleteDevice erases all stored history for one device.
func (s *Store) DeleteDevice(id string) error {
	if _, err := s.db.Exec(`DELETE FROM power_samples WHERE device_id = ?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM energy_hourly WHERE device_id = ?`, id)
	return err
}

// Samples returns raw readings for one device in [since, until], oldest first.
func (s *Store) Samples(id string, since, until int64) ([]Sample, error) {
	rows, err := s.db.Query(`
		SELECT ts, watts FROM power_samples
		WHERE device_id = ? AND ts BETWEEN ? AND ?
		ORDER BY ts`, id, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Sample{}
	for rows.Next() {
		var x Sample
		if err := rows.Scan(&x.T, &x.W); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
