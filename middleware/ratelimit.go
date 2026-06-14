package middleware

import (
    "net/http"
    "sync"

    "golang.org/x/time/rate"
)

type IPRateLimiter struct {
    ips map[string]*rate.Limiter
    mu  sync.RWMutex
    r   rate.Limit
    b   int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
    return &IPRateLimiter{
        ips: make(map[string]*rate.Limiter),
        r:   r,
        b:   b,
    }
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
    i.mu.Lock()
    defer i.mu.Unlock()

    limiter, exists := i.ips[ip]
    if !exists {
        limiter = rate.NewLimiter(i.r, i.b)
        i.ips[ip] = limiter
    }
    return limiter
}

func RateLimitMiddleware(limiter *IPRateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip := r.RemoteAddr
            l := limiter.GetLimiter(ip)
            if !l.Allow() {
                http.Error(w, `{"error":"demasiadas solicitudes","code":429}`, http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}