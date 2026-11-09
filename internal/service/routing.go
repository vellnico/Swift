package service

import (
	"context"
	"errors"

	"github.com/vellnico/swift/internal/routing"
	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/sub"
)

// Preset returns the active split-tunnelling preset; "ru_direct" unless switched off.
func Preset(ctx context.Context, st *store.Store) (string, error) {
	v, err := st.Setting(ctx, "routing_preset")
	if errors.Is(err, store.ErrNotFound) || v == "" {
		return routing.PresetRUDirect, nil
	}
	return v, err
}

func SetPreset(ctx context.Context, st *store.Store, preset string) error {
	if preset != routing.PresetRUDirect && preset != routing.PresetNone {
		return &ValidationError{Fields: map[string]string{"preset": "invalid"}}
	}
	return st.SetSetting(ctx, "routing_preset", preset)
}

// RoutingFor adapts the preset to the subscription renderer (nil = all via VPN).
func RoutingFor(st *store.Store) func(ctx context.Context) (sub.Routing, error) {
	return func(ctx context.Context) (sub.Routing, error) {
		p, err := Preset(ctx, st)
		if err != nil || p != routing.PresetRUDirect {
			return nil, err
		}
		return routing.RUDirect{}, nil
	}
}
