package event

func NewFake() Interface {
	return &fakeEventGenerator{}
}

type fakeEventGenerator struct{}

func (f *fakeEventGenerator) Add(infoList ...Info) {
}
