package test

type TestCases []TestCase

func (tc TestCases) Errors() []TestCase {
	var errors []TestCase
	for _, test := range tc {
		if test.Err != nil {
			errors = append(errors, test)
		}
	}
	return errors
}
