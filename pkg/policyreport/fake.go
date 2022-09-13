package policyreport

func NewFake() Generator {
	return &fakeReporter{}
}

type fakeReporter struct{}

func (f *fakeReporter) Cleanup() chan string {
	return nil
}

func (f *fakeReporter) Run(int, <-chan struct{}) {
}

func (f *fakeReporter) Add(infos ...Info) {
}

func (f *fakeReporter) MapperReset(string) {
}

func (f *fakeReporter) MapperInactive(string) {
}

func (f *fakeReporter) MapperInvalidate() {
}
