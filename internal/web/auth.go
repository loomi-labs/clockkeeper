package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	userIDKey      contextKey = "user_id"
	isAnonymousKey contextKey = "is_anonymous"
)

// UserIDFromContext returns the authenticated user ID from the context.
func UserIDFromContext(ctx context.Context) int {
	if v, ok := ctx.Value(userIDKey).(int); ok {
		return v
	}
	return 0
}

// IsAnonymousFromContext returns whether the authenticated user is anonymous.
func IsAnonymousFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(isAnonymousKey).(bool)
	return v
}

// AuthInterceptor validates JWT tokens on ConnectRPC requests.
type AuthInterceptor struct {
	secretKey []byte
}

// NewAuthInterceptor creates a new JWT auth interceptor.
func NewAuthInterceptor(secretKey string) *AuthInterceptor {
	return &AuthInterceptor{secretKey: []byte(secretKey)}
}

// skipAuth lists procedures that don't require authentication.
var skipAuth = map[string]bool{
	"/clockkeeper.v1.ClockKeeperService/LoginWithDiscord":       true,
	"/clockkeeper.v1.ClockKeeperService/CreateAnonymousSession": true,
	"/clockkeeper.v1.ClockKeeperService/GetAuthConfig":          true,
}

func (a *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if skipAuth[req.Spec().Procedure] {
			return next(ctx, req)
		}
		userID, isAnon, err := a.validate(req.Header().Get("Authorization"))
		if err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}
		ctx = context.WithValue(ctx, userIDKey, userID)
		ctx = context.WithValue(ctx, isAnonymousKey, isAnon)
		return next(ctx, req)
	}
}

func (a *AuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (a *AuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if skipAuth[conn.Spec().Procedure] {
			return next(ctx, conn)
		}
		userID, isAnon, err := a.validate(conn.RequestHeader().Get("Authorization"))
		if err != nil {
			return connect.NewError(connect.CodeUnauthenticated, err)
		}
		ctx = context.WithValue(ctx, userIDKey, userID)
		ctx = context.WithValue(ctx, isAnonymousKey, isAnon)
		return next(ctx, conn)
	}
}

func (a *AuthInterceptor) validate(authHeader string) (int, bool, error) {
	if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
		return 0, false, errors.New("missing or invalid authorization header")
	}
	tokenStr := authHeader[7:]

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.secretKey, nil
	})
	if err != nil || !token.Valid {
		return 0, false, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, false, errors.New("invalid token claims")
	}
	sub, _ := claims.GetSubject()
	if sub == "" {
		return 0, false, errors.New("missing subject in token")
	}
	userID, err := strconv.Atoi(sub)
	if err != nil {
		return 0, false, errors.New("invalid subject in token")
	}

	isAnon, _ := claims["anon"].(bool)

	return userID, isAnon, nil
}

// IssueToken creates a signed JWT for the given user.
// Anonymous users get a 7-day expiry, authenticated users get 30 days.
func (a *AuthInterceptor) IssueToken(userID int, anonymous bool) (string, error) {
	expiry := 30 * 24 * time.Hour
	if anonymous {
		expiry = 7 * 24 * time.Hour
	}
	jti, err := generateJTI()
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"sub": strconv.Itoa(userID),
		"exp": time.Now().Add(expiry).Unix(),
		"iat": time.Now().Unix(),
		"jti": jti,
	}
	if anonymous {
		claims["anon"] = true
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secretKey)
}

// generateJTI returns a random 16-byte hex string for use as a JWT ID.
func generateJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
