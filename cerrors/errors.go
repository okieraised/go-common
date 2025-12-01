package cerrors

import (
	"errors"
	"fmt"
	"net/http"
)

type AppError struct {
	Code       string
	Message    string
	HTTPStatus int
	Cause      error
}

func (e *AppError) Error() string {
	if e == nil {
		return "OK"
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func (e *AppError) Unwrap() error { return e.Cause }

func (e *AppError) WithCause(err error) error {
	if e == nil {
		return nil
	}
	c := *e
	c.Cause = err
	return &c
}

func (e *AppError) WithMessage(msg string, a ...any) error {
	if e == nil {
		return nil
	}
	c := *e
	if len(a) > 0 {
		c.Message = fmt.Sprintf(msg, a...)
	} else {
		c.Message = msg
	}
	return &c
}

func Sanitize(err error) error {
	if err == nil {
		return nil
	}
	var ae *AppError
	if errors.As(err, &ae) && ae == nil {
		return nil
	}
	return err
}

// CodeOf returns the code if err is *AppError; "UNKNOWN" otherwise; "OK" for nil.
func CodeOf(err error) string {
	switch e := err.(type) {
	case nil:
		return "OK"
	case *AppError:
		return e.Code
	default:
		return "UNKNOWN"
	}
}

// MessageOf returns the message if err is *AppError; err.Error() otherwise; "OK" for nil.
func MessageOf(err error) string {
	switch e := err.(type) {
	case nil:
		return "OK"
	case *AppError:
		if e.Message != "" {
			return e.Message
		}
		return e.Code
	default:
		return e.Error()
	}
}

// HTTPStatusOf returns the HTTP status if err is *AppError; otherwise 500; 200 for nil.
func HTTPStatusOf(err error) int {
	switch e := err.(type) {
	case nil:
		return http.StatusOK
	case *AppError:
		if e.HTTPStatus != 0 {
			return e.HTTPStatus
		}
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// IsCode reports whether err is an *AppError with the given code.
func IsCode(err error, code string) bool {
	var e *AppError
	if errors.As(err, &e) {
		return e.Code == code
	}
	return false
}

// def is a small constructor for sentinels.
func def(code, msg string, httpStatus int) *AppError {
	return &AppError{Code: code, Message: msg, HTTPStatus: httpStatus}
}

var (
	OK = def("OK", "OK", http.StatusOK)
)

var (
	ErrGenericBadRequest      = def("400000", "bad request error", http.StatusBadRequest)
	ErrGenericUnauthorized    = def("400001", "unauthorized request error", http.StatusUnauthorized)
	ErrGenericPermission      = def("400003", "invalid permission error", http.StatusForbidden)
	ErrGenericUnknownAPIPath  = def("400004", "unknown api path", http.StatusNotFound)
	ErrGenericInternalServer  = def("500000", "internal server error", http.StatusInternalServerError)
	ErrGenericRequestTimedOut = def("500004", "request timeout error", http.StatusGatewayTimeout)
	ErrInvalidDatabaseClient  = def("500005", "invalid database client", http.StatusInternalServerError)
)

var (
	ErrCallbackStateParamIsMissing    = def("410000", "callback state param is missing", http.StatusBadRequest)
	ErrMismatchedCallbackStateParam   = def("410000", "mismatched callback state param", http.StatusBadRequest)
	ErrFailedToGetStoredStateParam    = def("410000", "failed to get stored state", http.StatusBadRequest)
	ErrAuthorizationCodeIsMissing     = def("410000", "authorization code is missing", http.StatusBadRequest)
	ErrFailedToVerifyIDToken          = def("410000", "failed to verify ID token", http.StatusBadRequest)
	ErrCannotFindIDTokenInOAuth2Token = def("410000", "cannot find ID token in OAuth2 token", http.StatusBadRequest)
	ErrFailedToGetUserInfoClaims      = def("410000", "failed to get user info claims", http.StatusBadRequest)
	ErrInvalidCallbackStateParam      = def("410000", "invalid callback state param", http.StatusBadRequest)
	ErrFailedToCheckUserIdentity      = def("410000", "failed to check user identity", http.StatusInternalServerError)
)

var (
	ErrMissingAuthenticationHeader = def("410000", "missing authentication header", http.StatusUnauthorized)
	ErrInvalidAuthenticationHeader = def("410001", "invalid authentication header", http.StatusUnauthorized)
	ErrInvalidExpirationClaim      = def("410002", "invalid expiration claim", http.StatusUnauthorized)
	ErrAccessTokenHasExpired       = def("410003", "access token has expired", http.StatusUnauthorized)
	ErrInvalidSubjectClaim         = def("410004", "invalid access token's subject claim", http.StatusUnauthorized)
	ErrInvalidUserIDClaim          = def("410005", "invalid access token's user id claim", http.StatusUnauthorized)
	ErrInvalidRolesClaim           = def("410006", "invalid access token's roles claim", http.StatusUnauthorized)
	ErrInvalidPermissionsClaim     = def("410007", "invalid access token's permissions claim", http.StatusUnauthorized)
	ErrInvalidTokenSignature       = def("410008", "invalid token signature", http.StatusUnauthorized)
	ErrInvalidRolesClaimType       = def("410009", "invalid roles claim type", http.StatusUnauthorized)
	ErrInvalidRoleNameClaimType    = def("4100010", "invalid role name claim type", http.StatusUnauthorized)
	ErrRefreshTokenIsMissing       = def("4100011", "refresh token is missing", http.StatusBadRequest)
	ErrInvalidRefreshToken         = def("4100012", "invalid refresh token", http.StatusUnauthorized)
	ErrRefreshTokenIsRevoked       = def("4100013", "refresh token is revoked", http.StatusUnauthorized)
)
