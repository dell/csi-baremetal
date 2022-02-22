package backoff

import (
	"errors"
	"math/rand"
	"time"
)

// ErrMaxBackoffRetriesExceeded stays for max backoff retries exceeded error
var ErrMaxBackoffRetriesExceeded = errors.New("max backoff retries exceeded")

// Handler is a backoff handler
type Handler interface {
	Handle(retries int) time.Duration
}

type exponentialHandler struct {
	config *Config
}

func (s *exponentialHandler) Handle(retries int) time.Duration {
	// if there were no retries, then return baseDelay time, which is 1 second by default
	if retries == 0 {
		return s.config.BaseDelay
	}
	// recalculating current backoff and comparing with maximum
	curBackoff, max := float64(s.config.BaseDelay), float64(s.config.MaxDelay)
	for curBackoff < max && retries > 0 {
		curBackoff *= s.config.Multiplier
		retries--
	}
	if curBackoff > max {
		curBackoff = max
	}
	// randomize backoff delays so that if cluster of requests start at
	// the same time, they won't operate in lockstep.
	if curBackoff *= 1 + s.config.Jitter*(rand.Float64()*2-1); curBackoff < 0 {
		return 0
	}
	return time.Duration(curBackoff)
}

// NewExponentialHandler is a constructor for backoff handler
func NewExponentialHandler(config *Config) Handler {
	return &exponentialHandler{
		config: config,
	}
}
