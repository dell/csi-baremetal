module eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git

go 1.14

require (
	github.com/antonfisher/nested-logrus-formatter v1.0.3
	github.com/container-storage-interface/spec v1.2.0
	github.com/coreos/rkt v1.30.0
	github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp v0.4.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/jstemmer/go-junit-report v0.9.1 // indirect
	github.com/kubernetes-csi/csi-test/v3 v3.1.0
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.7.1
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.4.0
	golang.org/x/arch v0.0.0-20200312215426-ff8b605520f4 // indirect
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550 // indirect
	golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/grpc v1.27.0
	gopkg.in/yaml.v2 v2.2.5
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.16.4
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/kubernetes v1.16.4
	sigs.k8s.io/controller-runtime v0.4.0
	sigs.k8s.io/controller-tools v0.2.2 // indirect
)

replace (
	github.com/coreos/etcd v3.3.17+incompatible => github.com/coreos/etcd v3.3.4+incompatible
	k8s.io/api => k8s.io/api v0.16.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.16.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.4
	k8s.io/apiserver => k8s.io/apiserver v0.16.4
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.16.4
	k8s.io/client-go => k8s.io/client-go v0.16.4
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.16.4
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.16.4
	k8s.io/code-generator => k8s.io/code-generator v0.16.4
	k8s.io/component-base => k8s.io/component-base v0.16.4
	k8s.io/cri-api => k8s.io/cri-api v0.16.4
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.16.4
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.16.4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.16.4
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.16.4
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.16.4
	k8s.io/kubectl => k8s.io/kubectl v0.16.4
	k8s.io/kubelet => k8s.io/kubelet v0.16.4
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.16.4
	k8s.io/metrics => k8s.io/metrics v0.16.4
	k8s.io/node-api => k8s.io/node-api v0.16.4
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.16.4
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.16.4
	k8s.io/sample-controller => k8s.io/sample-controller v0.16.4
)
