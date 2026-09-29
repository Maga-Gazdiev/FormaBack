package model

import "errors"

type Route struct {
	From   string   `json:"from"`
	To     []string `json:"to"`
	Group  string   `json:"group"`
	Engine string   `json:"-"`
}

var (
	ErrUnsupported = errors.New("unsupported conversion")
	ErrTooLarge    = errors.New("file too large")
	ErrInvalid     = errors.New("invalid file")
	ErrConversion  = errors.New("conversion failed")
)
