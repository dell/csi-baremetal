package k8s

// DefaultMaxBackoffRetries is a maximum backoff reties number by default
var DefaultMaxBackoffRetries = 3

// KubeClientRequestOptions represents kubeclient request options, such as maximum backoff retries
type KubeClientRequestOptions struct {
	MaxBackoffRetries *int
}

// DefaultKubeClientRequestOptions helps to initialize default kubeclient request options
func DefaultKubeClientRequestOptions() KubeClientRequestOptions {
	return KubeClientRequestOptions{
		MaxBackoffRetries: &DefaultMaxBackoffRetries,
	}
}

func mergeKubeClientRequestOptions(opts ...*KubeClientRequestOptions) KubeClientRequestOptions {
	k := DefaultKubeClientRequestOptions()

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if opt.MaxBackoffRetries != nil {
			k.MaxBackoffRetries = opt.MaxBackoffRetries
		}
	}

	return k
}
