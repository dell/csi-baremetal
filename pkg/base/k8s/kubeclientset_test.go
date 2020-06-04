package k8s

import (
	"k8s.io/client-go/rest"
	"testing"
)

// Since we run test not in k8s, it will always fail
// It ain't much but it's honest work.
func TestGetK8SClientset(t *testing.T) {
	_, err := GetK8SClientset()
	if err != rest.ErrNotInCluster {
		t.Errorf("GetK8SClientset() error got = %v, want %v", err, rest.ErrNotInCluster)
	}
}
