package error

import "errors"

// ErrorNotFound indicates that requested object wasn't found
var (
	ErrorNotFound                 = errors.New("not found")
	ErrorEmptyParameter           = errors.New("empty parameter")
	ErrorFailedParsing            = errors.New("failed to parse")
	ErrorGetDriveFailed           = errors.New("failed to get drive cr")
	ErrorRejectReservationRequest = errors.New("reject reservation request")
)
