package mocks

import "k8s.io/apimachinery/pkg/runtime"

// NoOpRecorder is blank implementation of event recorder interface
type NoOpRecorder struct{}

// Eventf do nothing
func (n *NoOpRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
}
