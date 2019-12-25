package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/lvm"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/util"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Server struct {
	Port      int
	HandleLVM bool
	mutex     *sync.Mutex
}

const VgName = "csivg" // TODO: do not hardcode it, vg name should provide in request

// GetDisks is a function for getting disks from node
func (s *Server) GetDisks(w http.ResponseWriter, r *http.Request) {
	ll := logrus.WithFields(logrus.Fields{
		"component": "restServer",
		"method":    "GetDisks",
	})
	ll.Info("Processing request")

	w.Header().Set("Content-Type", "application/json")

	disks := util.AllDisks()
	ll.Info(disks)
	err := json.NewEncoder(w).Encode(disks)

	if err != nil {
		logrus.Errorf("Failed to get disk: %v", err)
	}
}

// GetVolumeGroupInfo returns Volume Group info
func (s *Server) GetVolumeGroupInfoHandler(w http.ResponseWriter, r *http.Request) {
	ll := logrus.WithFields(logrus.Fields{
		"component": "restServer",
		"method":    "GetVolumeGroupInfoHandler",
	})
	ll.Info("Processing request")

	w.Header().Set("Content-Type", "application/json")

	nodeVG, err := lvm.VolumeGroupState(VgName) // TODO: do not hardcode this name
	if err != nil {
		ll.Errorf("failed to get state of volume group %s, error: %v", VgName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ll.Info(nodeVG)
	err = json.NewEncoder(w).Encode(nodeVG)

	if err != nil {
		ll.Errorf("Failed to get Volume Group: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// CreateLogicalVolumeHandler create LV on node
func (s *Server) CreateLogicalVolumeHandler(w http.ResponseWriter, r *http.Request) {
	ll := logrus.WithFields(logrus.Fields{
		"component": "restServer",
		"method":    "CreateLogicalVolumeHandler",
	})
	ll.Info("Got request")

	s.mutex.Lock()
	defer func() {
		s.mutex.Unlock()
		ll.Infof("Server Mutex was unlocked")
	}()
	ll.Info("Mutex Locked. Start processing ...")

	var createLV lvm.LogicalVolume
	err := json.NewDecoder(r.Body).Decode(&createLV)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(createLV.LVSize) == 0 {
		http.Error(w, "LV size should be provided", http.StatusBadRequest)
		return
	}
	if ok, _ := lvm.IsLogicalVolumeExist(createLV.Name, createLV.VGName); ok {
		ll.Infof("LV %v exist.", createLV)
		w.WriteHeader(http.StatusOK)
		return
	}
	err = lvm.CreateLogicalVolume(createLV.Name, createLV.LVSize, createLV.VGName)
	if err != nil {
		ll.Errorf("Could not create logical volume %v, error: %v", createLV, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ll.Infof("LV %s/%s created", createLV.VGName, createLV.Name)
	w.WriteHeader(http.StatusOK)
}

// RemoveLogicalVolumeHandler create LV on node
func (s *Server) RemoveLogicalVolumeHandler(w http.ResponseWriter, r *http.Request) {
	ll := logrus.WithFields(logrus.Fields{
		"component": "restServer",
		"method":    "RemoveLogicalVolumeHandler",
	})
	ll.Info("Got request")

	s.mutex.Lock()
	defer func() {
		s.mutex.Unlock()
		ll.Infof("Server Mutex was unlocked")
	}()
	ll.Info("Mutex Locked. Start processing ...")

	var removeLV lvm.LogicalVolume
	err := json.NewDecoder(r.Body).Decode(&removeLV)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if ok, _ := lvm.IsLogicalVolumeExist(removeLV.Name, removeLV.VGName); !ok { // assume that LV was removed
		ll.Infof("Logical Volume %v is not exist.", removeLV)
		w.WriteHeader(http.StatusOK)
		return
	}
	err = util.WipeFS(fmt.Sprintf("/dev/%s/%s", removeLV.VGName, removeLV.Name))
	if err != nil {
		ll.Errorf("Could not wipeFS from %s. Error: %v",
			fmt.Sprintf("/dev/%s/%s", removeLV.VGName, removeLV.Name), err)
	}
	err = lvm.RemoveLogicalVolume(removeLV.Name, removeLV.VGName)
	if err != nil {
		ll.Errorf("Could not remove logical volume %v, error: %v", removeLV, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.WriteHeader(http.StatusOK)
}

// GetLogicalVolumeHandler return LV info
func (s *Server) GetLogicalVolumeHandler(w http.ResponseWriter, r *http.Request) {
	ll := logrus.WithFields(logrus.Fields{
		"component": "restServer",
		"method":    "GetLogicalVolumeHandler",
	})
	ll.Info("Processing request")

	var lv lvm.LogicalVolume
	err := json.NewDecoder(r.Body).Decode(&lv)
	if err != nil {
		ll.Errorf("Could not decode body. Error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if ok, _ := lvm.IsLogicalVolumeExist(lv.Name, lv.VGName); !ok {
		ll.Infof("LV %s does not exist", lv.Name)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// remove all LVM staff from node
func (s *Server) RemoveLVMHandler(w http.ResponseWriter, r *http.Request) {
	ll := logrus.WithFields(logrus.Fields{
		"component": "restServer",
		"method":    "RemoveLVMHandler",
	})
	ll.Info("Got request")

	s.mutex.Lock()
	defer func() {
		s.mutex.Unlock()
		ll.Infof("Server Mutex was unlocked")
	}()
	ll.Info("Mutex Locked. Start processing ...")

	vg := lvm.VolumeGroup{
		Name: VgName,
	}
	if wasErr := vg.RemoveLVMStaff(); wasErr {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// StartRest is a function for starting REST server
func (s *Server) Start() {
	logrus.Info("Starting REST server on ", s.Port)
	s.mutex = &sync.Mutex{}
	r := mux.NewRouter()
	r.HandleFunc("/disks", s.GetDisks).Methods("GET")
	if s.HandleLVM {
		logrus.Infof("Server will handle LVM request")
		r.HandleFunc("/vg", s.GetVolumeGroupInfoHandler).Methods("GET")
		r.HandleFunc("/rlvm", s.RemoveLVMHandler).Methods("GET") // TODO: should be DELETE method
		r.HandleFunc("/lv", s.CreateLogicalVolumeHandler).Methods("PUT")
		r.HandleFunc("/lv", s.RemoveLogicalVolumeHandler).Methods("DELETE")
		r.HandleFunc("/lv", s.GetLogicalVolumeHandler).Methods("GET")
	}

	if err := http.ListenAndServe(fmt.Sprintf(":%d", s.Port), r); err != nil {
		logrus.Errorf("Failed to start server: %v", err)
		os.Exit(1)
	}
}
