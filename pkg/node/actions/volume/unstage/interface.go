package unstage

import "context"

type Action interface {
	Handle(ctx context.Context) error
}
