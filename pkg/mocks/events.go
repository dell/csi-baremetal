package mocks

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
)

type eventRecorderCalls struct {
	Object     runtime.Object
	Eventtype  string
	Reason     string
	MessageFmt string
	Args       []interface{}
}

// NoOpRecorder is blank implementation of event recorder interface which stores calls to the interface methods
type NoOpRecorder struct {
	Calls []eventRecorderCalls
	m     sync.Mutex
}

// Eventf do nothing
func (n *NoOpRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	c := eventRecorderCalls{
		Object:     object,
		Eventtype:  eventtype,
		Reason:     reason,
		MessageFmt: messageFmt,
		Args:       args,
	}
	n.m.Lock()
	n.Calls = append(n.Calls, c)
	n.m.Unlock()
}
