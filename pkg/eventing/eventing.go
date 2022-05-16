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

// EventSeverityType type of severity for CSI events
type EventSeverityType string

// CSI events severity types
const (
	NormalType   = "Normal"
	WarningType  = "Warning"
	ErrorType    = "Error"
	CriticalType = "Critical"
)

// EventSymptomCode type of symptom codes for CSI events
type EventSymptomCode string

// CSI events symptom codes
const (
	DriveHealthFailureSymptomCode = "01"
	DriveDiscoveredSymptomCode    = "02"
	DriveStatusChangedSymptomCode = "03"
	DriveHealthGoodSymptomCode    = "04"
	FakeAttachSymptomCode         = "05"

	NoneSymptomCode = "NONE"

	SymptomCodeLabelKey = "SymptomID"
	SymptomCodePrefix   = "CSI"
)

// EventReason type of reasons for CSI events
type EventReason string

// EventDescription contains info to record CSI event
type EventDescription struct {
	reason      EventReason
	severity    EventSeverityType
	symptomCode EventSymptomCode
}

// List of all CSI events with description
var (
	DriveHealthFailure = &EventDescription{
		reason:      "DriveHealthFailure",
		severity:    ErrorType,
		symptomCode: DriveHealthFailureSymptomCode,
	}
	DriveHealthSuspect = &EventDescription{
		reason:      "DriveHealthSuspect",
		severity:    WarningType,
		symptomCode: DriveHealthFailureSymptomCode,
	}
	DriveReadyForRemoval = &EventDescription{
		reason:      "DriveReadyForRemoval",
		severity:    WarningType,
		symptomCode: DriveHealthFailureSymptomCode,
	}
	DriveReadyForPhysicalRemoval = &EventDescription{
		reason:      "DriveReadyForPhysicalRemoval",
		severity:    WarningType,
		symptomCode: DriveHealthFailureSymptomCode,
	}
	DriveSuccessfullyRemoved = &EventDescription{
		reason:      "DriveSuccessfullyRemoved",
		severity:    NormalType,
		symptomCode: DriveHealthFailureSymptomCode,
	}
	DriveRemovalFailed = &EventDescription{
		reason:      "DriveRemovalFailed",
		severity:    ErrorType,
		symptomCode: DriveHealthFailureSymptomCode,
	}
	DriveRemovedByForce = &EventDescription{
		reason:      "DriveRemovedByForce",
		severity:    WarningType,
		symptomCode: DriveHealthFailureSymptomCode,
	}

	DriveDiscovered = &EventDescription{
		reason:      "DriveDiscovered",
		severity:    NormalType,
		symptomCode: DriveDiscoveredSymptomCode,
	}

	DriveStatusOffline = &EventDescription{
		reason:      "DriveStatusOffline",
		severity:    ErrorType,
		symptomCode: DriveStatusChangedSymptomCode,
	}
	DriveStatusOnline = &EventDescription{
		reason:      "DriveStatusOnline",
		severity:    NormalType,
		symptomCode: DriveStatusChangedSymptomCode,
	}

	DriveHealthGood = &EventDescription{
		reason:      "DriveHealthGood",
		severity:    NormalType,
		symptomCode: DriveHealthGoodSymptomCode,
	}

	FakeAttachInvolved = &EventDescription{
		reason:      "FakeAttachInvolved",
		severity:    ErrorType,
		symptomCode: FakeAttachSymptomCode,
	}
	FakeAttachCleared = &EventDescription{
		reason:      "FakeAttachCleared",
		severity:    NormalType,
		symptomCode: FakeAttachSymptomCode,
	}

	VolumeDiscovered = &EventDescription{
		reason:      "VolumeDiscovered",
		severity:    NormalType,
		symptomCode: NoneSymptomCode,
	}
	VolumeBadHealth = &EventDescription{
		reason:      "VolumeBadHealth",
		severity:    WarningType,
		symptomCode: NoneSymptomCode,
	}
	VolumeUnknownHealth = &EventDescription{
		reason:      "VolumeUnknownHealth",
		severity:    WarningType,
		symptomCode: NoneSymptomCode,
	}
	VolumeGoodHealth = &EventDescription{
		reason:      "VolumeGoodHealth",
		severity:    NormalType,
		symptomCode: NoneSymptomCode,
	}
	VolumeSuspectHealth = &EventDescription{
		reason:      "VolumeSuspectHealth",
		severity:    WarningType,
		symptomCode: NoneSymptomCode,
	}

	DriveHealthUnknown = &EventDescription{
		reason:      "DriveHealthUnknown",
		severity:    WarningType,
		symptomCode: NoneSymptomCode,
	}
	DriveHasData = &EventDescription{
		reason:      "DriveHasData",
		severity:    NormalType,
		symptomCode: NoneSymptomCode,
	}
	DriveClean = &EventDescription{
		reason:      "DriveClean",
		severity:    NormalType,
		symptomCode: NoneSymptomCode,
	}
	DriveHealthOverridden = &EventDescription{
		reason:      "DriveHealthOverridden",
		severity:    WarningType,
		symptomCode: NoneSymptomCode,
	}

	VolumeGroupScanInvolved = &EventDescription{
		reason:      "VolumeGroupScanInvolved",
		severity:    NormalType,
		symptomCode: NoneSymptomCode,
	}
	VolumeGroupScanFailed = &EventDescription{
		reason:      "VolumeGroupScanFailed",
		severity:    ErrorType,
		symptomCode: NoneSymptomCode,
	}
	VolumeGroupScanNoErrors = &EventDescription{
		reason:      "VolumeGroupScanNoErrors",
		severity:    NormalType,
		symptomCode: NoneSymptomCode,
	}
	VolumeGroupScanErrorsFound = &EventDescription{
		reason:      "VolumeGroupScanErrorsFound",
		severity:    WarningType,
		symptomCode: NoneSymptomCode,
	}
	VolumeGroupReactivateInvolved = &EventDescription{
		reason:      "VolumeGroupReactivateInvolved",
		severity:    WarningType,
		symptomCode: NoneSymptomCode,
	}
	VolumeGroupReactivateFailed = &EventDescription{
		reason:      "VolumeGroupReactivateFailed",
		severity:    ErrorType,
		symptomCode: NoneSymptomCode,
	}

	WBTValueSetFailed = &EventDescription{
		reason:      "WBTValueSetFailed",
		severity:    ErrorType,
		symptomCode: NoneSymptomCode,
	}
	WBTValueRestoreFailed = &EventDescription{
		reason:      "WBTValueRestoreFailed",
		severity:    WarningType,
		symptomCode: NoneSymptomCode,
	}
	WBTConfigMapUpdateFailed = &EventDescription{
		reason:      "WBTConfigMapUpdateFailed",
		severity:    WarningType,
		symptomCode: NoneSymptomCode,
	}
)
