package scheduler

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	manifestBackupName = "kube-scheduler.backup_csi-baremetal"
	manifestPerm       = 0600
)

// NewManifestPatcher initialize and return ManifestPatcher instance
func NewManifestPatcher(logger *logrus.Logger) *ManifestPatcher {
	return &ManifestPatcher{
		logger: logger.WithField("component", "Scheduler patcher"),
	}
}

// ManifestPatcherConfig configuration for ManifestPathcer
type ManifestPatcherConfig struct {
	ManifestPath     string
	SourceConfigPath string
	TargetConfigPath string
	SourcePolicyPath string
	TargetPolicyPath string
	BackupPath       string
}

// ManifestPatcher try to apply patches to kube-scheduler manifests
type ManifestPatcher struct {
	logger *logrus.Entry
}

// Apply will start Patch process if config and policy files are exist
func (mp *ManifestPatcher) Apply(config ManifestPatcherConfig) error {
	if checkFileExist(config.SourceConfigPath) || checkFileExist(config.SourcePolicyPath) {
		mp.logger.Info("trying to patch scheduler manifest")
		return mp.Patch(config)
	}
	mp.logger.Info("trying to restore scheduler manifest")
	return mp.Restore(config)
}

// Patch try to edit scheduler's manifest to enable extender
func (mp *ManifestPatcher) Patch(config ManifestPatcherConfig) error {
	manifestContent, err := ioutil.ReadFile(config.ManifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest file %s: %s", config.ManifestPath, err.Error())
	}
	var schedulerPod corev1.Pod
	err = yaml.Unmarshal(manifestContent, &schedulerPod)
	if err != nil {
		return fmt.Errorf("failed to unmarshal scheduler pod config: %s", err.Error())
	}

	filesChanged, err := mp.checkOrCopyConfigurationFiles(config)
	if err != nil {
		return fmt.Errorf("failed to copy config files: %s", err.Error())
	}

	volumesChanged := mp.checkOrAddVolumes(config, &schedulerPod)
	containerChanged := mp.checkOrModifyContainerSpec(config, &schedulerPod)

	podManifestChanged := volumesChanged || containerChanged

	if !(podManifestChanged || filesChanged) {
		mp.logger.Info("scheduler manifest already patched")
		return nil
	}

	if podManifestChanged {
		err = ioutil.WriteFile(getManifestBackupPath(config), manifestContent, manifestPerm)
		if err != nil {
			return fmt.Errorf("failed to create config backup: %s", err.Error())
		}
		manifestContent, err = yaml.Marshal(&schedulerPod)
		if err != nil {
			return fmt.Errorf("failed to marshal scheduler pod manifes: %s", err.Error())
		}
	}
	// we need to write manifest file always to trigger scheduler's pod restart
	// when config or policy files are changed
	err = ioutil.WriteFile(config.ManifestPath, manifestContent, manifestPerm)
	if err != nil {
		return fmt.Errorf("failed to write scheduler manifest: %s", err.Error())
	}
	mp.logger.Infof("scheduler manifest %s patched", config.ManifestPath)
	return nil
}

// Restore try to restore scheduler's manifest from backup
func (mp *ManifestPatcher) Restore(config ManifestPatcherConfig) error {
	backupFilePath := getManifestBackupPath(config)
	dataFromBackup, err := ioutil.ReadFile(backupFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			mp.logger.Info("backup file not found, skip restore")
			return nil
		}
		return fmt.Errorf("failed to read backup file %s: %s", backupFilePath, err.Error())
	}
	err = ioutil.WriteFile(config.ManifestPath, dataFromBackup, manifestPerm)
	if err != nil {
		return fmt.Errorf("failed to read backup file %s: %s", backupFilePath, err.Error())
	}
	err = os.Remove(backupFilePath)
	if err != nil {
		// ignore error
		mp.logger.Errorf("failed to remove backup file %s: %s", backupFilePath, err.Error())
	}
	mp.logger.Infof("scheduler manifest %s restored", config.ManifestPath)
	return nil
}

func (mp *ManifestPatcher) checkOrAddVolumes(config ManifestPatcherConfig, pod *corev1.Pod) bool {
	var changes bool
	var configVolumeFound bool
	var policyVolumeFound bool

	for _, vol := range pod.Spec.Volumes {
		if vol.HostPath == nil {
			continue
		}
		switch vol.HostPath.Path {
		case config.TargetConfigPath:
			configVolumeFound = true
			mp.logger.Debug("config volume already exist for scheduler POD")
		case config.TargetPolicyPath:
			policyVolumeFound = true
			mp.logger.Debug("policy volume already exist for scheduler POD")
		}
	}
	if !configVolumeFound {
		changes = true
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			createVolumeSpec("scheduler-config", config.TargetConfigPath))
	}
	if !policyVolumeFound {
		changes = true
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			createVolumeSpec("scheduler-policy", config.TargetPolicyPath))
	}
	return changes
}

func (mp *ManifestPatcher) checkOrModifyContainerSpec(config ManifestPatcherConfig, pod *corev1.Pod) bool {
	var changes bool
	var configMountFound bool
	var policyMountFound bool
	var commandArgFound bool

	for i := 0; i < len(pod.Spec.Containers); i++ {
		container := pod.Spec.Containers[i]
		if container.Name != "kube-scheduler" {
			continue
		}
		for _, mount := range container.VolumeMounts {
			switch mount.MountPath {
			case config.TargetConfigPath:
				configMountFound = true
				mp.logger.Debug("config mount already exist for scheduler POD")
			case config.TargetPolicyPath:
				policyMountFound = true
				mp.logger.Debug("policy mount already exist for scheduler POD")
			}
		}
		if !configMountFound {
			changes = true
			container.VolumeMounts = append(container.VolumeMounts,
				createMountSpec("scheduler-config", config.TargetConfigPath))
		}
		if !policyMountFound {
			changes = true
			container.VolumeMounts = append(container.VolumeMounts,
				createMountSpec("scheduler-policy", config.TargetPolicyPath))
		}
		for _, commandArg := range container.Command {
			if strings.Contains(commandArg, "--config") {
				commandArgFound = true
			}
		}
		if !commandArgFound {
			changes = true
			container.Command = append(container.Command, fmt.Sprintf("--config=%s", config.TargetConfigPath))
		}
		pod.Spec.Containers[i] = container
	}
	return changes
}

func (mp *ManifestPatcher) checkOrCopyConfigurationFiles(config ManifestPatcherConfig) (bool, error) {
	configNotChanged, err := isFilesEqual(config.SourceConfigPath, config.TargetConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to config files: %s", err.Error())
	}
	policyNotChanged, err := isFilesEqual(config.SourcePolicyPath, config.TargetPolicyPath)
	if err != nil {
		return false, fmt.Errorf("failed to policy files: %s", err.Error())
	}
	if configNotChanged && policyNotChanged {
		mp.logger.Info("config and policy files are not changed")
		return false, nil
	}
	if !configNotChanged {
		err = copyFiles(config.SourceConfigPath, config.TargetConfigPath)
		if err != nil {
			return false, fmt.Errorf("failed to write config file %s: %s", config.TargetConfigPath, err.Error())
		}
		mp.logger.Info("config file updated")
	}
	if !policyNotChanged {
		err = copyFiles(config.SourcePolicyPath, config.TargetPolicyPath)
		if err != nil {
			return false, fmt.Errorf("failed to write policy file %s: %s", config.TargetPolicyPath, err.Error())
		}
		mp.logger.Info("policy file updated")
	}
	return true, nil
}

func createVolumeSpec(name, path string) corev1.Volume {
	typeFile := corev1.HostPathFile
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: path,
				Type: &typeFile,
			},
		},
	}
}

func createMountSpec(name, mountpath string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,
		ReadOnly:  true,
		MountPath: mountpath,
	}
}

func getManifestBackupPath(config ManifestPatcherConfig) string {
	return config.BackupPath + manifestBackupName
}

func isFilesEqual(src, target string) (bool, error) {
	srcContent, err := ioutil.ReadFile(src)
	if err != nil {
		return false, fmt.Errorf("failed to read source file %s: %s", src, err.Error())
	}
	targetContent, err := ioutil.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read target file %s: %s", target, err.Error())
	}
	return bytes.Equal(srcContent, targetContent), nil
}

func copyFiles(src, dst string) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	dstFolder := path.Dir(dst)
	_, err = os.Stat(dstFolder)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		err := os.MkdirAll(dstFolder, 0700)
		if err != nil {
			return err
		}
	}
	err = ioutil.WriteFile(dst, data, manifestPerm)
	if err != nil {
		return err
	}
	return nil
}

func checkFileExist(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
