package apicall

import "time"

type APICallConfiguration struct {
	maxAPICallResponseLength int64
	timeout                  time.Duration
}

func NewAPICallConfiguration(maxLen int64, timeout time.Duration) APICallConfiguration {
	return APICallConfiguration{
		maxAPICallResponseLength: maxLen,
		timeout:                  timeout,
	}
}

func (c APICallConfiguration) GetTimeout() time.Duration {
	return c.timeout
}
