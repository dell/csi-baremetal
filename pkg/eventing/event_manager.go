package eventing

// EventManager is wrapper to manipulate EventDescription
type EventManager struct{}

// GetReason returns event reason
func (e *EventManager) GetReason(event *EventDescription) string {
	return string(event.reason)
}

// GetSeverity returns event severity
func (e *EventManager) GetSeverity(event *EventDescription) string {
	return string(event.severity)
}

// GetLabels return labels for event or nil it has no ones
func (e *EventManager) GetLabels(event *EventDescription) map[string]string {
	if event.symptomCode == NoneSymptomCode {
		return nil
	}

	return constructEventLabels(event)
}

// constructEventLabels return map in the following format
// {SymptomID: CSI-XX}
func constructEventLabels(event *EventDescription) map[string]string {
	symptomID := SymptomCodePrefix + "-" + event.symptomCode
	return map[string]string{SymptomCodeLabelKey: string(symptomID)}
}

// GenerateFake returns filled EventDescription
// Test function
func (e *EventManager) GenerateFake() *EventDescription {
	return &EventDescription{
		reason:      "Fake-reason",
		severity:    NormalType,
		symptomCode: NoneSymptomCode,
	}
}

// GenerateFakeWithLabel returns filled EventDescription contains symptom code label
// Test function
func (e *EventManager) GenerateFakeWithLabel() *EventDescription {
	return &EventDescription{
		reason:      "Fake-reason",
		severity:    NormalType,
		symptomCode: "Fake-symptom-code",
	}
}
