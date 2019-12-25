package util

import (
	"testing"
)

func TestIsNotMounted(t *testing.T) {
	mounted := IsMountedBockDevice("/dev/random", "/usr/local/bin")
	if mounted {
		t.Errorf("IsMountedBockDevice should return false")
	}
}
