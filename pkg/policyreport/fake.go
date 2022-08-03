package policyreport

func NewFake() GeneratorInterface {
	return &fakeReporter{}
}

type fakeReporter struct {
}

func (f *fakeReporter) Add(infos ...Info) {

}

func (f *fakeReporter) MapperReset(string) {

}

func (f *fakeReporter) MapperInactive(string) {

}

func (f *fakeReporter) MapperInvalidate() {

}
