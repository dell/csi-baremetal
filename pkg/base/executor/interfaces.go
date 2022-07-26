package executor

import "context"

type handle func(ctx context.Context)

// Executor is a processor of handle functor with retry duration
type Executor interface {
	Start(ctx context.Context, handle handle)
}
