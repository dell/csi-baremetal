/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package featureconfig

import "sync"

const (
	// FeatureACReservation store name for ACReservation feature
	FeatureACReservation = "ACReservation"
	// FeatureNodeIDFromAnnotation store name for NodeIDFromAnnotation feature
	FeatureNodeIDFromAnnotation = "NodeIDFromAnnotation"
	// FeatureExtenderWaitForResources if enabled extender will do few retries if no capacity found
	FeatureExtenderWaitForResources = "ExtenderWaitForResources"
)

// FeatureChecker is a "read" interface for FeatureConfig
type FeatureChecker interface {
	// IsEnabled check if features is enabled
	IsEnabled(name string) bool
	// List list all features
	List() []string
}

// FeatureConfigurator is a "write" interface for FeatureConfig
type FeatureConfigurator interface {
	FeatureChecker
	// Update adds new feature or update existing
	Update(name string, enabled bool)
}

// NewFeatureConfig returns new instance of FeatureConfig
func NewFeatureConfig() *FeatureConfig {
	return &FeatureConfig{
		lock:     sync.RWMutex{},
		features: make(map[string]bool),
	}
}

// FeatureConfig store features flags
type FeatureConfig struct {
	features map[string]bool
	lock     sync.RWMutex
}

// IsEnabled is implementation of FeatureChecker interface
func (f *FeatureConfig) IsEnabled(name string) bool {
	f.lock.RLock()
	defer f.lock.RUnlock()
	enabled, exist := f.features[name]
	return exist && enabled
}

// List is implementation of FeatureChecker interface
func (f *FeatureConfig) List() []string {
	f.lock.RLock()
	defer f.lock.RUnlock()
	featureNames := make([]string, 0, len(f.features))
	for f := range f.features {
		featureNames = append(featureNames, f)
	}
	return featureNames
}

// Update is implementation of FeatureConfigurator interface
func (f *FeatureConfig) Update(name string, enabled bool) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.features[name] = enabled
}
