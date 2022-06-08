package step

import (
	"github.com/kyverno/kyverno/test/e2e/framework/client"
)

type Step func(client.Client)
