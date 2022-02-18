package k8s

var DefaultMaxBackoffRetries = 0

type KubeClientRequestOptions struct {
	MaxBackoffRetries *int
}

func DefaultKubeClientRequestOptions() KubeClientRequestOptions{
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
