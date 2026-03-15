package web

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

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
	"/clockkeeper.v1.ClockKeeperService/Login": true,
}

func (a *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if skipAuth[req.Spec().Procedure] {
			return next(ctx, req)
		}
		if err := a.validate(req.Header().Get("Authorization")); err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}
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
		if err := a.validate(conn.RequestHeader().Get("Authorization")); err != nil {
			return connect.NewError(connect.CodeUnauthenticated, err)
		}
		return next(ctx, conn)
	}
}

func (a *AuthInterceptor) validate(authHeader string) error {
	if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
		return errors.New("missing or invalid authorization header")
	}
	tokenStr := authHeader[7:]

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.secretKey, nil
	})
	if err != nil || !token.Valid {
		return errors.New("invalid token")
	}
	return nil
}

// IssueToken creates a signed JWT with 30-day expiry.
func (a *AuthInterceptor) IssueToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(30 * 24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secretKey)
}

// HashPassword hashes a plaintext password using bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
