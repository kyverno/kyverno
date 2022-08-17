package toggle

import (
	"os"
	"strconv"
)

const (
	AutogenInternalsFlagName    = "autogenInternals"
	AutogenInternalsDescription = "Enables autogen internal policies. When this is 'true' policy rules should not be mutated."
	AutogenInternalsEnvVar      = "FLAG_AUTOGEN_INTERNALS"
	DefaultAutogenInternals     = false
)

var autogenInternals *bool

func getBool(in string) (*bool, error) {
	if in == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(in)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func AutogenInternalsFlag(in string) error {
	if value, err := getBool(in); err != nil {
		return err
	} else {
		autogenInternals = value
		return nil
	}
}

func AutogenInternals() bool {
	if autogenInternals != nil {
		return *autogenInternals
	}
	if value, err := getBool(os.Getenv(AutogenInternalsEnvVar)); err == nil && value != nil {
		return *value
	}
	return DefaultAutogenInternals
}
