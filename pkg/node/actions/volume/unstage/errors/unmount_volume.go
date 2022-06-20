package errors

import "fmt"

type UnmountVolumeError struct {
	s string
}

func (e UnmountVolumeError) Error() string {
	return fmt.Sprintf("failed to unmount volume: '%s'", e.s)
}

func NewUnmountVolumeError(s string) UnmountVolumeError {
	return UnmountVolumeError{s: s}
}
