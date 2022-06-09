package step

import (
	"github.com/kyverno/kyverno/test/e2e/framework/client"
	"github.com/onsi/ginkgo"
)

func By(message string) Step {
	return func(client.Client) {
		ginkgo.By(message)
	}
}
