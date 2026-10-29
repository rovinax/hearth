package store

import "errors"

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrConflict  = errors.New("conflict")
	ErrLastAdmin = errors.New("last admin")
	ErrInvalid   = errors.New("invalid")
)

const (
	RoleAdmin  = "admin"
	RoleUser   = "user"
	VisPrivate = "private"
	VisShared  = "shared"
	KindImage  = "image"
	KindVideo  = "video"
	KindAudio  = "audio"
	KindOther  = "other"
)
