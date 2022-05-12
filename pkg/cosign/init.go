package cosign

import (
	"fmt"

	"github.com/sigstore/cosign/cmd/cosign/cli/fulcio"
)

func Init() error {
	certs := fulcio.GetRoots()
	if certs == nil {
		return fmt.Errorf("failed to initialize Fulcio roots")
	}

	return nil
}
