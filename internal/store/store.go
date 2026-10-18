package store

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

type Point struct {
	T int64   `json:"t"`
	W float64 `json:"w"`
}

type Energy struct {
	DeviceID string
	KWh      float64
	AvgW     float64
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
		CREATE INDEX IF NOT EXISTS idx_power_dev_ts ON power_samples(device_id, ts);`); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) RecordPower(id string, ts int64, watts float64) error {
	_, err := s.db.Exec(`INSERT INTO power_samples(device_id, ts, watts) VALUES (?, ?, ?)`, id, ts, watts)
	return err
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

// Energy integrates samples into kWh. pollSeconds is the sampling interval.
func (s *Store) Energy(since int64, pollSeconds float64) ([]Energy, error) {
	rows, err := s.db.Query(`
		SELECT device_id, SUM(watts), AVG(watts), COUNT(*)
		FROM power_samples WHERE ts >= ?
		GROUP BY device_id ORDER BY SUM(watts) DESC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Energy{}
	for rows.Next() {
		var e Energy
		var sumW float64
		if err := rows.Scan(&e.DeviceID, &sumW, &e.AvgW, &e.Samples); err != nil {
			return nil, err
		}
		e.KWh = sumW * pollSeconds / 3.6e6
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) Prune(before int64) error {
	_, err := s.db.Exec(`DELETE FROM power_samples WHERE ts < ?`, before)
	return err
}
