package processor

import "context"

type Processor interface {
	Handle(ctx context.Context)
}
