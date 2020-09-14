package node

import (
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNodeLivenessCheck(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("OK by default", func(t *testing.T) {
		check := NewLivenessCheckHelper(logger, nil, nil)
		assert.True(t, check.Check())
	})

	t.Run("Marked as failed, ttl not expired", func(t *testing.T) {
		check := NewLivenessCheckHelper(logger, nil, nil)
		check.Fail()
		assert.True(t, check.Check())
	})

	t.Run("Marked as failed, ttl expired", func(t *testing.T) {
		ttl := time.Millisecond * 10
		check := NewLivenessCheckHelper(logger, &ttl, nil)
		check.Fail()
		time.Sleep(ttl)
		assert.False(t, check.Check())
	})

	t.Run("Marked OK, no updates for a long time", func(t *testing.T) {
		ttl := time.Millisecond * 10
		check := NewLivenessCheckHelper(logger, &ttl, nil)
		time.Sleep(ttl)
		assert.True(t, check.Check())
	})
	t.Run("Marked OK, no updates for a long time, timeout expired", func(t *testing.T) {
		ttl := time.Millisecond * 10
		timeout := time.Millisecond * 20
		check := NewLivenessCheckHelper(logger, &ttl, &timeout)
		time.Sleep(timeout)
		assert.False(t, check.Check())
	})

	t.Run("Recover", func(t *testing.T) {
		ttl := time.Millisecond * 10
		check := NewLivenessCheckHelper(logger, &ttl, nil)
		check.Fail()
		time.Sleep(ttl)
		assert.False(t, check.Check())
		check.OK()
		assert.True(t, check.Check())
	})
}
