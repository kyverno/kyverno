package fake

// FakeAuth providers implementation for testing, retuning true for all operations
type FakeAuth struct{}

// NewFakeAuth returns a new instance of Fake Auth that returns true for each operation
func NewFakeAuth() *FakeAuth {
	a := FakeAuth{}
	return &a
}

// CanICreate returns 'true'
func (a *FakeAuth) CanICreate(kind, namespace string) (bool, error) {
	return true, nil
}

// CanIUpdate returns 'true'
func (a *FakeAuth) CanIUpdate(kind, namespace string) (bool, error) {
	return true, nil
}

// CanIDelete returns 'true'
func (a *FakeAuth) CanIDelete(kind, namespace string) (bool, error) {
	return true, nil
}

// CanIGet returns 'true'
func (a *FakeAuth) CanIGet(kind, namespace string) (bool, error) {
	return true, nil
}
