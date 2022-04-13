# CRDs generation routine

CRDs generated in this repository and placed to the [`csi-baremetal-operator`](https://github.com/dell/csi-baremetal-operator) repository (charts folder).

All charts crds generated with [`controller-tools`](https://github.com/kubernetes-sigs/controller-tools).

### **Don't edit crds manually** 

Use right annotation for your structs so validations would be applied on the install step.

Refer to [`CRD Validation`](https://book.kubebuilder.io/reference/markers/crd-validation.html) documentation for more info.

