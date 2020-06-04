/*
Package events implement a library that will help send events to k8s.
The current implementation uses simple non-blocking EventRecorder.
But it causes goroutine leak in some edge cases(goroutine spawn isn't controlled).

Why not use k8s.io/client-go/tools/record?

You can. It has more features, but it's limited by event type.
Currently, types supported are Info and Warning.
Also lack labeling support.


For usage example see example_test.go
*/
package events
