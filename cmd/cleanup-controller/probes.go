package main

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/tls"
	"k8s.io/client-go/tools/cache"
)

type probes struct {
	logger          logr.Logger
	certValidator   tls.CertValidator
	caSecretSynced  cache.InformerSynced
	tlsSecretSynced cache.InformerSynced
}

func (p probes) IsReady(ctx context.Context) bool {
	if !p.caSecretSynced() || !p.tlsSecretSynced() {
		return false
	}
	valid, err := p.certValidator.ValidateCert(ctx)
	if err != nil {
		p.logger.Error(err, "failed to validate certificates")
		return false
	}
	return valid
}

func (probes) IsLive(context.Context) bool {
	return true
}
