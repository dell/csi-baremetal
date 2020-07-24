package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	k8sV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api/v1"
	_ "sigs.k8s.io/controller-runtime/pkg/client"
	k8sCl "sigs.k8s.io/controller-runtime/pkg/client"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

type Extender struct {
	k8sClient *k8s.KubeClient
	logger    *logrus.Entry
}

const (
	pluginNameMask = "baremetal"
)

func NewExtender(logger *logrus.Logger) *Extender {
	k8sClient, err := k8s.GetK8SClient()
	if err != nil {
		logger.Fatalf("fail to create kubernetes client, error: %v", err)
	}
	kubeClient := k8s.NewKubeClient(k8sClient, logger, "kube-system")
	return &Extender{
		k8sClient: kubeClient,
		logger:    logger.WithField("component", "Extender"),
	}
}

// FilterHandler extracts ExtenderArgs struct from req and writes ExtenderFilterResult to the w
func (e *Extender) FilterHandler(w http.ResponseWriter, req *http.Request) {
	ll := e.logger.WithField("method", "FilterHandler")
	ll.Debugf("Processing request: %v", req)

	w.Header().Set("Content-Type", "application/json")
	resp := json.NewEncoder(w)

	var (
		extenderArgs schedulerapi.ExtenderArgs
		extenderRes  = &schedulerapi.ExtenderFilterResult{}
	)

	if err := json.NewDecoder(req.Body).Decode(&extenderArgs); err != nil {
		ll.Errorf("Unable to decode request body: %v", err)
		extenderRes.Error = err.Error()
		e.encodeResults(resp, extenderRes)
	}

	ll.Info("Running filter")
	extenderFilterResult, err := e.filter(req.Context(), extenderArgs)
	if err != nil {
		extenderRes.Error = err.Error()
	} else {
		extenderRes = extenderFilterResult
	}
	e.encodeResults(resp, extenderRes)
}

func (e *Extender) encodeResults(resp *json.Encoder, res *schedulerapi.ExtenderFilterResult) {
	ll := e.logger.WithField("method", "encodeResults")
	if err := resp.Encode(res); err != nil {
		ll.Errorf("Unable to write response: %v", err)
	} else if res.Error == "" {
		ll.Infof("ExtenderFilterResult was written, not suitable nodes: %+v", res.FailedNodes)
	}
}

// filter filters nodes that don't fit pod volumes requirements and constructs ExtenderFilterResult struct
func (e *Extender) filter(ctx context.Context,
	args schedulerapi.ExtenderArgs) (*schedulerapi.ExtenderFilterResult, error) {
	pod := args.Pod
	ll := e.logger.WithFields(logrus.Fields{
		"method": "filter",
		"pod":    pod.Name,
	})
	ll.Debug("Processing ...")

	volumes := make([]*genV1.Volume, 0)

	// check whether there are Ephemeral volumes or no
	for _, v := range pod.Spec.Volumes {
		e.logger.Debugf("Inspecting pod volume %+v", v)
		if v.CSI != nil {
			if strings.Contains(v.CSI.Driver, pluginNameMask) {
				volume, err := e.constructVolumeFromCSISource(v)
				if err != nil {
					ll.Errorf("Unable to construct API Volume for Ephemeral volume: %v", err)
				}
				// need to apply any result for getting at leas amount of volumes
				volumes = append(volumes, volume)
			}
			continue
		}
		if v.PersistentVolumeClaim != nil {
			pvc := &k8sV1.PersistentVolumeClaim{}
			err := e.k8sClient.Get(ctx,
				k8sCl.ObjectKey{Name: v.PersistentVolumeClaim.ClaimName, Namespace: pod.Namespace},
				pvc)
			if err != nil {
				e.logger.Errorf("Unable to read PVC %s in NS %s: %v. ", v.PersistentVolumeClaim.ClaimName, pod.Namespace, err)
				return nil, err
			}
			if strings.Contains(*pvc.Spec.StorageClassName, "baremetal") {
				storageRes, ok := pvc.Spec.Resources.Requests[k8sV1.ResourceStorage]
				if !ok {
					ll.Errorf("There is no key for storage resource for PVC %s", pvc.Name)
					storageRes = resource.Quantity{}
				}
				volumes = append(volumes, &genV1.Volume{
					Id:           pvc.Name,
					StorageClass: *pvc.Spec.StorageClassName,
					Size:         storageRes.Value(),
					Mode:         string(*pvc.Spec.VolumeMode),
					Ephemeral:    false,
				})
			}
		}
	}
	ll.Debugf("Required volumes: %v", volumes)

	var toReturn = &schedulerapi.ExtenderFilterResult{
		Nodes:       args.Nodes,
		NodeNames:   nil,
		FailedNodes: nil,
		Error:       "",
	}
	return toReturn, nil
}

func (e *Extender) constructVolumeFromCSISource(v k8sV1.Volume) (vol *genV1.Volume, err error) {
	vol = &genV1.Volume{StorageClass: apiV1.StorageClassAny}

	sc, ok := v.CSI.VolumeAttributes[base.StorageTypeKey]
	if !ok {
		return vol, fmt.Errorf("unable to detect storage class for volume %s for attributes %v",
			v.Name, v.CSI.VolumeAttributes)
	}

	sizeStr, ok := v.CSI.VolumeAttributes[base.SizeKey]
	if !ok {
		return vol, fmt.Errorf("unable to detect size for volume %s for attributes %v",
			v.Name, v.CSI.VolumeAttributes)
	}

	size, err := util.StrToBytes(sizeStr)
	if err != nil {
		return vol, fmt.Errorf("unable to convert string %s to bytes: %v", sizeStr, err)
	}

	return &genV1.Volume{
		Id:           v.Name,
		StorageClass: sc,
		Size:         size,
		Ephemeral:    true,
	}, nil
}
