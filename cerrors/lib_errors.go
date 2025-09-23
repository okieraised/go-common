package cerrors

import "errors"

var (
	ErrRequiredConnectionParamsAreEmpty = errors.New("postgres: required connection parameters are empty")
)

var (
	ErrQueueClosed      = errors.New("queue closed")
	ErrQueueFullDropped = errors.New("queue full, dropped")
)

var (
	ErrNoFormatMatched = errors.New("no datetime format matched")
)
