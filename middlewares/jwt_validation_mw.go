package middlewares

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/okieraised/go-common/api_response"
	"github.com/okieraised/go-common/cerrors"
	"github.com/opentracing/opentracing-go/log"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
)

// Claims defines the JWT payload we care about.
type Claims struct {
	PreferredUsername string   `json:"preferred_username"`
	Roles             []string `json:"roles"`
	jwt.RegisteredClaims
}

// JWTValidationMW validates Bearer JWT from header or cookie,
// sets user context (userID, subject, roles, permissions), and passes through.
func JWTValidationMW() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		tokenStr, bad := extractBearer(ctx)
		if bad != "" {
			unauthorized(ctx, cerrors.ErrMissingAuthenticationHeader, bad)
			return
		}

		// Parse+verify the token into typed claims
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(tok *jwt.Token) (any, error) {
			if _, ok := tok.Method.(*jwt.SigningMethodRSA); !ok {
				err := fmt.Errorf("unexpected signing method: %v", tok.Header["alg"])
				return nil, err
			}
			return keycloak.GetCert(), nil
		})
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
			unauthorized(ctx, cerrors.ErrGenericUnauthorized, "invalid token")
			return
		}

		// Validate claims (typed, no MapClaims casting)
		if err := validateAndAttach(ctx, claims, log); err != nil {
			log.Error("claims validation failed", zap.Error(err))
			ctx.AbortWithStatusJSON(http.StatusUnauthorized,
				api_response.NewResponse(ctx).ToResponse(
					cerrors.ErrCodeMapper[err],
					cerrors.ErrMessageMapper[err],
					nil, nil, nil,
				),
			)
			return
		}

		log.Info("jwt token is valid",
			zap.String("sub", ctx.GetString(constants.ContextValueSubject)),
			zap.String("uid", ctx.GetString(constants.ContextValueUserID)),
		)

		ctx.Set(constants.ContextValueAccessToken, tokenStr)
		ctx.Next()
	}
}

// extractBearer gets the bearer token from Authorization header or access-token cookie.
// Returns token string and an error message (empty if OK).
func extractBearer(ctx *gin.Context) (string, string) {
	authHeader := strings.TrimSpace(ctx.GetHeader(constants.AuthorizationHeader))
	if authHeader == "" {
		// Fallback to cookie
		accessTokenCookie, err := ctx.Cookie(constants.CookieAccessToken)
		if err != nil || strings.TrimSpace(accessTokenCookie) == "" {
			return "", "missing Authorization header or access token cookie"
		}
		return constants.AuthorizationTypeBearer + " " + accessTokenCookie, ""
	}

	typ, token, ok := splitAuthHeader(authHeader)
	if !ok || typ != constants.AuthorizationTypeBearer || token == "" {
		return "", "invalid Authorization header format"
	}
	return token, ""
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

func unauthorized(ctx *gin.Context, code error, msg string) {
	ctx.AbortWithStatusJSON(
		http.StatusUnauthorized,
		api_response.NewResponse(ctx).Populate(
			cerrors.ErrCodeMapper[code],
			msg,
			nil, nil, nil,
		),
	)
}

// validateAndAttach mirrors your original ValidateJWTClaims but uses typed claims.
func validateAndAttach(ctx *gin.Context, c *Claims, log *zap.Logger) error {
	// Exp (RegisteredClaims has time pointers; jwt.ParseWithClaims already checks exp by default,
	// but we keep your explicit check + custom error mapping)
	if c.ExpiresAt == nil {
		return cerrors.ErrInvalidExpirationClaim
	}
	if c.ExpiresAt.Time.Before(time.Now()) {
		return cerrors.ErrJWTTokenHasExpired
	}

	// Subject (user id)
	if c.Subject == "" {
		return cerrors.ErrInvalidUserIDClaim
	}
	if _, err := uuid.Parse(c.Subject); err != nil {
		return cerrors.ErrInvalidUserID
	}
	ctx.Set(constants.ContextValueUserID, c.Subject)

	// preferred_username
	if strings.TrimSpace(c.PreferredUsername) == "" {
		return cerrors.ErrInvalidSubjectClaim
	}
	ctx.Set(constants.ContextValueSubject, c.PreferredUsername)

	// roles
	if c.Roles == nil {
		return cerrors.ErrInvalidRoleNameClaim
	}
	effectiveRoles := make([]string, 0, len(c.Roles))
	for _, r := range c.Roles {
		if r == "" {
			return cerrors.ErrInvalidRoleNameClaimType
		}
		effectiveRoles = append(effectiveRoles, r)
	}
	ctx.Set(constants.ContextValueRoleNames, effectiveRoles)

	// Build permissions from Casbin policies
	effectivePolicies := make(map[string]struct{})
	permissions := make([]string, 0, 8)
	for _, role := range effectiveRoles {
		policies, err := casbin_adapter.GetInstance().FilterPolicy(role)
		if err != nil {
			return err
		}
		for _, p := range policies {
			// policy format assumed: [sub, obj, act, ...]
			if len(p) < 3 {
				continue
			}
			perm := p[1] + "." + p[2]
			if _, seen := effectivePolicies[perm]; !seen {
				effectivePolicies[perm] = struct{}{}
				permissions = append(permissions, perm)
			}
		}
	}
	ctx.Set(constants.ContextValuePermissions, permissions)

	log.Debug("claims attached",
		zap.String("user_id", c.Subject),
		zap.String("username", c.PreferredUsername),
		zap.Int("roles_count", len(effectiveRoles)),
		zap.Int("perm_count", len(permissions)),
	)
	return nil
}

// func JWTValidationMW() gin.HandlerFunc {
//	return func(ctx *gin.Context) {
//		logging.GetInstance().GetLogger().Info("validating jwt token")
//		authHeader := ctx.GetHeader(constants.AuthorizationHeader)
//		if authHeader == "" {
//			accessTokenCookie, err := ctx.Cookie(constants.CookieAccessToken)
//			if err != nil || accessTokenCookie == "" {
//				ctx.AbortWithStatusJSON(
//					http.StatusUnauthorized,
//					api_response.NewResponse(ctx).ToResponse(
//						cerrors.ErrCodeMapper[cerrors.ErrMissingAuthenticationHeader],
//						cerrors.ErrMessageMapper[cerrors.ErrMissingAuthenticationHeader],
//						nil,
//						nil,
//						nil,
//					),
//				)
//				return
//			}
//
//			authHeader = strings.Join([]string{
//				constants.AuthorizationTypeBearer,
//				accessTokenCookie,
//			}, " ")
//		}
//
//		authHdrPart := strings.Split(authHeader, " ")
//		switch len(authHdrPart) {
//		case 2:
//			if authHdrPart[0] != constants.AuthorizationTypeBearer {
//				ctx.AbortWithStatusJSON(
//					http.StatusUnauthorized,
//					api_response.NewResponse(ctx).ToResponse(
//						cerrors.ErrCodeMapper[cerrors.ErrInvalidAuthenticationHeader],
//						cerrors.ErrMessageMapper[cerrors.ErrInvalidAuthenticationHeader],
//						nil,
//						nil,
//						nil,
//					),
//				)
//				return
//			}
//		default:
//			logging.GetInstance().GetLogger().Info(fmt.Sprintf("authentication header contains [%d] parts", len(authHdrPart)))
//			ctx.AbortWithStatusJSON(
//				http.StatusUnauthorized,
//				api_response.NewResponse(ctx).ToResponse(
//					cerrors.ErrCodeMapper[cerrors.ErrInvalidAuthenticationHeader],
//					cerrors.ErrMessageMapper[cerrors.ErrInvalidAuthenticationHeader],
//					nil,
//					nil,
//					nil,
//				))
//			return
//		}
//
//		// Validate jwt token
//		// Parse jwt token
//		parsedToken, err := jwt.Parse(authHdrPart[1], func(token *jwt.Token) (interface{}, error) {
//			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
//				logging.GetInstance().GetLogger().Error(fmt.Sprintf("unexpected signing method: %v", token.Header["alg"]))
//				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
//			}
//			return keycloak.GetCert(), nil
//		})
//		if err != nil {
//			logging.GetInstance().GetLogger().Error(err.Error())
//			switch {
//			case errors.Is(err, jwt.ErrTokenMalformed):
//				ctx.AbortWithStatusJSON(
//					http.StatusUnauthorized,
//					api_response.NewResponse(ctx).ToResponse(
//						cerrors.ErrCodeMapper[cerrors.ErrGenericUnauthorized],
//						jwt.ErrTokenMalformed.Error(),
//						nil,
//						nil,
//						nil,
//					),
//				)
//				return
//			case errors.Is(err, jwt.ErrTokenSignatureInvalid):
//				ctx.AbortWithStatusJSON(
//					http.StatusUnauthorized,
//					api_response.NewResponse(ctx).ToResponse(
//						cerrors.ErrCodeMapper[cerrors.ErrGenericUnauthorized],
//						jwt.ErrTokenSignatureInvalid.Error(),
//						nil,
//						nil,
//						nil,
//					),
//				)
//				return
//			case errors.Is(err, jwt.ErrTokenExpired):
//				ctx.AbortWithStatusJSON(
//					http.StatusUnauthorized,
//					api_response.NewResponse(ctx).ToResponse(
//						cerrors.ErrCodeMapper[cerrors.ErrGenericUnauthorized],
//						jwt.ErrTokenExpired.Error(),
//						nil,
//						nil,
//						nil,
//					),
//				)
//				return
//			case errors.Is(err, jwt.ErrTokenNotValidYet):
//				ctx.AbortWithStatusJSON(
//					http.StatusUnauthorized,
//					api_response.NewResponse(ctx).ToResponse(
//						cerrors.ErrCodeMapper[cerrors.ErrGenericUnauthorized],
//						jwt.ErrTokenNotValidYet.Error(),
//						nil,
//						nil,
//						nil,
//					),
//				)
//				return
//			default:
//				ctx.AbortWithStatusJSON(
//					http.StatusUnauthorized,
//					api_response.NewResponse(ctx).ToResponse(
//						cerrors.ErrCodeMapper[cerrors.ErrGenericUnauthorized],
//						err.Error(),
//						nil,
//						nil,
//						nil,
//					),
//				)
//			}
//		}
//
//		err = ValidateJWTClaims(ctx, parsedToken)
//		if err != nil {
//			logging.GetInstance().GetLogger().Error(err.Error())
//			ctx.AbortWithStatusJSON(
//				http.StatusUnauthorized,
//				api_response.NewResponse(ctx).ToResponse(
//					cerrors.ErrCodeMapper[err],
//					cerrors.ErrMessageMapper[err],
//					nil,
//					nil,
//					nil,
//				),
//			)
//			return
//		}
//
//		logging.GetInstance().GetLogger().Info(fmt.Sprintf("jwt token is valid for subject [%s]", ctx.GetString(constants.ContextValueSubject)))
//
//		ctx.Set(constants.ContextValueAccessToken, authHdrPart[1])
//		ctx.Next()
//		return
//	}
//}
//
//func ValidateJWTClaims(ctx *gin.Context, parsedToken *jwt.Token) error {
//	// Check exp claim
//	exp, err := parsedToken.Claims.GetExpirationTime()
//	if err != nil {
//		return cerrors.ErrInvalidExpirationClaim
//	}
//
//	// Validate exp claim
//	if exp.Before(time.Now()) {
//		return cerrors.ErrJWTTokenHasExpired
//	}
//
//	// Validate subject claim
//	userIDStr, err := parsedToken.Claims.GetSubject()
//	if err != nil {
//		return cerrors.ErrInvalidUserIDClaim
//	}
//
//	_, err = uuid.Parse(userIDStr)
//	if err != nil {
//		return cerrors.ErrInvalidUserID
//	}
//
//	ctx.Set(constants.ContextValueUserID, userIDStr)
//
//	// Validate preferred_username claim
//	username, ok := parsedToken.Claims.(jwt.MapClaims)["preferred_username"]
//	if !ok {
//		return cerrors.ErrInvalidSubjectClaim
//	}
//	usernameStr, ok := username.(string)
//	if !ok {
//		return cerrors.ErrInvalidSubjectClaim
//	}
//	ctx.Set(constants.ContextValueSubject, usernameStr)
//
//	// Validate role claim
//	rolesClaims, ok := parsedToken.Claims.(jwt.MapClaims)["roles"]
//	if !ok {
//		return cerrors.ErrInvalidRoleNameClaim
//	}
//
//	roles, ok := rolesClaims.([]interface{})
//	if !ok {
//		return cerrors.ErrInvalidRolesClaimType
//	}
//	effectiveRoles := make([]string, 0)
//	for _, role := range roles {
//		roleName, ok := role.(string)
//		if !ok {
//			return cerrors.ErrInvalidRoleNameClaimType
//		}
//		effectiveRoles = append(effectiveRoles, roleName)
//	}
//	ctx.Set(constants.ContextValueRoleNames, effectiveRoles)
//
//	effectivePolicies := make(map[string]struct{})
//	permissions := make([]string, 0, len(effectivePolicies))
//	for _, role := range effectiveRoles {
//		policies, err := casbin_adapter.GetInstance().FilterPolicy(role)
//		if err != nil {
//			return err
//		}
//		for _, policy := range policies {
//			perm := strings.Join([]string{policy[1], policy[2]}, ".")
//			if _, ok := effectivePolicies[perm]; !ok {
//				effectivePolicies[perm] = struct{}{}
//				permissions = append(permissions, perm)
//			}
//		}
//	}
//
//	ctx.Set(constants.ContextValuePermissions, permissions)
//	return nil
//}
