# Proposal:  CSI deployment CRD

Last updated: 05.03.20

## Abstract

To deploy CSI component with operator, it's necessary to provide CRD so operator can deploy CSI programmable using its configuration 

## Background

Currently CSI is deployed using helm charts and value

## Proposal

Example of CRD:
```
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: csi.dell.com
  labels:
    app.kubernetes.io/name: csi-operator
spec:
  group: csi.dell.com
  names:
    kind: CSI
    listKind: CSIList
    plural: csi
    singular: csi
  scope: Namespaced
  versions:
    - name: v1beta1
      served: true
      storage: true
```

Example of CR:
```
apiVersion: csi.dell.com/v1beta1
kind: CSI
metadata:
  name: csi
  namespace: default
  labels:
    app.kubernetes.io/name: csi-baremetal
    app.kubernetes.io/managed-by: csi-operator
    app.kubernetes.io/version: 1.0.0
spec:
  driver:
    controller:
      image:
        registry: asdrepo.isus.emc.com:9042
        name: csi-baremetal-driver
        pullPolicy: Always
        tag: green
      log:
        format: text
        level: info
      sidecars:
        - name: csi-provisioner
          image:
            name: csi-provisioner
            registry: asdrepo.isus.emc.com:9042
            pullPolicy: Always
            tag: v1.6.0
        - name: csi-attacher
          image:
            name: csi-attacher
            registry: asdrepo.isus.emc.com:9042
            pullPolicy: Always
            tag: v1.0.1
        - name: csi-resizer
          image:
            name: csi-resizer
            registry: asdrepo.isus.emc.com:9042
            pullPolicy: Always
            tag: v1.1.0
      testEnv: false
    node:
      driveMgr:
        image:
          name: csi-baremetal-basemgr
          registry: asdrepo.isus.emc.com:9042
          pullPolicy: Always
          tag: green
        endpoint: tcp://localhost:8888
        deployConfig: false
        amountOfLoopback: 3
        sizeOfLoopback: 101Mi
      image:
        registry: asdrepo.isus.emc.com:9042
        pullPolicy: Always
        tag: green
      log:
        format: text
        level: info
      sidecars:
        - name: csi-node-driver-registrar
          image:
            name: csi-node-driver-registrar
            registry: asdrepo.isus.emc.com:9042
            pullPolicy: Always
            tag: v1.0.1-gke.0
      testEnv: false
    metrics:
      path: /metrics
      port: 8787
    logReceiver:
      name: fluent-bit
      image:
        name: fluent-bit
        registry: asdrepo.isus.emc.com:9042
        pullPolicy: Always
        tag: shippable
    deployAlertsConfig: false
  scheduler:
    image:
      name: csi-baremetal-scheduler-extender
      registry: asdrepo.isus.emc.com:9042
      pullPolicy: Always
      tag: green
    log:
      format: text
      level: info
    metrics:
      path: /metrics
      port: 8787
    patcher:
      enable: false
      manifest: /etc/kubernetes/manifests/kube-scheduler.yaml
      srcConfigPath: config.yaml
      srcPolicyPath: policy.yaml
      targetConfigPath: /etc/kubernetes/scheduler/config.yaml
      targetPolicyPath: /etc/kubernetes/scheduler/policy.yaml
      interval: 60
      restoreOnShutdown: false
      configMapName: schedulerpatcher-config
    storageProvisioner: csi-baremetal
    testEnv: false
  operator:
    image:
      name: csi-baremetal-operator
      registry: asdrepo.isus.emc.com:9042
      pullPolicy: Always
      tag: green
    log:
      format: text
      level: info
    testEnv: false
```

Example of CR in code:

```
type CSI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              CSISpec   `json:"spec"`
	Status            CSIStatus `json:"status,omitempty"`
}

type CSISpec struct {
	Driver       *Driver       `json:"driver,omitempty"`
	NodeOperator *NodeOperator `json:"operator,omitempty"`
	Scheduler    *Scheduler    `json:"scheduler,omitempty"`
}

type Driver struct {
	Controller *Controller `json:"controller,omitempty"`
	Node       *Node       `json:"node,omitempty"`
	Image      *Image      `json:"image,omitempty"`
	Metrics    *Metrics    `json:"metrics,omitempty"`
	LogReceiver *LogReceiver `json:"logReceiver,omitempty"`
	DeployAlertsConfig bool  `json:"deployAlertsConfig,omitempty"`
}

type Controller struct {
	Image       *Image       `json:"image,omitempty"`
	Log         *Log         `json:"log,omitempty"`
	Sidecars    []*Sidecar   `json:"sidecars,omitempty"`
	TestEnv     bool         `json:"testEnv,omitempty"`
}

type Node struct {
	DriveMgr    *DriveMgr    `json:"driveMgr,omitempty"`
	Image       *Image       `json:"image,omitempty"`
	Log         *Log         `json:"log,omitempty"`
	Sidecars    []*Sidecar   `json:"sidecars,omitempty"`
	TestEnv     bool         `json:"testEnv,omitempty"`
}

type DriveMgr struct {
	Image            *Image `json:"image,omitempty"`
	Endpoint         string `json:"string,omitempty"`
	DeployConfig     bool   `json:"deployConfig,omitempty"`
	AmountOfLoopback bool   `json:"amountOfLoopback,omitempty"`
	SizeOfLoopback   bool   `json:"sizeOfLoopback,omitempty"`
}

type NodeOperator struct {
	Image   *Image `json:"image,omitempty"`
	Log     *Log   `json:"log,omitempty"`
	TestEnv bool   `json:"testEnv,omitempty"`
}

type Scheduler struct {
	Image              *Image   `json:"image,omitempty"`
	Log                *Log     `json:"log,omitempty"`
	Metrics            *Metrics `json:"metrics,omitempty"`
	Patcher            *Patcher `json:"patcher,omitempty"`
	StorageProvisioner string   `json:"storageProvisioner,omitempty"`
	TestEnv            bool     `json:"testEnv,omitempty"`
}

type Patcher struct {
	Enable            bool   `json:"enable,omitempty"`
	Manifest          string `json:"manifest,omitempty"`
	SrcConfigPath     string `json:"srcConfigPath,omitempty"`
	SrcPolicyPath     string `json:"srcPolicyPath,omitempty"`
	TargetConfigPath  string `json:"targetConfigPath,omitempty"`
	TargetPolicyPath  string `json:"targetPolicyPath,omitempty"`
	Interval          int    `json:"interval,omitempty"`
	RestoreOnShutdown bool   `json:"restoreOnShutdown,omitempty"`
	ConfigMapName     string `json:"configMapName,omitempty"`
}

type Log struct {
	Format *LogFormat   `json:"format,omitempty"`
	Level  *Level `json:"level,omitempty"`
}

type Sidecar struct {
	Name  string `json:"name,omitempty"`
	Image *Image `json:"image,omitempty"`
}

type LogReceiver struct {
	Name  string `json:"name,omitempty"`
	Image *Image `json:"image,omitempty"`
}

type Image struct {
	Registry   string `json:"registry,omitempty"`
	Name       string `json:"name,omitempty"`
	Tag        string `json:"tag,omitempty"`
	PullPolicy string `json:"pullPolicy,omitempty"`
}

type Metrics struct {
	Path string `json:"path,omitempty"`
	Port string `json:"port,omitempty"`
}
```

## Compatibility

There is no problem with compatibility

