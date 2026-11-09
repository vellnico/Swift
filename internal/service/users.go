package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/vellnico/swift/internal/auth"
	"github.com/vellnico/swift/internal/store"
)

// Users manages people who get access; xray is updated without restarts.
type Users struct {
	Store *store.Store
	Xray  *Xray
}

func newCredentials(u *store.User) {
	u.SubToken = auth.RandomToken(24)
	u.VlessUUID = uuid.NewString()
	u.Hy2Password = auth.RandomToken(24)
}

func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n == 0 || n > 64 {
		return name, &ValidationError{Fields: map[string]string{"name": "required"}}
	}
	return name, nil
}

func dupName(err error) error {
	if errors.Is(err, store.ErrDuplicate) {
		return &ValidationError{Fields: map[string]string{"name": "duplicate"}}
	}
	return err
}

func (s *Users) Create(ctx context.Context, name, note string) (store.User, error) {
	name, err := validName(name)
	if err != nil {
		return store.User{}, err
	}
	u := store.User{Name: name, Note: strings.TrimSpace(note), Enabled: true}
	newCredentials(&u)
	u, err = s.Store.CreateUser(ctx, u)
	if err != nil {
		return u, dupName(err)
	}
	if err := s.Xray.AddUser(ctx, u); err != nil {
		_ = s.Store.DeleteUser(context.WithoutCancel(ctx), u.ID)
		return u, err
	}
	return u, nil
}

// UserUpdate holds optional changes; nil fields stay as they are.
type UserUpdate struct {
	Name    *string `json:"name"`
	Note    *string `json:"note"`
	Enabled *bool   `json:"enabled"`
}

func (s *Users) Update(ctx context.Context, id int64, upd UserUpdate) (store.User, error) {
	old, err := s.Store.User(ctx, id)
	if err != nil {
		return old, err
	}
	u := old
	if upd.Name != nil {
		if u.Name, err = validName(*upd.Name); err != nil {
			return old, err
		}
	}
	if upd.Note != nil {
		u.Note = strings.TrimSpace(*upd.Note)
	}
	if upd.Enabled != nil {
		u.Enabled = *upd.Enabled
	}
	if err := s.Store.UpdateUser(ctx, u); err != nil {
		return old, dupName(err)
	}
	if u.Enabled != old.Enabled {
		if u.Enabled {
			err = s.Xray.AddUser(ctx, u)
		} else {
			err = s.Xray.RemoveUser(ctx, old)
		}
		if err != nil {
			_ = s.Store.UpdateUser(context.WithoutCancel(ctx), old)
			return old, err
		}
	}
	return u, nil
}

// Reissue gives the user a new subscription link and new credentials; the old link stops working.
func (s *Users) Reissue(ctx context.Context, id int64) (store.User, error) {
	old, err := s.Store.User(ctx, id)
	if err != nil {
		return old, err
	}
	u := old
	newCredentials(&u)
	if err := s.Store.UpdateUser(ctx, u); err != nil {
		return old, err
	}
	if u.Enabled {
		// same email in xray, so remove the old account before adding the new one
		if err := s.Xray.RemoveUser(ctx, old); err != nil {
			return u, err
		}
		if err := s.Xray.AddUser(ctx, u); err != nil {
			return u, err
		}
	}
	return u, nil
}

func (s *Users) Delete(ctx context.Context, id int64) (store.User, error) {
	u, err := s.Store.User(ctx, id)
	if err != nil {
		return u, err
	}
	if err := s.Store.DeleteUser(ctx, id); err != nil {
		return u, err
	}
	return u, s.Xray.RemoveUser(ctx, u)
}
