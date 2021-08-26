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

package eventing

// Event types
const (
	NormalType   = "Normal"
	WarningType  = "Warning"
	ErrorType    = "Error"
	CriticalType = "Critical"
)

// CSI events reason list
const (
	VolumeDiscovered    = "VolumeDiscovered"
	VolumeBadHealth     = "VolumeBadHealth"
	VolumeUnknownHealth = "VolumeUnknownHealth"
	VolumeGoodHealth    = "VolumeGoodHealth"
	VolumeSuspectHealth = "VolumeSuspectHealth"

	DriveDiscovered              = "DriveDiscovered"
	DriveHealthSuspect           = "DriveHealthSuspect"
	DriveHealthFailure           = "DriveHealthFailure"
	DriveHealthGood              = "DriveHealthGood"
	DriveHealthUnknown           = "DriveHealthUnknown"
	DriveStatusOnline            = "DriveStatusOnline"
	DriveStatusOffline           = "DriveStatusOffline"
	DriveRemovalFailed           = "DriveRemovalFailed"
	DriveReadyForRemoval         = "DriveReadyForRemoval"
	DriveReadyForPhysicalRemoval = "DriveReadyForPhysicalRemoval"
	DriveSuccessfullyRemoved     = "DriveSuccessfullyRemoved"
	DriveHasData                 = "DriveHasData"
	DriveClean                   = "DriveClean"
	DriveHealthOverridden        = "DriveHealthOverridden"
	DriveRemovedByForce          = "DriveRemovedByForce"

	FakeAttachInvolved = "FakeAttachInvolved"
	FakeAttachCleared  = "FakeAttachCleared"

	VolumeGroupScanFailed         = "VolumeGroupScanFailed"
	VolumeGroupScanInvolved       = "VolumeGroupScanInvolved"
	VolumeGroupReactivateInvolved = "VolumeGroupReactivateInvolved"
	VolumeGroupReactivateFailed   = "VolumeGroupReactivateFailed"
)

// CSI events SymptomCode map
const (
	SymptomCodeLabelKey = "SymptomID"

	DriveHealthFailureSymptomCode = "CSI-01"
	DriveDiscoveredSymptomCode    = "CSI-02"
	DriveStatusChangedSymptomCode = "CSI-03"
	DriveHealthGoodSymptomCode    = "CSI-04"
	FakeAttachSymptomCode         = "CSI-05"
)

var (
	reasonSymptomCodes = map[string]string{
		DriveHealthFailure:           DriveHealthFailureSymptomCode,
		DriveHealthSuspect:           DriveHealthFailureSymptomCode,
		DriveReadyForRemoval:         DriveHealthFailureSymptomCode,
		DriveReadyForPhysicalRemoval: DriveHealthFailureSymptomCode,
		DriveSuccessfullyRemoved:     DriveHealthFailureSymptomCode,
		DriveRemovalFailed:           DriveHealthFailureSymptomCode,
		DriveRemovedByForce:          DriveHealthFailureSymptomCode,

		DriveDiscovered: DriveDiscoveredSymptomCode,

		DriveStatusOffline: DriveStatusChangedSymptomCode,
		DriveStatusOnline:  DriveStatusChangedSymptomCode,

		DriveHealthGood: DriveHealthGoodSymptomCode,

		FakeAttachInvolved: FakeAttachSymptomCode,
		FakeAttachCleared:  FakeAttachSymptomCode,
	}
)

// GetReasonSymptomCodes returns const map Reason: Symptom Code
// Function was created to avoid linter issue
func GetReasonSymptomCodes() map[string]string {
	return reasonSymptomCodes
}
