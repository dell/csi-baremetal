module github.com/dell/csi-baremetal

go 1.15

require (
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/antonfisher/nested-logrus-formatter v1.0.3
	github.com/container-storage-interface/spec v1.2.0
	github.com/coreos/rkt v1.30.0
	github.com/dell/csi-baremetal-operator v0.4.0-beta // indirect
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-kit/log v0.1.0 // indirect
	github.com/go-task/slim-sprig v0.0.0-20210107165309-348f09dbbbc0 // indirect
	github.com/golang/protobuf v1.4.2
	github.com/google/uuid v1.1.1
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/kubernetes-csi/csi-test/v3 v3.1.0
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.7.0
	// ./bin/mockery --dir=/usr/share/go/pkg/mod/k8s.io/client-go\@v0.18.19/kubernetes/typed/core/v1/ --name=EventInterface
	github.com/vektra/mockery/v2 v2.9.4 // indirect
	golang.org/x/net v0.0.0-20210428140749-89ef3d95e781
	golang.org/x/sys v0.0.0-20210603081109-ebe580a85c40 // indirect
	google.golang.org/grpc v1.27.0
	google.golang.org/protobuf v1.26.0-rc.1 // indirect
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools v2.2.0+incompatible
	k8s.io/api v1.18.19
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v1.18.19
	k8s.io/kube-scheduler v0.18.19
	k8s.io/kubernetes v1.18.19
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176
	sigs.k8s.io/controller-runtime v0.7.2
)

replace (
	k8s.io/api => k8s.io/api v0.18.19
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.19
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.19
	k8s.io/apiserver => k8s.io/apiserver v0.18.19
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.19
	k8s.io/client-go => k8s.io/client-go v0.18.19
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.19
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.19
	k8s.io/code-generator => k8s.io/code-generator v0.18.19
	k8s.io/component-base => k8s.io/component-base v0.18.19
	k8s.io/cri-api => k8s.io/cri-api v0.18.19
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.18.19
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.19
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.18.19
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.18.19
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.18.19
	k8s.io/kubectl => k8s.io/kubectl v0.18.19
	k8s.io/kubelet => k8s.io/kubelet v0.18.19
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.19
	k8s.io/metrics => k8s.io/metrics v0.18.19
	k8s.io/node-api => k8s.io/node-api v0.18.19
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.19
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.18.19
	k8s.io/sample-controller => k8s.io/sample-controller v0.18.19
)
