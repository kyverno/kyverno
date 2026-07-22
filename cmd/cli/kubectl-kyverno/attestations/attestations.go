package attestations

import (
	"encoding/json"
	"fmt"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/path"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/pkg/image/verifiers/local"
)

// Load reads and validates the local attestation predicate files declared in a test and
// returns a local.Provider that serves them during image verification. Predicate file paths
// are resolved relative to testDir (or read from the git filesystem when fs is non-nil).
func Load(fs billy.Filesystem, testDir string, isGit bool, entries []v1alpha1.TestAttestation) (*local.Provider, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	provider := local.NewProvider()
	for i, entry := range entries {
		predicatePath := path.GetFullPaths([]string{entry.PredicateFile}, testDir, isGit)[0]
		data, err := common.ReadFile(fs, predicatePath)
		if err != nil {
			return nil, fmt.Errorf("attestations[%d]: failed to read predicate file %s: %w", i, predicatePath, err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("attestations[%d]: predicate file %s is empty", i, predicatePath)
		}
		var predicate map[string]any
		if err := json.Unmarshal(data, &predicate); err != nil {
			return nil, fmt.Errorf("attestations[%d]: predicate file %s contains invalid JSON: %w", i, predicatePath, err)
		}
		if len(predicate) == 0 {
			return nil, fmt.Errorf("attestations[%d]: predicate file %s contains an empty JSON object", i, predicatePath)
		}
		provider.Add(entry.Image, entry.PredicateType, predicate)
	}
	return provider, nil
}
