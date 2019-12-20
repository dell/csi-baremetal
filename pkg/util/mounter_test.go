package util

import (
	"testing"
)

func TestIsNotMounted(t *testing.T) {
	mounted := IsMounted("/dev/random", "/usr/local/bin")
	if mounted {
		t.Errorf("IsMounted should return false")
	}
}
