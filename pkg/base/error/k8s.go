package error

import k8sError "k8s.io/apimachinery/pkg/api/errors"

// IsSafeReturnError returns if error is safe to retry
func IsSafeReturnError(err error) bool {
	return k8sError.IsServerTimeout(err) ||
		k8sError.IsTimeout(err) ||
		k8sError.IsTooManyRequests(err)
}

// AlwaysSafeReturnError returns true for all errors
func AlwaysSafeReturnError(err error) bool {
	return true
}
