package sql

import "errors"

var (
	ErrUnsupportedDialect = errors.New("unsupported SQL dialect")
)
