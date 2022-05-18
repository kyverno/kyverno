package cosign

import (
	"fmt"

	"github.com/sigstore/cosign/cmd/cosign/cli/fulcio"
)

func Init() error {
	if fulcio.GetRoots() == nil {
		return fmt.Errorf("failed to initialize Fulcio roots")
	}
	return nil
}
