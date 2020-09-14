package node

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// LivenessDefaultTTL default TTL for node liveness checker
	LivenessDefaultTTL = 2 * time.Minute
	// LivenessDefaultTimeout hard timeout for node liveness checker
	LivenessDefaultTimeout = 10 * time.Minute
)

// LivenessHelper is an interface that provide method for liveness check
type LivenessHelper interface {
	OK()
	Fail()
	Check() bool
}

// NewLivenessCheckHelper returns new instance of LivenessCheckHelper
func NewLivenessCheckHelper(logger *logrus.Logger, ttl *time.Duration, timeout *time.Duration) *LivenessCheckHelper {
	now := time.Now()
	tTTL := LivenessDefaultTTL
	if ttl != nil {
		tTTL = *ttl
	}
	tTimeout := LivenessDefaultTimeout
	if timeout != nil {
		tTimeout = *timeout
	}
	return &LivenessCheckHelper{
		ttl:     tTTL,
		timeout: tTimeout,
		lastOK:  now,
		isOK:    true,
		logger:  logger.WithField("component", "LivenessCheckHelper"),
	}
}

// LivenessCheckHelper is a helper for node liveness checking
type LivenessCheckHelper struct {
	m      sync.RWMutex
	logger *logrus.Entry

	ttl     time.Duration
	timeout time.Duration

	lastOK time.Time
	isOK   bool
}

// OK marks check as OK, update TTL
func (nlc *LivenessCheckHelper) OK() {
	nlc.m.Lock()
	nlc.lastOK = time.Now()
	nlc.isOK = true
	nlc.logger.Debug("updated with OK")
	nlc.m.Unlock()
}

// Fail marks check as failed
func (nlc *LivenessCheckHelper) Fail() {
	nlc.m.Lock()
	nlc.isOK = false
	nlc.logger.Debug("updated with Fail")
	nlc.m.Unlock()
}

// Check returns computed liveness check result
func (nlc *LivenessCheckHelper) Check() bool {
	nlc.m.RLock()
	defer nlc.m.RUnlock()
	if time.Now().Before(nlc.lastOK.Add(nlc.ttl)) {
		// ttl not expired yet
		nlc.logger.Debug("Check: OK")
		return true
	}
	if !nlc.isOK {
		// ttl expired and a failed result detected
		nlc.logger.Debug("Check: failed")
		return false
	}
	if time.Since(nlc.lastOK) > nlc.timeout {
		// hard timeout: fail not detected, but there are no OKs for a long time
		nlc.logger.Warn("Check: failed, hard timeout")
		return false
	}
	// ttl expired but, fail not detected, hard timeout not happen yet
	// we return true, because drive discover can still run
	nlc.logger.Debug("Check: OK, ttl expired")
	return true
}

// DummyLivenessHelper is a dummy implementation of LivenessHelper interface
type DummyLivenessHelper struct {
	CheckResult bool
}

// OK dummy implementation of OK
func (d *DummyLivenessHelper) OK() {}

// Fail dummy implementation of Fail
func (d *DummyLivenessHelper) Fail() {}

// Check dummy implementation of Check
func (d *DummyLivenessHelper) Check() bool { return d.CheckResult }
