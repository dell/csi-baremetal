package error

import "errors"

// ErrorNotFound indicates that requested object wasn't found
var ErrorNotFound = errors.New("not found")
