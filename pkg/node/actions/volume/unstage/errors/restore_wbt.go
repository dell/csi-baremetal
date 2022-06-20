package errors

import "fmt"

type RestoreWBTError struct {
	s string
}

func (e RestoreWBTError) Error() string {
	return fmt.Sprintf("failed to restore wbt: '%s'", e.s)
}

func NewRestoreWBTError(s string) RestoreWBTError {
	return RestoreWBTError{s: s}
}
