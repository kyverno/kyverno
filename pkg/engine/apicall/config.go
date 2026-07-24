package apicall

import "time"

type APICallConfiguration struct {
	maxAPICallResponseLength int64
	timeout                  time.Duration
	// enableSATokenInjection controls whether the scoped ServiceAccount token is
	// automatically injected as an Authorization: Bearer header into outbound
	// service calls that do not already set one. This is a cluster-operator
	// setting — policy authors cannot override it. Defaults to false.
	enableSATokenInjection bool
}

func NewAPICallConfiguration(maxLen int64, timeout time.Duration, enableSATokenInjection bool) APICallConfiguration {
	return APICallConfiguration{
		maxAPICallResponseLength: maxLen,
		timeout:                  timeout,
		enableSATokenInjection:   enableSATokenInjection,
	}
}

func (c APICallConfiguration) GetTimeout() time.Duration {
	return c.timeout
}
