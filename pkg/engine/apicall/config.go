package apicall

import "time"

// DefaultAPICallTimeout is the default timeout for external API calls
const DefaultAPICallTimeout = 30 * time.Second

type APICallConfiguration struct {
	maxAPICallResponseLength int64
	timeout                  time.Duration
}

func NewAPICallConfiguration(maxLen int64) APICallConfiguration {
	return APICallConfiguration{
		maxAPICallResponseLength: maxLen,
		timeout:                  DefaultAPICallTimeout,
	}
}

func NewAPICallConfigurationWithTimeout(maxLen int64, timeout time.Duration) APICallConfiguration {
	return APICallConfiguration{
		maxAPICallResponseLength: maxLen,
		timeout:                  timeout,
	}
}

func (c APICallConfiguration) GetTimeout() time.Duration {
	if c.timeout == 0 {
		return DefaultAPICallTimeout
	}
	return c.timeout
}
