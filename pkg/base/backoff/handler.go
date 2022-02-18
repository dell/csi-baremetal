package backoff

import (
	"errors"
	"math/rand"
	"time"

	"google.golang.org/grpc/backoff"
)

var ErrMaxBackoffRetriesExceeded = errors.New("max backoff retries exceeded")

type Handler interface {
	Handle(retries int) time.Duration
}

type exponentialHandler struct {
	config *backoff.Config
}

func (s *exponentialHandler) Handle(retries int) time.Duration {
	// if there were no retries, then return baseDelay time
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

func NewExponentialHandler(config *backoff.Config) Handler {
	return &exponentialHandler{
		config: config,
	}
}
