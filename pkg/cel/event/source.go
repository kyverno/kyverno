package event

// EventSource represents the source of the event.
type EventSource string

const (
	SourceAdmissionController EventSource = "AdmissionController"
	SourceBackgroundScanner   EventSource = "BackgroundScanner"
)
