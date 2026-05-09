package domain

import "errors"

var (
	ErrNotFound        = errors.New("server not found")
	ErrExists          = errors.New("server already exists")
	ErrInvalidName     = errors.New("invalid server name")
	ErrInvalidPort     = errors.New("invalid port number")
	ErrEmptyField      = errors.New("empty field not allowed")
	ErrVersionNotFound = errors.New("version not found in changelog")
	ErrInvalidVersion  = errors.New("invalid version number")
)