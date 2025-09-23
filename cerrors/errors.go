package cerrors

import "errors"

var (
	ErrGenericBadRequest      = errors.New("bad request error")
	ErrGenericInternalServer  = errors.New("internal server error")
	ErrGenericRequestTimedOut = errors.New("request timeout error")
	ErrGenericUnauthorized    = errors.New("unauthorized request error")
	ErrGenericPermission      = errors.New("invalid permission error")
	ErrGenericUnknownAPIPath  = errors.New("unknown api path")
)

var ErrCodeMapper = map[error]string{
	nil: "OK",
}

var ErrMessageMapper = map[error]string{
	nil: "OK",
}
