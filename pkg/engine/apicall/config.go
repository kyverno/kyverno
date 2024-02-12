package apicall

type APICallConfiguration struct {
	maxAPICallResponseLength int64
}

func NewAPICallConfiguration(maxLen int64) APICallConfiguration {
	return APICallConfiguration{
		maxAPICallResponseLength: maxLen,
	}
}
