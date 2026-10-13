package sigstoreguard

import "github.com/theupdateframework/go-tuf/v2/metadata"

// authenticatedTargetAbsent proves absence using only metadata already verified
// by go-tuf. A lookup error alone cannot establish absence: the same not-found
// error is returned when the delegation traversal limit is exhausted.
func authenticatedTargetAbsent(client *legacyTUFClient, name string) bool {
	trusted := client.GetTrustedMetadataSet()
	pending := []string{metadata.TARGETS}
	visited := map[string]bool{}
	for len(pending) != 0 {
		role := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if visited[role] {
			continue
		}
		targets := trusted.Targets[role]
		if targets == nil {
			// A missing, invalid, or not-yet-visited delegation leaves the
			// lookup incomplete, even if its siblings were authenticated.
			return false
		}
		if _, declared := targets.Signed.Targets[name]; declared {
			return false
		}
		visited[role] = true
		if targets.Signed.Delegations == nil {
			continue
		}
		// Match go-tuf's ordered depth-first traversal, including succinct
		// bins and terminating delegations that prohibit backtracking.
		children := targets.Signed.Delegations.GetRolesForTarget(name)
		for i, child := range children {
			if child.Terminating {
				pending = nil
				children = children[:i+1]
				break
			}
		}
		for i := len(children) - 1; i >= 0; i-- {
			pending = append(pending, children[i].Name)
		}
	}
	return true
}
