module github.com/dell/csi-baremetal

go 1.16

require (
	github.com/antonfisher/nested-logrus-formatter v1.0.3
	github.com/banzaicloud/logrus-runtime-formatter v0.0.0-20190729070250-5ae5475bae5e
	github.com/container-storage-interface/spec v1.5.0
	github.com/coreos/rkt v1.30.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/google/uuid v1.1.2
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/kubernetes-csi/csi-test/v3 v3.1.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.14.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/vektra/mockery/v2 v2.9.4 // indirect
	golang.org/x/net v0.0.0-20211209124913-491a49abca63
	golang.org/x/tools v0.1.5 // indirect
	google.golang.org/grpc v1.38.0
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools v2.2.0+incompatible
	k8s.io/api v1.22.5
	k8s.io/apiextensions-apiserver v0.22.2 // indirect
	k8s.io/apimachinery v0.22.5
	k8s.io/client-go v0.22.5
	k8s.io/kube-scheduler v0.22.5
	k8s.io/kubernetes v1.22.5
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/controller-runtime v0.9.0
	sigs.k8s.io/controller-tools v0.5.0 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.22.5
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.22.5
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.5
	k8s.io/apiserver => k8s.io/apiserver v0.22.5
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.22.5
	k8s.io/client-go => k8s.io/client-go v0.22.5
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.22.5
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.5
	k8s.io/code-generator => k8s.io/code-generator v0.22.5
	k8s.io/component-base => k8s.io/component-base v0.22.5
	k8s.io/component-helpers => k8s.io/component-helpers v0.22.5
	k8s.io/controller-manager => k8s.io/controller-manager v0.22.5
	k8s.io/cri-api => k8s.io/cri-api v0.22.5
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.22.5
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.22.5
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.22.5
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.22.5
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.22.5
	k8s.io/kubectl => k8s.io/kubectl v0.22.5
	k8s.io/kubelet => k8s.io/kubelet v0.22.5
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.22.5
	k8s.io/metrics => k8s.io/metrics v0.22.5
	k8s.io/mount-utils => k8s.io/mount-utils v0.22.5
	k8s.io/node-api => k8s.io/node-api v0.22.5
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.22.5
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.22.5
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.22.5
	k8s.io/sample-controller => k8s.io/sample-controller v0.22.5
)
