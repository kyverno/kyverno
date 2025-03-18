package event

// EventFilterFunc is a function type for filtering events.
type EventFilterFunc func(event PolicyEvent) bool

func (r *eventRecorder) isFiltered(event PolicyEvent) bool {
	for _, filter := range r.filters {
		if filter(event) {
			return true
		}
	}
	return false
}
