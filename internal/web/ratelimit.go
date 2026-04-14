package web

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/time/rate"
)

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimitInterceptor enforces per-user request rate limits.
// Authenticated users get a higher limit than anonymous users.
// Unauthenticated endpoints (skipAuth) are limited by peer IP.
type RateLimitInterceptor struct {
	mu        sync.Mutex
	limiters  map[string]*rateLimiterEntry
	anonRate  rate.Limit
	anonBurst int
	authRate  rate.Limit
	authBurst int
	done      chan struct{}
}

// NewRateLimitInterceptor creates a rate limiter with the given per-minute limits.
func NewRateLimitInterceptor(anonPerMin, authPerMin int) *RateLimitInterceptor {
	rl := &RateLimitInterceptor{
		limiters:  make(map[string]*rateLimiterEntry),
		anonRate:  rate.Limit(float64(anonPerMin) / 60.0),
		anonBurst: max(anonPerMin/3, 5),
		authRate:  rate.Limit(float64(authPerMin) / 60.0),
		authBurst: max(authPerMin/6, 10),
		done:      make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimitInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := rl.allow(ctx, req.Peer()); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (rl *RateLimitInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (rl *RateLimitInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := rl.allow(ctx, conn.Peer()); err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (rl *RateLimitInterceptor) allow(ctx context.Context, peer connect.Peer) error {
	userID := UserIDFromContext(ctx)
	isAnon := IsAnonymousFromContext(ctx)

	var key string
	var r rate.Limit
	var burst int

	if userID != 0 {
		tier := "auth"
		if isAnon {
			tier = "anon"
		}
		key = fmt.Sprintf("user:%d:%s", userID, tier)
		if isAnon {
			r, burst = rl.anonRate, rl.anonBurst
		} else {
			r, burst = rl.authRate, rl.authBurst
		}
	} else {
		// Unauthenticated endpoint — rate limit by IP.
		host, _, _ := net.SplitHostPort(peer.Addr)
		if host == "" {
			host = peer.Addr
		}
		key = "ip:" + host
		r, burst = rl.anonRate, rl.anonBurst
	}

	limiter := rl.getLimiter(key, r, burst)
	if !limiter.Allow() {
		return connect.NewError(connect.CodeResourceExhausted, errors.New("too many requests"))
	}
	return nil
}

func (rl *RateLimitInterceptor) getLimiter(key string, r rate.Limit, burst int) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, ok := rl.limiters[key]
	if !ok {
		entry = &rateLimiterEntry{
			limiter: rate.NewLimiter(r, burst),
		}
		rl.limiters[key] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

// Stop terminates the background cleanup goroutine.
func (rl *RateLimitInterceptor) Stop() {
	close(rl.done)
}

// cleanup removes stale rate limiter entries every 10 minutes.
func (rl *RateLimitInterceptor) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-rl.done:
			return
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-30 * time.Minute)
			for key, entry := range rl.limiters {
				if entry.lastSeen.Before(cutoff) {
					delete(rl.limiters, key)
				}
			}
			rl.mu.Unlock()
		}
	}
}
