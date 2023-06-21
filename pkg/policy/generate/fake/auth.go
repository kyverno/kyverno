package fake

import "context"

// FakeAuth providers implementation for testing, retuning true for all operations
type FakeAuth struct{}

// NewFakeAuth returns a new instance of Fake Auth that returns true for each operation
func NewFakeAuth() *FakeAuth {
	a := FakeAuth{}
	return &a
}

// CanICreate returns 'true'
func (a *FakeAuth) CanICreate(_ context.Context, kind, namespace, sub string) (bool, error) {
	return true, nil
}

// CanIUpdate returns 'true'
func (a *FakeAuth) CanIUpdate(_ context.Context, kind, namespace, sub string) (bool, error) {
	return true, nil
}

// CanIDelete returns 'true'
func (a *FakeAuth) CanIDelete(_ context.Context, kind, namespace, sub string) (bool, error) {
	return true, nil
}

// CanIGet returns 'true'
func (a *FakeAuth) CanIGet(_ context.Context, kind, namespace, sub string) (bool, error) {
	return true, nil
}
