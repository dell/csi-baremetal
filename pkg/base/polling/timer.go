package polling

import (
	"context"
	"time"
)

type timer struct {
	duration time.Duration
}
type handle func(context.Context)

func (t *timer) Start(ctx context.Context, handle handle) {
	timer := time.NewTimer(t.duration)
	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			handle(ctx)
			timer.Reset(t.duration)
		}

	}
}

func NewTimer(dur time.Duration) *timer {
	return &timer{
		duration: dur}
}
