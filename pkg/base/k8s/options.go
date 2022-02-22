package k8s

// DefaultMaxBackoffRetries is a maximum backoff reties number by default
var DefaultMaxBackoffRetries = 3

// ClientOptions represents kubeclient request options, such as maximum backoff retries
type ClientOptions struct {
	MaxBackoffRetries *int
}

// DefaultClientOptions helps to initialize default kubeclient request options
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		MaxBackoffRetries: &DefaultMaxBackoffRetries,
	}
}

func mergeClientOptions(opts ...*ClientOptions) ClientOptions {
	k := DefaultClientOptions()

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
