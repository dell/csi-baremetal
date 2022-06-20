package actions

import "context"

type Actions []action

func (a Actions) Apply(ctx context.Context) error {
	for _, action := range a {
		if err := action.Handle(ctx); err != nil {
			return err
		}
	}

	return nil
}

type action interface {
	Handle(ctx context.Context) error
}
