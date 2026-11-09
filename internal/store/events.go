package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const maxEvents = 1000

type Event struct {
	ID     int64          `json:"id"`
	TS     time.Time      `json:"ts"`
	Level  string         `json:"level"`
	Source string         `json:"source"`
	Key    string         `json:"key"`
	Params map[string]any `json:"params"`
}

// AddEvent appends to the journal and keeps only the newest 1000 rows.
// Params must never contain secrets: they are shown in the UI.
func (s *Store) AddEvent(ctx context.Context, level, source, key string, params map[string]any) (Event, error) {
	if params == nil {
		params = map[string]any{}
	}
	body, err := json.Marshal(params)
	if err != nil {
		return Event{}, fmt.Errorf("encode event params: %w", err)
	}
	ev := Event{TS: s.Now(), Level: level, Source: source, Key: key, Params: params}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO events(ts, level, source, message_key, params_json) VALUES (?, ?, ?, ?, ?)`,
		ev.TS.UnixMilli(), level, source, key, string(body))
	if err != nil {
		return Event{}, fmt.Errorf("add event: %w", err)
	}
	ev.ID, _ = res.LastInsertId()
	if ev.ID%50 == 0 {
		if _, err := s.DB.ExecContext(ctx, `DELETE FROM events WHERE id <= ?`, ev.ID-maxEvents); err != nil {
			return ev, fmt.Errorf("trim events: %w", err)
		}
	}
	return ev, nil
}

// Events returns up to limit events older than beforeID (0 = newest), newest first.
func (s *Store) Events(ctx context.Context, beforeID int64, limit int, level string) ([]Event, error) {
	if limit <= 0 || limit > maxEvents {
		limit = 100
	}
	q := `SELECT id, ts, level, source, message_key, params_json FROM events WHERE (? = 0 OR id < ?) AND (? = '' OR level = ?) ORDER BY id DESC LIMIT ?`
	rows, err := s.DB.QueryContext(ctx, q, beforeID, beforeID, level, level, limit)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var ev Event
		var ts int64
		var params string
		if err := rows.Scan(&ev.ID, &ts, &ev.Level, &ev.Source, &ev.Key, &params); err != nil {
			return nil, err
		}
		ev.TS = time.UnixMilli(ts)
		if err := json.Unmarshal([]byte(params), &ev.Params); err != nil {
			ev.Params = map[string]any{}
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}
