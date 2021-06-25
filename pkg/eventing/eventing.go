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

// Volume event reason list
const (
	VolumeDiscovered    = "VolumeDiscovered"
	VolumeBadHealth     = "VolumeBadHealth"
	VolumeUnknownHealth = "VolumeUnknownHealth"
	VolumeGoodHealth    = "VolumeGoodHealth"
	VolumeSuspectHealth = "VolumeSuspectHealth"

	DriveDiscovered           = "DriveDiscovered"
	DriveHealthSuspect        = "DriveHealthSuspect"
	DriveHealthFailure        = "DriveHealthFailure"
	DriveHealthGood           = "DriveHealthGood"
	DriveHealthUnknown        = "DriveHealthUnknown"
	DriveStatusOnline         = "DriveStatusOnline"
	DriveStatusOffline        = "DriveStatusOffline"
	DriveReplacementFailed    = "DriveReplacementFailed"
	DriveReadyForReplacement  = "DriveReadyForReplacement"
	DriveSuccessfullyReplaced = "DriveSuccessfullyReplaced"
	DriveHasData              = "DriveHasData"
	DriveClean                = "DriveClean"
	DriveHealthOverridden     = "DriveHealthOverridden"
)
