/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
		lastOK:  time.Now(),
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
func (h *LivenessCheckHelper) OK() {
	h.m.Lock()
	h.lastOK = time.Now()
	h.isOK = true
	h.logger.Debug("updated with OK")
	h.m.Unlock()
}

// Fail marks check as failed
func (h *LivenessCheckHelper) Fail() {
	h.m.Lock()
	h.isOK = false
	h.logger.Debug("updated with Fail")
	h.m.Unlock()
}

// Check returns computed liveness check result
func (h *LivenessCheckHelper) Check() bool {
	h.m.RLock()
	defer h.m.RUnlock()
	if time.Now().Before(h.lastOK.Add(h.ttl)) {
		// ttl not expired yet
		h.logger.Debug("Check: OK")
		return true
	}
	if !h.isOK {
		// ttl expired and a failed result detected
		h.logger.Debug("Check: failed")
		return false
	}
	if time.Since(h.lastOK) > h.timeout {
		// hard timeout: fail not detected, but there are no OKs for a long time
		h.logger.Warn("Check: failed, hard timeout")
		return false
	}
	// ttl expired but, fail not detected, hard timeout not happen yet
	// we return true, because drive discover can still run
	h.logger.Debug("Check: OK, ttl expired")
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
