package processor

import "context"

// Processor contains functional to execute via timer
type Processor interface {
	Handle(ctx context.Context)
}
