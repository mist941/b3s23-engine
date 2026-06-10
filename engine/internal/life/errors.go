package life

import "errors"

var (
	ErrBadDimensions = errors.New("life: invalid grid dimensions")
	ErrBadWordCount = errors.New("life: packed word count does not match dimensions")
	ErrSizeMismatch = errors.New("life: grid dimensions mismatch")
)
