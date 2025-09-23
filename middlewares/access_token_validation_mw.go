package middlewares

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/okieraised/go-common/api_response"
	"github.com/okieraised/go-common/cerrors"
	"github.com/okieraised/go-common/constants"
	"github.com/okieraised/go-common/infrastructures/logger"
	"go.uber.org/zap"
)

// Claims defines the JWT payload we care about.
type Claims struct {
	PreferredUsername string   `json:"preferred_username"`
	Roles             []string `json:"roles"`
	jwt.RegisteredClaims
}

// AccessTokenValidationMW validates the Authorization: Bearer <JWT> header
// using any of the provided algorithms/keys (e.g. {"RS256": rsaPub, "HS256": []byte("secret")}).
func AccessTokenValidationMW(keys map[string]any) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		tokenStr, appErr := extractBearer(ctx)
		if appErr != nil {
			ctx.AbortWithStatusJSON(
				appErr.HTTPStatus,
				api_response.New[any](ctx).Populate(
					appErr.Code, appErr.Message, nil, nil, nil,
				),
			)
			return
		}

		claims := &Claims{}
		token, err := ParseJWTWithAlgs(tokenStr, claims, keys)
		if err != nil {
			switch {
			case errors.Is(err, jwt.ErrTokenMalformed):
				unauthorized(ctx, cerrors.ErrGenericUnauthorized, jwt.ErrTokenMalformed.Error())
			case errors.Is(err, jwt.ErrTokenSignatureInvalid):
				unauthorized(ctx, cerrors.ErrGenericUnauthorized, jwt.ErrTokenSignatureInvalid.Error())
			case errors.Is(err, jwt.ErrTokenExpired):
				unauthorized(ctx, cerrors.ErrGenericUnauthorized, jwt.ErrTokenExpired.Error())
			case errors.Is(err, jwt.ErrTokenNotValidYet):
				unauthorized(ctx, cerrors.ErrGenericUnauthorized, jwt.ErrTokenNotValidYet.Error())
			default:
				unauthorized(ctx, cerrors.ErrGenericUnauthorized, err.Error())
			}
			return
		}
		if !token.Valid {
			unauthorized(ctx, cerrors.ErrGenericUnauthorized, "invalid access token")
			return
		}

		if appErr = validateAndAttach(ctx, claims); appErr != nil {
			ctx.AbortWithStatusJSON(
				appErr.HTTPStatus,
				api_response.New[any](ctx).Populate(
					appErr.Code, appErr.Message, nil, nil, nil,
				),
			)
			return
		}

		logger.GetDefault().Info("access token is valid",
			zap.String("sub", ctx.GetString(constants.ContextFieldUsername)),
		)
		ctx.Set(constants.ContextFieldAccessToken, tokenStr)
		ctx.Next()
	}
}

func ParseJWTWithAlgs(tokenStr string, claims jwt.Claims, keys map[string]any) (*jwt.Token, error) {
	if len(keys) == 0 {
		return nil, errors.New("no algorithms/keys configured")
	}

	// Build the allowed methods list from the provided keys.
	allowed := make([]string, 0, len(keys))
	for alg := range keys {
		allowed = append(allowed, alg)
	}

	keyfunc := func(tok *jwt.Token) (any, error) {
		alg, _ := tok.Header["alg"].(string)
		if alg == "" {
			return nil, errors.New("missing alg header")
		}
		// Double check jwt library's parsed method matches header.
		if m := tok.Method.Alg(); m != alg {
			return nil, fmt.Errorf("alg mismatch: header=%s method=%s", alg, m)
		}
		key, ok := keys[alg]
		if !ok {
			return nil, fmt.Errorf("unsupported signing method: %s", alg)
		}
		return key, nil
	}
	return jwt.ParseWithClaims(tokenStr, claims, keyfunc, jwt.WithValidMethods(allowed))
}

// extractBearer gets the bearer token from Authorization header or access-token cookie.
// Returns token string and an error message (empty if OK).
func extractBearer(ctx *gin.Context) (string, *cerrors.AppError) {
	authHeader := strings.TrimSpace(ctx.GetHeader(constants.HeaderAuthorization))
	if authHeader == "" {
		return "", cerrors.ErrMissingAuthenticationHeader
	}

	typ, token, ok := splitAuthHeader(authHeader)
	if !ok || typ != constants.AuthorizationTypeBearer || token == "" {
		return "", cerrors.ErrInvalidAuthenticationHeader
	}
	return token, nil
}

// splitAuthHeader parses "Type token" into type and token.
func splitAuthHeader(h string) (typ, token string, ok bool) {
	h = strings.TrimSpace(h)
	parts := strings.Fields(h)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func unauthorized(ctx *gin.Context, appErr *cerrors.AppError, msg string) {
	ctx.AbortWithStatusJSON(
		http.StatusUnauthorized,
		api_response.New[any](ctx).Populate(
			appErr.Code,
			msg,
			nil, nil, nil,
		),
	)
}

// validateAndAttach mirrors your original ValidateJWTClaims but uses typed claims.
func validateAndAttach(ctx *gin.Context, c *Claims) *cerrors.AppError {
	if c.ExpiresAt == nil {
		return cerrors.ErrInvalidExpirationClaim
	}
	if c.ExpiresAt.Time.Before(time.Now()) {
		return cerrors.ErrAccessTokenHasExpired
	}

	// Subject (user id)
	if c.Subject == "" {
		return cerrors.ErrInvalidUserIDClaim
	}
	ctx.Set(constants.ContextFieldUserID, c.Subject)

	// preferred_username
	if strings.TrimSpace(c.PreferredUsername) == "" {
		return cerrors.ErrInvalidSubjectClaim
	}
	ctx.Set(constants.ContextFieldUsername, c.PreferredUsername)

	// roles
	if c.Roles == nil {
		return cerrors.ErrInvalidRolesClaim
	}
	effectiveRoles := make([]string, 0, len(c.Roles))
	for _, r := range c.Roles {
		if r == "" {
			continue
		}
		effectiveRoles = append(effectiveRoles, r)
	}
	ctx.Set(constants.ContextFieldRoleNames, effectiveRoles)

	//
	//// Build permissions from Casbin policies
	//effectivePolicies := make(map[string]struct{})
	//permissions := make([]string, 0, 8)
	//for _, role := range effectiveRoles {
	//	policies, err := casbin_adapter.GetInstance().FilterPolicy(role)
	//	if err != nil {
	//		return err
	//	}
	//	for _, p := range policies {
	//		// policy format assumed: [sub, obj, act, ...]
	//		if len(p) < 3 {
	//			continue
	//		}
	//		perm := p[1] + "." + p[2]
	//		if _, seen := effectivePolicies[perm]; !seen {
	//			effectivePolicies[perm] = struct{}{}
	//			permissions = append(permissions, perm)
	//		}
	//	}
	//}
	//ctx.Set(constants.ContextValuePermissions, permissions)
	//
	return nil
}
