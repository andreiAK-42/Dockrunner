package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	r        rate.Limit
	burst    int
	ttl      time.Duration
}

func NewRateLimiter(rps, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		r:        rate.Limit(rps),
		burst:    burst,
		ttl:      3 * time.Minute,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if v, ok := rl.visitors[ip]; ok {
		v.lastSeen = time.Now()
		return v.limiter
	}
	limiter := rate.NewLimiter(rl.r, rl.burst)
	rl.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
	return limiter
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > rl.ttl {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}
