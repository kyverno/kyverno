package registryclient

import "github.com/kyverno/kyverno/pkg/utils/egress"

// ErrRegistryAddressBlocked marks a registry target rejected by the egress guard.
var ErrRegistryAddressBlocked = egress.ErrAddressBlocked
