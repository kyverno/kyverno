# PR branch triage (DRY RUN) — kyverno/kyverno base=main

| Category | Count | Proposed label | Action |
|---|---|---|---|
| LEGACY_ONLY | 95 | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| MIXED | 69 | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| CEL_ONLY | 74 | `type_cel` | Keep on main |
| SHARED_ONLY | 96 | `type_shared` | Keep on main (verify after legacy removal lands) |
| REVIEW-MIGRATION | 5 | _(none — human review)_ | Keep on main — requires human confirmation (migration-grace, see §1.5) |

**No PRs were modified. This report is read-only. Labels are proposed, not applied (see `apply-labels.sh --execute`). `probe` is `SKIPPED` unless `--probe-rebase` was passed; the automated-retarget gate requires `category=LEGACY_ONLY && override!=KEEP_MAIN && probe=OK`.**

## LEGACY_ONLY (95) — label `type_legacy`

| PR | Author | Title | Draft | Fork | CanModify | Mergeable | Files (L/C/S) | Content signal | Probe | Proposed label | Proposed Action |
|---|---|---|---|---|---|---|---|---|---|---|---|
| [#17525](https://github.com/kyverno/kyverno/pull/17525) | @badnikhil | fix: honour the caller context in notary attestation verification |  | Y | Y | MERGEABLE | 4/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17502](https://github.com/kyverno/kyverno/pull/17502) | @KR-Ravindra | fix: improve generate rule immutability error message | Y | Y | Y | MERGEABLE | 2/0/0 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17462](https://github.com/kyverno/kyverno/pull/17462) | @itsvishalyadav | fix(exceptions): honour the operations field when matching PolicyExcep |  | Y | Y | MERGEABLE | 2/0/0 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17409](https://github.com/kyverno/kyverno/pull/17409) | @AftAb-25 | fix: correct contextSize accounting in ReplaceContextEntry |  | Y | Y | MERGEABLE | 2/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17397](https://github.com/kyverno/kyverno/pull/17397) | @AftAb-25 | fix: prevent pivot mutation in FindAndShiftReferences loop |  | Y | Y | MERGEABLE | 2/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17394](https://github.com/kyverno/kyverno/pull/17394) | @itsvishalyadav | fix(apicall): allow Service APICalls in namespaced policies |  | Y | Y | MERGEABLE | 1/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17366](https://github.com/kyverno/kyverno/pull/17366) | @e-esakman | fix(anchor): classify wrapped anchor errors by type |  | Y | Y | MERGEABLE | 4/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17355](https://github.com/kyverno/kyverno/pull/17355) | @AftAb-25 | fix: nil pointer dereference in validation engine on UPDATE requests |  | Y | Y | MERGEABLE | 1/0/0 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17290](https://github.com/kyverno/kyverno/pull/17290) | @waterWang | fix: anchor variable validation regexes to prevent bypass via substrin |  | Y | Y | MERGEABLE | 4/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17286](https://github.com/kyverno/kyverno/pull/17286) | @ranyhb | fix: recompute and retry mutate existing on target conflict |  | Y | Y | MERGEABLE | 8/0/1 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17279](https://github.com/kyverno/kyverno/pull/17279) | @vishalmore90 | fix(pkg/policy): use RetryOnConflict for unlabelDownstream |  | Y | Y | MERGEABLE | 2/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17243](https://github.com/kyverno/kyverno/pull/17243) | @waterWang | fix: evaluate validate.deny on image-verify cache hit (#17173) |  | Y | Y | MERGEABLE | 1/0/0 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17218](https://github.com/kyverno/kyverno/pull/17218) | @pujitha24 | fix(cli): treat expected-fail patchedResources diff as a passing test |  | Y | Y | MERGEABLE | 4/0/3 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17183](https://github.com/kyverno/kyverno/pull/17183) | @cavemansatyn-design | fix(cel): honor allowExistingViolations on updates |  | Y | Y | MERGEABLE | 2/0/0 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17171](https://github.com/kyverno/kyverno/pull/17171) | @vishalmore90 | [Bug] Support generateName in resource filter evaluations during admis |  | Y | Y | MERGEABLE | 2/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17163](https://github.com/kyverno/kyverno/pull/17163) | @sagarkhandagre998 | fix: write CLI 'fix' output files with 0600 permissions |  | Y | Y | MERGEABLE | 4/0/0 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17129](https://github.com/kyverno/kyverno/pull/17129) | @omlahore | fix(engine): guard non-string SubstituteAll results against unchecked  |  | Y | Y | MERGEABLE | 6/0/2 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17128](https://github.com/kyverno/kyverno/pull/17128) | @vishalmore90 | fix: implement bounded worker pools for webhook async tasks  |  | Y | Y | MERGEABLE | 4/0/4 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17123](https://github.com/kyverno/kyverno/pull/17123) | @vishalmore90 | [Bug] Unbounded goroutines in policy metrics controller |  | Y | Y | MERGEABLE | 1/0/3 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17057](https://github.com/kyverno/kyverno/pull/17057) | @CodingRI | Refactor/generate validation offline mode |  | Y | Y | MERGEABLE | 4/0/0 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17056](https://github.com/kyverno/kyverno/pull/17056) | @khanskr | fix(validation): guard wildcard deny condition key type assertion |  | Y | Y | MERGEABLE | 2/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17055](https://github.com/kyverno/kyverno/pull/17055) | @CodingRI | refactor: use error unwrapping for anchor error matching |  | Y | Y | MERGEABLE | 3/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17046](https://github.com/kyverno/kyverno/pull/17046) | @omlahore | fix(imageverify): reject non-string attestation fields instead of pani |  | Y | Y | MERGEABLE | 5/0/0 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17028](https://github.com/kyverno/kyverno/pull/17028) | @CodingRI | Feat/extend shared informer factory dynamic fallback |  | Y | Y | MERGEABLE | 2/0/4 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17002](https://github.com/kyverno/kyverno/pull/17002) | @CodingRI | fix:implement real readiness probe via TLS cert validation |  | Y | Y | MERGEABLE | 2/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#17000](https://github.com/kyverno/kyverno/pull/17000) | @SanthanCH | feat: allow imageRegistry context errors to be caught |  | Y | Y | MERGEABLE | 5/0/13 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16988](https://github.com/kyverno/kyverno/pull/16988) | @SashaMIT | fix(apicall): apply httpBlocklist/allowlist to ServiceCall path |  | Y | Y | CONFLICTING | 4/0/1 | LEGACY_CONTENT | CONFLICT | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16955](https://github.com/kyverno/kyverno/pull/16955) | @ArneshBanerjee | fix(engine): guard shallow substitution against non-string values |  | Y | Y | MERGEABLE | 2/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16897](https://github.com/kyverno/kyverno/pull/16897) | @CodexRaunak | Validate cleanup-controller TLS certificates in readiness probes |  | Y | Y | MERGEABLE | 3/0/3 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16896](https://github.com/kyverno/kyverno/pull/16896) | @aryanghai12 | fix: guard type assertion in shallow variable substitution to prevent  |  | Y | Y | MERGEABLE | 2/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16886](https://github.com/kyverno/kyverno/pull/16886) | @beep-boopp | fix: apply --registryCredentialHelpers when pod has imagePullSecrets |  | Y | Y | CONFLICTING | 6/0/6 | LEGACY_CONTENT | CONFLICT | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16879](https://github.com/kyverno/kyverno/pull/16879) | @jenting | fix: cloneList sync deletes all targets when one source is deleted |  | Y | Y | MERGEABLE | 4/0/0 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16859](https://github.com/kyverno/kyverno/pull/16859) | @bhuvan-somisetty | fix(engine): safely enforce maxAPICallResponseLength without nil point |  | Y | Y | MERGEABLE | 2/0/0 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16830](https://github.com/kyverno/kyverno/pull/16830) | @akshita317 | fix: correct typos in log messages, CLI help text and comments |  | Y | Y | MERGEABLE | 1/0/3 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16829](https://github.com/kyverno/kyverno/pull/16829) | @karthik120710 | add validation primitive type in patten validation |  | Y | Y | MERGEABLE | 2/0/0 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16816](https://github.com/kyverno/kyverno/pull/16816) | @bhuvan-somisetty | fix: avoid nil pointer panic verifying sigstore bundle attestations wi |  | Y | Y | MERGEABLE | 2/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16813](https://github.com/kyverno/kyverno/pull/16813) | @KartikSuryavanshi | feat: add User Namespaces support to PSS require-run-as-nonroot polici |  | Y | Y | MERGEABLE | 5/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16812](https://github.com/kyverno/kyverno/pull/16812) | @KartikSuryavanshi | fix: deduplicate generateExisting UpdateRequests and fix hot-loop retr |  | Y | Y | MERGEABLE | 6/0/4 | LEGACY_CONTENT | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16810](https://github.com/kyverno/kyverno/pull/16810) | @akshita317 | fix: correct ephemeralContainers SELinux paths and runAsNonRoot allowe |  | Y | Y | MERGEABLE | 1/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16801](https://github.com/kyverno/kyverno/pull/16801) | @bhuvan-somisetty | fix(policy): support wildcard matchLabels when listing background poli |  | Y | Y | MERGEABLE | 3/0/0 | NEUTRAL | OK | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16794](https://github.com/kyverno/kyverno/pull/16794) | @falloficaruss | feat: add registry credential support to cleanup-controller |  | Y | Y | MERGEABLE | 1/0/2 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16763](https://github.com/kyverno/kyverno/pull/16763) | @rx18-eng | fix(pss): support the Host Probes / Lifecycle Hooks control and honor  | Y | Y | Y | MERGEABLE | 6/0/1 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16754](https://github.com/kyverno/kyverno/pull/16754) | @bhuvan-somisetty | fix(cosign): resolve bundle artifactType on fallback tag registries |  | Y | Y | CONFLICTING | 1/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16746](https://github.com/kyverno/kyverno/pull/16746) | @bhuvan-somisetty | fix(engine): set default Content-Type header to application/json for A |  | Y | Y | MERGEABLE | 2/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16690](https://github.com/kyverno/kyverno/pull/16690) | @7se7en72025 | fix: deduplicate redundant SubjectAccessReview checks during policy va |  | Y | Y | MERGEABLE | 9/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16672](https://github.com/kyverno/kyverno/pull/16672) | @raus7n | perf: cache default manifest verification config |  | Y | Y | MERGEABLE | 1/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16660](https://github.com/kyverno/kyverno/pull/16660) | @harinandhreddy0411 | feat(background-controller): enable prometheus workqueue metrics |  | Y | Y | CONFLICTING | 1/0/1 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16647](https://github.com/kyverno/kyverno/pull/16647) | @Goutham-Annem | test: add unit tests for ExpandStaticKeys in pkg/engine/internal |  | Y | Y | MERGEABLE | 1/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16585](https://github.com/kyverno/kyverno/pull/16585) | @ShubhamArora073 | feat: warn on wildcard policy creation/update | Y | Y | Y | CONFLICTING | 3/0/1 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16584](https://github.com/kyverno/kyverno/pull/16584) | @ShubhamArora073 | fix: suppress event broadcaster logs at low verbosity | Y | Y | Y | MERGEABLE | 1/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16552](https://github.com/kyverno/kyverno/pull/16552) | @tendinginfinity24 | Feat/globalcontext name lookup |  | Y | Y | CONFLICTING | 1/0/9 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16448](https://github.com/kyverno/kyverno/pull/16448) | @viktor42b | fix(webhook): bound admission report creation goroutines with a timeou |  | Y | Y | CONFLICTING | 2/0/4 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16400](https://github.com/kyverno/kyverno/pull/16400) | @om7057 | fix: bypass empty digest check on unmutated verified images |  | Y | Y | MERGEABLE | 2/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16368](https://github.com/kyverno/kyverno/pull/16368) | @sagarkhandagre998 | fix(operator): support cross-type resource quantity comparison in Equa |  | Y | Y | MERGEABLE | 4/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16363](https://github.com/kyverno/kyverno/pull/16363) | @risjai | fix(engine): make Equals/NotEquals symmetric for string vs numeric ope |  | Y | Y | MERGEABLE | 7/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16355](https://github.com/kyverno/kyverno/pull/16355) | @santhil-cyber | Fix typo in Loader interface documentation |  | Y | Y | MERGEABLE | 1/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16306](https://github.com/kyverno/kyverno/pull/16306) | @TheRealNoob | feat(charts): support map structure for customPolicies in kyverno-poli |  | Y | Y | MERGEABLE | 3/0/1 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16259](https://github.com/kyverno/kyverno/pull/16259) | @harinandhreddy0411 | fix: safely handle missing jmespath keys to allow logical fallbacks |  | Y | Y | MERGEABLE | 2/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16243](https://github.com/kyverno/kyverno/pull/16243) | @karthikmanam | feat: support local attestations in kyverno test |  | Y | Y | CONFLICTING | 9/0/11 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16230](https://github.com/kyverno/kyverno/pull/16230) | @lucchmielowski | fix(registry): preserve global keychain for image verification |  | Y | Y | CONFLICTING | 11/0/8 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16163](https://github.com/kyverno/kyverno/pull/16163) | @ObaidAbdullah16 | test: fix kyverno-policies default values filename |  | Y | Y | MERGEABLE | 1/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16125](https://github.com/kyverno/kyverno/pull/16125) | @jakharmonika364 | feat: allow HTTP headers and caBundle in ServiceCall to reference Secr |  | Y | Y | CONFLICTING | 13/0/18 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16122](https://github.com/kyverno/kyverno/pull/16122) | @Karthikk-18 | Test : add unit tests for buildPolicywithDeletedRules |  | Y | Y | MERGEABLE | 1/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16111](https://github.com/kyverno/kyverno/pull/16111) | @CygnusMaximillian | feat: add admission warning for wildcard policies |  | Y | Y | CONFLICTING | 2/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16082](https://github.com/kyverno/kyverno/pull/16082) | @aaa-aashna | fix(imageVerify): preserve trusted material for bundle verification |  | Y | Y | CONFLICTING | 1/0/1 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16069](https://github.com/kyverno/kyverno/pull/16069) | @AftAb-25 | fix: replace brittle strings.Contains with errors.As in anchor error m |  | Y | Y | MERGEABLE | 3/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16047](https://github.com/kyverno/kyverno/pull/16047) | @jakharmonika364 | fix(apicall): add SSRF blocklist/allowlist enforcement for service cal |  | Y | Y | CONFLICTING | 4/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#16019](https://github.com/kyverno/kyverno/pull/16019) | @jakharmonika364 | fix(apicall): add --enableSATokenInjection controller flag to control  |  | Y | Y | CONFLICTING | 5/0/8 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15980](https://github.com/kyverno/kyverno/pull/15980) | @jakharmonika364 | fix: enable Pod Security Standards relaxation for user namespace pods  |  | Y | Y | MERGEABLE | 2/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15951](https://github.com/kyverno/kyverno/pull/15951) | @pierluigilenoci | feat: add wildcard support for policyName in PolicyException |  | Y | Y | MERGEABLE | 1/0/8 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15938](https://github.com/kyverno/kyverno/pull/15938) | @itvi-1234 | fix(engine): replace `context.TODO()` with propagated `context(ctx`) i |  | Y | Y | CONFLICTING | 13/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15918](https://github.com/kyverno/kyverno/pull/15918) | @itvi-1234 | fix(pkg) : Fixed Typos in `validate_manifest.go` |  | Y | Y | MERGEABLE | 1/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15907](https://github.com/kyverno/kyverno/pull/15907) | @aabhinavvvvvvv | fix(engine): checkpoint JSON context in validateOldObject to prevent f |  | Y | Y | CONFLICTING | 6/0/2 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15905](https://github.com/kyverno/kyverno/pull/15905) | @itvi-1234 | fix(engine): avoid runtime panic from unsafe type assertion |  | Y | Y | MERGEABLE | 2/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15869](https://github.com/kyverno/kyverno/pull/15869) | @Kunalbehbud | fix(cli): prevent panic on webhook matchConditions in mock mode |  | Y | Y | MERGEABLE | 2/0/4 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15854](https://github.com/kyverno/kyverno/pull/15854) | @alliasgher | chore: migrate cert manager to shared kyverno/pkg/certmanager |  | Y | Y | CONFLICTING | 1/0/7 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15841](https://github.com/kyverno/kyverno/pull/15841) | @utafrali | fix NPE when validating ClusterPolicy with matchConditions offline |  | Y | Y | MERGEABLE | 2/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15795](https://github.com/kyverno/kyverno/pull/15795) | @Rama542 | Fix event broadcasting log verbosity (#9581) |  | Y | Y | MERGEABLE | 1/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15791](https://github.com/kyverno/kyverno/pull/15791) | @nabutabu | change print marker and add default to failurepolicy |  | Y | Y | MERGEABLE | 4/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15760](https://github.com/kyverno/kyverno/pull/15760) | @jakharmonika364 | feat: add catchError support to imageRegistry context variable |  | Y | Y | CONFLICTING | 5/0/4 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15742](https://github.com/kyverno/kyverno/pull/15742) | @MatiasRoje | fix: resolve mutate.targets core resources correctly in load_target |  | Y | Y | MERGEABLE | 2/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15731](https://github.com/kyverno/kyverno/pull/15731) | @atharrva01 | fix(cleanup-controller): implement real TLS health probes |  | Y | Y | CONFLICTING | 2/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15708](https://github.com/kyverno/kyverno/pull/15708) | @jakharmonika364 | fix(cli): correctly report test failures for excluded resources |  | Y | Y | CONFLICTING | 7/0/11 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15548](https://github.com/kyverno/kyverno/pull/15548) | @nishanthreddydd | use NamespaceLister for CEL validation namespace lookups |  | Y | Y | MERGEABLE | 13/0/6 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15460](https://github.com/kyverno/kyverno/pull/15460) | @lucchmielowski | fix: failurePolicy not working for image verification. | Y | Y | Y | MERGEABLE | 2/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15458](https://github.com/kyverno/kyverno/pull/15458) | @atharrva01 | Expose Standard Workqueue Metrics for Kyverno Controllers |  | Y | Y | CONFLICTING | 2/0/6 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15432](https://github.com/kyverno/kyverno/pull/15432) | @archy-rock3t-cloud | test: add unit tests for AnyInHandler |  | Y | Y | MERGEABLE | 1/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15174](https://github.com/kyverno/kyverno/pull/15174) | @IshaanXCoder | Fix Duplicate dependency | Y | Y | Y | CONFLICTING | 2/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15158](https://github.com/kyverno/kyverno/pull/15158) | @Ayush-Patel-56 | fix: persist attestation data across rules |  | Y | Y | MERGEABLE | 4/0/1 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#15037](https://github.com/kyverno/kyverno/pull/15037) | @IshaanXCoder | fix/jmespath-expression-not-allowed |  | Y | Y | MERGEABLE | 3/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#14915](https://github.com/kyverno/kyverno/pull/14915) | @harshakumar25 | feat: support namespaced secrets for imageRegistryCredentials |  | Y | Y | CONFLICTING | 6/0/18 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#14883](https://github.com/kyverno/kyverno/pull/14883) | @iamsgarg-ob | Implicitly ignore DELETE operations for all rule types and configs | Y | Y | Y | MERGEABLE | 2/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#14814](https://github.com/kyverno/kyverno/pull/14814) | @dolisss | feat: add namespaceMatch configuration for policy namespace filtering |  | Y | Y | CONFLICTING | 20/0/0 | NEUTRAL | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#14711](https://github.com/kyverno/kyverno/pull/14711) | @goyalpalak18 | fix: accumulate generated resources from all rules in ProcessUR |  | Y | Y | MERGEABLE | 6/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |
| [#13576](https://github.com/kyverno/kyverno/pull/13576) | @JimBugwadia | add validation for condition types (any) | Y | Y | Y | CONFLICTING | 4/0/0 | LEGACY_CONTENT | SKIPPED | `type_legacy` | Retarget -> release-1.19 (rebase onto release-1.19) |

## MIXED (69) — label `type_mixed`

| PR | Author | Title | Draft | Fork | CanModify | Mergeable | Files (L/C/S) | Content signal | Probe | Proposed label | Proposed Action |
|---|---|---|---|---|---|---|---|---|---|---|---|
| [#17565](https://github.com/kyverno/kyverno/pull/17565) | @ANAMASGARD | perf: reduce engine context checkpoint copy cost |  | Y | Y | MERGEABLE | 9/0/0 | ERROR:diff_exit_1 | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17556](https://github.com/kyverno/kyverno/pull/17556) | @suhaani-agarwal | docs: make the repo agent-friendly (AGENTS.md hierarchy, ARCHITECTURE. |  | Y | Y | MERGEABLE | 0/0/31 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17551](https://github.com/kyverno/kyverno/pull/17551) | @Dreamstick9 | fix: propagate request context to toggle.FromContext call sites | Y | Y | Y | MERGEABLE | 6/2/9 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17536](https://github.com/kyverno/kyverno/pull/17536) | @IgorDaniel45 | fix(jmespath): preserve float64 precision in string conversion |  | Y | Y | MERGEABLE | 0/0/2 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17510](https://github.com/kyverno/kyverno/pull/17510) | @Rohanraj123 | fix(cel): generate a ValidatingAdmissionPolicy/MutatingAdmissionPolicy |  | Y | Y | MERGEABLE | 2/4/6 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17504](https://github.com/kyverno/kyverno/pull/17504) | @anushkagupta200615-jpg | fix(cli): flatten kind: List resources in apply and fake dynamic clien |  | Y | Y | MERGEABLE | 0/0/5 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17478](https://github.com/kyverno/kyverno/pull/17478) | @dhimanAbhi | fix: respect VAP matchConstraints in cli |  | Y | Y | MERGEABLE | 0/0/4 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17476](https://github.com/kyverno/kyverno/pull/17476) | @Sashang-debug | fix(controllers): properly propagate context in ttl and admissionpolic |  | Y | Y | MERGEABLE | 2/0/8 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17429](https://github.com/kyverno/kyverno/pull/17429) | @Mannan-Ali | feat: implement UpdateRequest cleanup TTL via configuration |  | Y | Y | CONFLICTING | 0/0/8 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17391](https://github.com/kyverno/kyverno/pull/17391) | @Retr0-XD | test(cli): add regression test case for issue #11519 |  | Y | Y | MERGEABLE | 0/0/5 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17389](https://github.com/kyverno/kyverno/pull/17389) | @Om-Beast | fix(cli): restore checks assertions in test API |  | Y | Y | MERGEABLE | 0/0/5 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17385](https://github.com/kyverno/kyverno/pull/17385) | @yashrajshuklaaa | fix: use dynamic APIVersion in event ObjectReferences instead of hardc |  | Y | Y | MERGEABLE | 0/0/1 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17335](https://github.com/kyverno/kyverno/pull/17335) | @bhuvan-somisetty | fix: support NamespacedValidatingPolicy in admissionpolicy-generator c |  | Y | Y | MERGEABLE | 1/1/4 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17331](https://github.com/kyverno/kyverno/pull/17331) | @karthikmanam | test(cli): add generate clone source regression tests |  | Y | Y | MERGEABLE | 0/0/2 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17322](https://github.com/kyverno/kyverno/pull/17322) | @Hiten0305l | fix(reports): filter rule status in GenerationEngineResponseToReportR… |  | Y | Y | MERGEABLE | 0/0/2 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17272](https://github.com/kyverno/kyverno/pull/17272) | @Roy-code5k | fix(cli) :- report failure when test expects fail but resource exclude |  | Y | Y | MERGEABLE | 0/0/2 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17258](https://github.com/kyverno/kyverno/pull/17258) | @vishalmore90 | fix(report): prevent nil pointer panic in IsPolicyReportable for typed |  | Y | Y | MERGEABLE | 0/0/2 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17233](https://github.com/kyverno/kyverno/pull/17233) | @pyd-07 | fix: skip unmatched image validating policies |  | Y | Y | MERGEABLE | 0/0/2 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17222](https://github.com/kyverno/kyverno/pull/17222) | @Manoj-Kumar-Selvaraj | fix: skip admission-only policies and paginate reports warmup lists |  | Y | Y | CONFLICTING | 0/0/3 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17194](https://github.com/kyverno/kyverno/pull/17194) | @waterWang | fix: cache KMS-based verifiers by key to prevent goroutine leak in ima |  | Y | Y | MERGEABLE | 2/1/0 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17134](https://github.com/kyverno/kyverno/pull/17134) | @KeerthiKumarR | test: improve test coverage for pkg/utils/match |  | Y | Y | MERGEABLE | 0/0/1 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17119](https://github.com/kyverno/kyverno/pull/17119) | @pujitha24 | fix(status): heal WebhookConfigured condition once a VAP is generated |  | Y | Y | MERGEABLE | 0/0/2 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17071](https://github.com/kyverno/kyverno/pull/17071) | @pujitha24 | fix(reports): drop stale background scan results when the owning polic |  | Y | Y | MERGEABLE | 0/0/2 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17040](https://github.com/kyverno/kyverno/pull/17040) | @Abhinash-Singh | fix(charts): relax PSS user-namespace pods in kyverno-policies chart |  | Y | Y | MERGEABLE | 6/0/1 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#17029](https://github.com/kyverno/kyverno/pull/17029) | @Dreamstick9 | fix(cli): resolve --audit-warn per rule instead of policy-wide |  | Y | Y | MERGEABLE | 0/0/7 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16998](https://github.com/kyverno/kyverno/pull/16998) | @SanthanCH | fix(imageverify): coalesce concurrent verification requests |  | Y | Y | CONFLICTING | 3/1/0 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16964](https://github.com/kyverno/kyverno/pull/16964) | @afarbos | fix(cli): use CRD-declared plural when indexing CR instances in fake D |  | Y | Y | MERGEABLE | 0/0/9 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16954](https://github.com/kyverno/kyverno/pull/16954) | @AftAb-25 | fix(mpol): resolve namespace in Evaluate() to respect namespaceSelecto |  | Y | Y | MERGEABLE | 1/2/1 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16928](https://github.com/kyverno/kyverno/pull/16928) | @khushal-winner | feat(jmespath): cache compiled queries on shared interface |  | Y | Y | MERGEABLE | 0/0/3 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16906](https://github.com/kyverno/kyverno/pull/16906) | @RajdeepKushwaha5 | ci: add a conformance suite map and resolver |  | Y | Y | MERGEABLE | 0/0/3 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16893](https://github.com/kyverno/kyverno/pull/16893) | @codebrak07 | docs(test): add documentation for the conformance test suite |  | Y | Y | MERGEABLE | 0/0/2 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16881](https://github.com/kyverno/kyverno/pull/16881) | @FirePheonix | refactor: optimize JMESPath query parsing in the image extractor |  | Y | Y | CONFLICTING | 0/0/1 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16840](https://github.com/kyverno/kyverno/pull/16840) | @Elvand-Lie | fix: autogen fine-grained namespaced image policies | Y | Y | Y | MERGEABLE | 0/0/2 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16772](https://github.com/kyverno/kyverno/pull/16772) | @bhuvan-somisetty | fix(jmespath): report function and argument in type errors |  | Y | Y | MERGEABLE | 0/0/5 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16765](https://github.com/kyverno/kyverno/pull/16765) | @Ady0333 | fix(cli): run checks-only test files in kyverno test |  | Y | Y | MERGEABLE | 0/0/2 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16693](https://github.com/kyverno/kyverno/pull/16693) | @rx18-eng | docs: add README for the integration testing framework |  | Y | Y | MERGEABLE | 0/0/1 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16659](https://github.com/kyverno/kyverno/pull/16659) | @Iqrima | fix: ignore separator-only documents and clean up YAML split tests |  | Y | Y | CONFLICTING | 6/55/65 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16605](https://github.com/kyverno/kyverno/pull/16605) | @vmsilvamolina | fix(cli): derive OCI apiVersion annotation from policy object |  | Y | Y | CONFLICTING | 0/0/2 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16586](https://github.com/kyverno/kyverno/pull/16586) | @Tommolo | fix: propagate namespaceSelector from NamespacedMutatingPolicy to webh |  | Y | Y | CONFLICTING | 0/0/4 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16583](https://github.com/kyverno/kyverno/pull/16583) | @ShubhamArora073 | fix(cli): report failure when test expects fail but resource excluded | Y | Y | Y | MERGEABLE | 0/0/2 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16547](https://github.com/kyverno/kyverno/pull/16547) | @mailnike | fix: prevent panic in lookupImageExtractor for empty imageExtractor pa |  | Y | Y | MERGEABLE | 0/0/2 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16497](https://github.com/kyverno/kyverno/pull/16497) | @Tavisha10 | fix(cli): honor --audit-warn for ValidatingPolicy audit failures |  | Y | Y | CONFLICTING | 0/0/9 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16418](https://github.com/kyverno/kyverno/pull/16418) | @rx18-eng | test(integration): add VAP generation testing with vpol/native enforce |  | Y | Y | MERGEABLE | 0/0/3 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16407](https://github.com/kyverno/kyverno/pull/16407) | @santhil-cyber | fix(cli): honor request.operation=CONNECT in policy context |  | Y | Y | MERGEABLE | 0/0/2 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16396](https://github.com/kyverno/kyverno/pull/16396) | @ANAMASGARD | fix: PolicyReports for Kyverno-generated admission policies (#16153) |  | Y | Y | MERGEABLE | 53/0/11 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16367](https://github.com/kyverno/kyverno/pull/16367) | @karthikmanam | fix: key clone source resources by policy and rule |  | Y | Y | CONFLICTING | 8/0/3 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16213](https://github.com/kyverno/kyverno/pull/16213) | @lucchmielowski | fix(verify-images): bind image verification cache to verified digest |  | Y | Y | CONFLICTING | 3/3/0 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16198](https://github.com/kyverno/kyverno/pull/16198) | @amine-zaiem | Feat/deleting policy reports |  | Y | Y | MERGEABLE | 2/3/4 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16170](https://github.com/kyverno/kyverno/pull/16170) | @jasdeepbhalla | feat(webhook): minimize webhook registration scope with ValidatingAdmi |  | Y | Y | CONFLICTING | 1/0/2 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16156](https://github.com/kyverno/kyverno/pull/16156) | @Karthikk-18 | test: add test coverage for updaterequest.go |  | Y | Y | MERGEABLE | 1/0/0 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16142](https://github.com/kyverno/kyverno/pull/16142) | @karthikmanam | feat: add namespace-based webhook filtering for namespaced policies |  | Y | Y | CONFLICTING | 0/0/4 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16136](https://github.com/kyverno/kyverno/pull/16136) | @ObaidAbdullah16 | docs: add technical outcome pages for platform engineering and complia |  | Y | Y | MERGEABLE | 0/0/2 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#16093](https://github.com/kyverno/kyverno/pull/16093) | @tejassinghbhati | docs: add AI & Agent Governance and Multi-Cluster Policy Management ou |  | Y | Y | MERGEABLE | 0/0/2 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15977](https://github.com/kyverno/kyverno/pull/15977) | @atharrva01 | feat(gpol): add reconciler-backed compiled-policy cache |  | Y | Y | MERGEABLE | 1/2/0 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15972](https://github.com/kyverno/kyverno/pull/15972) | @Ady0333 | [fix]: always re-arm cron schedule in cleanup and deleting controllers |  | Y | Y | MERGEABLE | 2/2/0 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15881](https://github.com/kyverno/kyverno/pull/15881) | @malsomesh9 | [codex] Fail CLI tests on messageExpression errors |  | Y | Y | CONFLICTING | 0/0/2 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15866](https://github.com/kyverno/kyverno/pull/15866) | @JimBugwadia | Feat/conditional authorization | Y | Y | Y | CONFLICTING | 4/30/100 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15849](https://github.com/kyverno/kyverno/pull/15849) | @Rama542 | feat: remove kyverno-json support (#15435) |  | Y | Y | CONFLICTING | 14/0/28 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15840](https://github.com/kyverno/kyverno/pull/15840) | @Rama542 | feat: add chainsaw tests for restrict-scale policy (#950) |  | Y | Y | MERGEABLE | 0/0/4 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15618](https://github.com/kyverno/kyverno/pull/15618) | @Rohanraj123 | feat(cli): add mock APICall and GlobalContextEntry support for kyverno | Y | Y | Y | CONFLICTING | 23/12/29 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15508](https://github.com/kyverno/kyverno/pull/15508) | @eddycharly | chore: move back to dockerhub in chainsaw tests | Y | Y | Y | CONFLICTING | 314/132/140 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15503](https://github.com/kyverno/kyverno/pull/15503) | @rishabh998186 | Add wildcard detection and emit policy warning events |  | Y | Y | CONFLICTING | 3/0/9 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15495](https://github.com/kyverno/kyverno/pull/15495) | @ndpvt-web | feat: warn when creating wildcard policies |  | Y | Y | CONFLICTING | 1/0/8 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15442](https://github.com/kyverno/kyverno/pull/15442) | @jsbasra | Added method to create PolicyExceptions via values.yaml |  | Y | Y | MERGEABLE | 3/0/0 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15412](https://github.com/kyverno/kyverno/pull/15412) | @sysedwinistrator | fix(cli/apply): use namespace from flag for manifests without namespac |  | Y | Y | CONFLICTING | 0/0/15 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15402](https://github.com/kyverno/kyverno/pull/15402) | @atharrva01 | feat(reports): respect webhook namespace/object selectors in backgroun |  | Y | Y | CONFLICTING | 0/0/3 | MIXED_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#15064](https://github.com/kyverno/kyverno/pull/15064) | @harshakumar25 | refactor: add context propagation to resource fetching functions |  | Y | Y | CONFLICTING | 3/2/3 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#14904](https://github.com/kyverno/kyverno/pull/14904) | @Meetjain1 | fix(cli:test): make results.rule act as a filter |  | Y | Y | MERGEABLE | 0/0/5 | LEGACY_CONTENT | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |
| [#14621](https://github.com/kyverno/kyverno/pull/14621) | @roandudel | fix: wildcard-policy-new-cr-match |  | Y | Y | MERGEABLE | 7/7/2 |  | SKIPPED | `type_mixed` | Manual review: split into 2 PRs or keep on main |

<details><summary>Mixed PR file breakdown</summary>

### #17565 perf: reduce engine context checkpoint copy cost
- LEGACY files: pkg/engine/background_context_test.go, pkg/engine/context/checkpoint_bench_test.go, pkg/engine/context/checkpoint_test.go, pkg/engine/context/context.go, pkg/engine/context/evaluate.go, pkg/engine/context/utils.go, pkg/engine/pss_admission_bench_test.go, pkg/engine/testdata/pss-restricted-clusterpolicies.yaml, pkg/engine/validate/query_alias_test.go
- Content signal: `ERROR:diff_exit_1` (legacy diff lines: 0, cel diff lines: 0)

### #17556 docs: make the repo agent-friendly (AGENTS.md hierarchy, ARCHITECTURE.md, docs/context)
- Content signal: `MIXED_CONTENT` (legacy diff lines: 56, cel diff lines: 99)

### #17551 fix: propagate request context to toggle.FromContext call sites
- LEGACY files: pkg/validation/policy/actions.go, pkg/validation/policy/actions_test.go, pkg/validation/policy/fuzz_test.go, pkg/validation/policy/validate.go, pkg/validation/policy/validate_test.go, pkg/webhooks/policy/handlers.go
- CEL files: pkg/webhooks/resource/mpol/handler.go, pkg/webhooks/resource/mpol/handler_test.go

### #17536 fix(jmespath): preserve float64 precision in string conversion
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 1, cel diff lines: 0)

### #17510 fix(cel): generate a ValidatingAdmissionPolicy/MutatingAdmissionPolicy per autogen group
- LEGACY files: pkg/controllers/admissionpolicygenerator/generate-vap.go, pkg/validation/policy/validate.go
- CEL files: pkg/cel/policies/mpol/autogen/autogen.go, pkg/cel/policies/mpol/autogen/autogen_test.go, pkg/controllers/admissionpolicygenerator/mpol.go, pkg/controllers/admissionpolicygenerator/vpol.go

### #17504 fix(cli): flatten kind: List resources in apply and fake dynamic client
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 2, cel diff lines: 0)

### #17478 fix: respect VAP matchConstraints in cli
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 9, cel diff lines: 0)

### #17476 fix(controllers): properly propagate context in ttl and admissionpolicygenerator controllers
- LEGACY files: cmd/cleanup-controller/handlers/admission/resource/handlers.go, cmd/cleanup-controller/handlers/admission/resource/handlers_test.go
- Content signal: `MIXED_CONTENT` (legacy diff lines: 6, cel diff lines: 2)

### #17429 feat: implement UpdateRequest cleanup TTL via configuration
- Content signal: `MIXED_CONTENT` (legacy diff lines: 15, cel diff lines: 201)

### #17391 test(cli): add regression test case for issue #11519
- Content signal: `MIXED_CONTENT` (legacy diff lines: 2, cel diff lines: 4)

### #17389 fix(cli): restore checks assertions in test API
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 13, cel diff lines: 0)

### #17385 fix: use dynamic APIVersion in event ObjectReferences instead of hardcoded values
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 3, cel diff lines: 0)

### #17335 fix: support NamespacedValidatingPolicy in admissionpolicy-generator controller
- LEGACY files: pkg/controllers/admissionpolicygenerator/generate-vap.go
- CEL files: pkg/controllers/admissionpolicygenerator/vpol.go

### #17331 test(cli): add generate clone source regression tests
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 8, cel diff lines: 0)

### #17322 fix(reports): filter rule status in GenerationEngineResponseToReportR…
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 8, cel diff lines: 0)

### #17272 fix(cli) :- report failure when test expects fail but resource excluded
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 8, cel diff lines: 0)

### #17258 fix(report): prevent nil pointer panic in IsPolicyReportable for typed nil policies
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 6, cel diff lines: 0)

### #17233 fix: skip unmatched image validating policies
- Content signal: `MIXED_CONTENT` (legacy diff lines: 6, cel diff lines: 3)

### #17222 fix: skip admission-only policies and paginate reports warmup lists
- Content signal: `MIXED_CONTENT` (legacy diff lines: 41, cel diff lines: 9)

### #17194 fix: cache KMS-based verifiers by key to prevent goroutine leak in image verification
- LEGACY files: pkg/image/verifiers/cpol/cosign/cosign.go, pkg/image/verifiers/cpol/cosign/sigstore.go
- CEL files: pkg/image/verifiers/ivpol/cosign/opts.go

### #17134 test: improve test coverage for pkg/utils/match
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 22, cel diff lines: 0)

### #17119 fix(status): heal WebhookConfigured condition once a VAP is generated
- Content signal: `MIXED_CONTENT` (legacy diff lines: 1, cel diff lines: 11)

### #17071 fix(reports): drop stale background scan results when the owning policy changes
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 1, cel diff lines: 0)

### #17040 fix(charts): relax PSS user-namespace pods in kyverno-policies chart
- LEGACY files: charts/kyverno-policies/ci/scripts/verify-user-namespaces-render.py, charts/kyverno-policies/ci/test-user-namespaces-values.yaml, charts/kyverno-policies/ci/test-vpol-user-namespaces-values.yaml, charts/kyverno-policies/templates/baseline/disallow-proc-mount.cel.yaml, charts/kyverno-policies/templates/baseline/disallow-proc-mount.yaml, charts/kyverno-policies/values.yaml
- Content signal: `MIXED_CONTENT` (legacy diff lines: 3, cel diff lines: 2)

### #17029 fix(cli): resolve --audit-warn per rule instead of policy-wide
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 39, cel diff lines: 0)

### #16998 fix(imageverify): coalesce concurrent verification requests
- LEGACY files: pkg/engine/internal/imageverifier.go, pkg/engine/internal/verification_group.go, pkg/engine/internal/verification_group_test.go
- CEL files: pkg/image/verification/cache/client.go

### #16964 fix(cli): use CRD-declared plural when indexing CR instances in fake DClient
- Content signal: `MIXED_CONTENT` (legacy diff lines: 1, cel diff lines: 3)

### #16954 fix(mpol): resolve namespace in Evaluate() to respect namespaceSelector
- LEGACY files: cmd/background-controller/main.go
- CEL files: pkg/cel/policies/mpol/engine/engine.go, pkg/cel/policies/mpol/engine/engine_test.go

### #16928 feat(jmespath): cache compiled queries on shared interface
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 1, cel diff lines: 0)

### #16906 ci: add a conformance suite map and resolver
- Content signal: `MIXED_CONTENT` (legacy diff lines: 1, cel diff lines: 1)

### #16893 docs(test): add documentation for the conformance test suite
- Content signal: `MIXED_CONTENT` (legacy diff lines: 3, cel diff lines: 3)

### #16881 refactor: optimize JMESPath query parsing in the image extractor
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 6, cel diff lines: 0)

### #16840 fix: autogen fine-grained namespaced image policies
- Content signal: `MIXED_CONTENT` (legacy diff lines: 2, cel diff lines: 6)

### #16772 fix(jmespath): report function and argument in type errors
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 3, cel diff lines: 0)

### #16765 fix(cli): run checks-only test files in kyverno test
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 2, cel diff lines: 0)

### #16693 docs: add README for the integration testing framework
- Content signal: `MIXED_CONTENT` (legacy diff lines: 4, cel diff lines: 45)

### #16659 fix: ignore separator-only documents and clean up YAML split tests
- LEGACY files: pkg/background/common/status.go, pkg/background/common/util.go, pkg/background/generate/cleanup.go, pkg/policy/generate.go, pkg/policy/policy_controller.go, pkg/webhooks/resource/generation/handler.go
- CEL files: config/crds/policies.kyverno.io/policies.kyverno.io_mutatingpolicies.yaml, config/crds/policies.kyverno.io/policies.kyverno.io_namespacedmutatingpolicies.yaml, pkg/background/mpol/processor.go, pkg/background/mpol/processor_test.go, pkg/cel/libs/imageverify/impl.go, pkg/cel/libs/imageverify/impl_test.go, pkg/cel/libs/imageverify/lib.go, pkg/cel/policies/ivpol/engine/engine.go, pkg/cel/policies/ivpol/engine/engine_test.go, pkg/cel/policies/ivpol/engine/predicate.go, pkg/cel/policies/ivpol/engine/predicate_test.go, pkg/cel/policies/mpol/compiler/applyconfig.go, pkg/cel/policies/mpol/compiler/compiler.go, pkg/cel/policies/mpol/compiler/compiler_test.go, pkg/cel/policies/mpol/compiler/policy.go

### #16605 fix(cli): derive OCI apiVersion annotation from policy object
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 15, cel diff lines: 0)

### #16586 fix: propagate namespaceSelector from NamespacedMutatingPolicy to webhook configuration
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 3, cel diff lines: 0)

### #16583 fix(cli): report failure when test expects fail but resource excluded
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 14, cel diff lines: 0)

### #16547 fix: prevent panic in lookupImageExtractor for empty imageExtractor path
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 2, cel diff lines: 0)

### #16497 fix(cli): honor --audit-warn for ValidatingPolicy audit failures
- Content signal: `MIXED_CONTENT` (legacy diff lines: 21, cel diff lines: 10)

### #16418 test(integration): add VAP generation testing with vpol/native enforcement equivalence
- Content signal: `MIXED_CONTENT` (legacy diff lines: 16, cel diff lines: 33)

### #16407 fix(cli): honor request.operation=CONNECT in policy context
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 12, cel diff lines: 0)

### #16396 fix: PolicyReports for Kyverno-generated admission policies (#16153)
- LEGACY files: test/conformance/chainsaw/generate-mutating-admission-policy/applyconfiguration/mutatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-mutating-admission-policy/autogen-enabled/mutatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-mutating-admission-policy/generation-config/default/mutatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-mutating-admission-policy/generation-config/disabled/mutatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-mutating-admission-policy/jsonpatch/mutatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-mutating-admission-policy/with-cel-exceptions/mutatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-mutating-admission-policy/with-multiple-exceptions/mutatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-validating-admission-policy/clusterpolicy/standard/generate/block-ephemeral-containers/validatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-validating-admission-policy/clusterpolicy/standard/generate/block-exec-in-pods/validatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-validating-admission-policy/clusterpolicy/standard/generate/cpol-all-match-resource/validatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-validating-admission-policy/clusterpolicy/standard/generate/cpol-any-exclude-namespace-match-resource/validatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-validating-admission-policy/clusterpolicy/standard/generate/cpol-any-exclude-resource-match-with-namespace-selector/validatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-validating-admission-policy/clusterpolicy/standard/generate/cpol-any-exclude-resource-match-with-object-selector/validatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-validating-admission-policy/clusterpolicy/standard/generate/cpol-any-exclude-resource/validatingadmissionpolicy.yaml, test/conformance/chainsaw/generate-validating-admission-policy/clusterpolicy/standard/generate/cpol-any-match-multiple-resources/validatingadmissionpolicy.yaml
- Content signal: `MIXED_CONTENT` (legacy diff lines: 4, cel diff lines: 32)

### #16367 fix: key clone source resources by policy and rule
- LEGACY files: test/cli/test-generate/duplicate-rule-clone-source/cloneSourceResources-a.yaml, test/cli/test-generate/duplicate-rule-clone-source/cloneSourceResources-b.yaml, test/cli/test-generate/duplicate-rule-clone-source/generatedResource-a.yaml, test/cli/test-generate/duplicate-rule-clone-source/generatedResource-b.yaml, test/cli/test-generate/duplicate-rule-clone-source/kyverno-test.yaml, test/cli/test-generate/duplicate-rule-clone-source/policy-a.yaml, test/cli/test-generate/duplicate-rule-clone-source/policy-b.yaml, test/cli/test-generate/duplicate-rule-clone-source/resource.yaml
- Content signal: `MIXED_CONTENT` (legacy diff lines: 6, cel diff lines: 8)

### #16213 fix(verify-images): bind image verification cache to verified digest
- LEGACY files: pkg/engine/image_verify_test.go, pkg/engine/internal/imageverifier.go, pkg/engine/internal/imageverifier_cache_test.go
- CEL files: pkg/image/verification/cache/client.go, pkg/image/verification/cache/client_test.go, pkg/image/verification/cache/interface.go

### #16198 Feat/deleting policy reports
- LEGACY files: pkg/controllers/cleanup/controller.go, pkg/controllers/cleanup/report.go
- CEL files: pkg/controllers/deleting/controller.go, pkg/controllers/deleting/report.go, pkg/controllers/deleting/report_test.go

### #16170 feat(webhook): minimize webhook registration scope with ValidatingAdmissionPolicy autogen
- LEGACY files: pkg/controllers/admissionpolicygenerator/generate-vap.go
- Content signal: `MIXED_CONTENT` (legacy diff lines: 8, cel diff lines: 23)

### #16156 test: add test coverage for updaterequest.go
- LEGACY files: pkg/policy/updaterequest_test.go
- Content signal: `MIXED_CONTENT` (legacy diff lines: 12, cel diff lines: 1)

### #16142 feat: add namespace-based webhook filtering for namespaced policies
- Content signal: `MIXED_CONTENT` (legacy diff lines: 8, cel diff lines: 23)

### #16136 docs: add technical outcome pages for platform engineering and compliance
- Content signal: `MIXED_CONTENT` (legacy diff lines: 4, cel diff lines: 8)

### #16093 docs: add AI & Agent Governance and Multi-Cluster Policy Management outcome pages
- Content signal: `MIXED_CONTENT` (legacy diff lines: 6, cel diff lines: 8)

### #15977 feat(gpol): add reconciler-backed compiled-policy cache
- LEGACY files: cmd/background-controller/main.go
- CEL files: pkg/cel/policies/gpol/engine/reconciler.go, pkg/cel/policies/gpol/engine/reconciler_test.go

### #15972 [fix]: always re-arm cron schedule in cleanup and deleting controllers
- LEGACY files: pkg/controllers/cleanup/controller.go, pkg/controllers/cleanup/controller_test.go
- CEL files: pkg/controllers/deleting/controller.go, pkg/controllers/deleting/controller_test.go

### #15881 [codex] Fail CLI tests on messageExpression errors
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 7, cel diff lines: 0)

### #15866 Feat/conditional authorization
- LEGACY files: config/crds/kyverno/kyverno.io_clusterpolicies.yaml, config/crds/kyverno/kyverno.io_policies.yaml, config/crds/kyverno/kyverno.io_updaterequests.yaml, pkg/engine/handlers/validation/validate_cel.go
- CEL files: charts/kyverno/charts/crds/templates/policies.kyverno.io/policies.kyverno.io_authorizingpolicies.yaml, charts/kyverno/charts/crds/templates/policies.kyverno.io/policies.kyverno.io_deletingpolicies.yaml, charts/kyverno/charts/crds/templates/policies.kyverno.io/policies.kyverno.io_generatingpolicies.yaml, charts/kyverno/charts/crds/templates/policies.kyverno.io/policies.kyverno.io_imagevalidatingpolicies.yaml, charts/kyverno/charts/crds/templates/policies.kyverno.io/policies.kyverno.io_mutatingpolicies.yaml, charts/kyverno/charts/crds/templates/policies.kyverno.io/policies.kyverno.io_namespaceddeletingpolicies.yaml, charts/kyverno/charts/crds/templates/policies.kyverno.io/policies.kyverno.io_namespacedgeneratingpolicies.yaml, charts/kyverno/charts/crds/templates/policies.kyverno.io/policies.kyverno.io_namespacedimagevalidatingpolicies.yaml, charts/kyverno/charts/crds/templates/policies.kyverno.io/policies.kyverno.io_namespacedmutatingpolicies.yaml, charts/kyverno/charts/crds/templates/policies.kyverno.io/policies.kyverno.io_namespacedvalidatingpolicies.yaml, charts/kyverno/charts/crds/templates/policies.kyverno.io/policies.kyverno.io_policyexceptions.yaml, charts/kyverno/charts/crds/templates/policies.kyverno.io/policies.kyverno.io_validatingpolicies.yaml, config/crds/policies.kyverno.io/policies.kyverno.io_authorizingpolicies.yaml, config/crds/policies.kyverno.io/policies.kyverno.io_deletingpolicies.yaml, config/crds/policies.kyverno.io/policies.kyverno.io_generatingpolicies.yaml

### #15849 feat: remove kyverno-json support (#15435)
- LEGACY files: api/kyverno/v1/common_types.go, api/kyverno/v1/rule_types.go, api/kyverno/v1/zz_generated.deepcopy.go, api/kyverno/v2beta1/common_types.go, api/kyverno/v2beta1/zz_generated.deepcopy.go, config/crds/kyverno/kyverno.io_clusterpolicies.yaml, config/crds/kyverno/kyverno.io_policies.yaml, pkg/autogen/v1/autogen.go, pkg/autogen/v1/autogen_test.go, pkg/autogen/v1/rule.go, pkg/engine/handlers/validation/validate_assert.go, pkg/engine/validation.go, pkg/policy/validate/validate.go, pkg/validation/policy/validate.go
- Content signal: `MIXED_CONTENT` (legacy diff lines: 71, cel diff lines: 7)

### #15840 feat: add chainsaw tests for restrict-scale policy (#950)
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 4, cel diff lines: 0)

### #15618 feat(cli): add mock APICall and GlobalContextEntry support for kyverno test
- LEGACY files: api/kyverno/v1/resource_description_types.go, api/kyverno/v2beta1/match_resources_types.go, api/kyverno/v2beta1/resource_description_types.go, pkg/engine/variables/operator/allin.go, pkg/engine/variables/operator/anyin.go, pkg/image/verifiers/cpol/notary/notary.go, pkg/webhooks/resource/generation/utils.go, pkg/webhooks/resource/imageverification/handler.go, pkg/webhooks/resource/mutation/mutation.go, pkg/webhooks/resource/validation/validation.go, pkg/webhooks/updaterequest/generator.go, test/cli/test/cel-http-get-mock/kyverno-test.yaml, test/cli/test/cel-http-get-mock/policy.yaml, test/cli/test/cel-http-get-mock/resources.yaml, test/cli/test/mock-apicall-responses/kyverno-test.yaml
- CEL files: pkg/background/mpol/processor.go, pkg/cel/libs/fake_context.go, pkg/cel/libs/fake_context_test.go, pkg/cel/libs/http_mock.go, pkg/cel/libs/http_mock_test.go, pkg/cel/policies/dpol/compiler/compiler.go, pkg/cel/policies/gpol/compiler/compiler.go, pkg/cel/policies/mpol/compiler/compiler.go, pkg/cel/policies/mpol/validate.go, pkg/cel/policies/vpol/compiler/compiler.go, pkg/cel/policies/vpol/validate.go, pkg/webhooks/resource/mpol/handler.go

### #15508 chore: move back to dockerhub in chainsaw tests
- LEGACY files: test/conformance/chainsaw/background-only/cluster-policy/no-admission-event-deprecated/resource.yaml, test/conformance/chainsaw/background-only/cluster-policy/no-admission-event/resource.yaml, test/conformance/chainsaw/background-only/cluster-policy/no-admission-report-deprecated/resource.yaml, test/conformance/chainsaw/background-only/cluster-policy/no-admission-report/resource.yaml, test/conformance/chainsaw/background-only/cluster-policy/not-rejected-deprecated/resource.yaml, test/conformance/chainsaw/background-only/cluster-policy/not-rejected/resource.yaml, test/conformance/chainsaw/background-only/policy/no-admission-event-deprecated/resource.yaml, test/conformance/chainsaw/background-only/policy/no-admission-event/resource.yaml, test/conformance/chainsaw/background-only/policy/no-admission-report-deprecated/resource.yaml, test/conformance/chainsaw/background-only/policy/no-admission-report/resource.yaml, test/conformance/chainsaw/background-only/policy/not-rejected-deprecated/resource.yaml, test/conformance/chainsaw/background-only/policy/not-rejected/resource.yaml, test/conformance/chainsaw/cleanup/clusterpolicy/cleanup-pod/pod.yaml, test/conformance/chainsaw/cleanup/clusterpolicy/context-cleanup-pod/pod.yaml, test/conformance/chainsaw/cleanup/policy/cleanup-pod/pod.yaml
- CEL files: test/conformance/chainsaw/deleting-policies/cel-lib/globalcontext-lib/deployment.yaml, test/conformance/chainsaw/deleting-policies/cel-lib/globalcontext-lib/pod.yaml, test/conformance/chainsaw/deleting-policies/cel-lib/http-lib/http-pod.yaml, test/conformance/chainsaw/deleting-policies/cel-lib/http-lib/pod.yaml, test/conformance/chainsaw/deleting-policies/cel-lib/image-data-lib/pod.yaml, test/conformance/chainsaw/deleting-policies/cel-lib/image-lib/pod.yaml, test/conformance/chainsaw/deleting-policies/cel-lib/resource-lib.yaml/pod.yaml, test/conformance/chainsaw/deleting-policies/delete-pod-by-namespaceObject/pod-assert.yaml, test/conformance/chainsaw/deleting-policies/delete-pod-by-namespaceObject/pod-error.yaml, test/conformance/chainsaw/deleting-policies/delete-pod-by-namespaceObject/pods.yaml, test/conformance/chainsaw/deleting-policies/delete-pod/pod.yaml, test/conformance/chainsaw/deleting-policies/dpol-v1alpha1/pod.yaml, test/conformance/chainsaw/deleting-policies/schedule/pod.yaml, test/conformance/chainsaw/deleting-policies/schedule/policy.yaml, test/conformance/chainsaw/generating-policies/context/api-call/http-pod.yaml

### #15503 Add wildcard detection and emit policy warning events
- LEGACY files: pkg/policy/policy_controller.go, pkg/validation/policy/validate.go, pkg/webhooks/policy/handlers.go
- Content signal: `MIXED_CONTENT` (legacy diff lines: 59, cel diff lines: 38)

### #15495 feat: warn when creating wildcard policies
- LEGACY files: pkg/validation/policy/validate.go
- Content signal: `MIXED_CONTENT` (legacy diff lines: 2, cel diff lines: 4)

### #15442 Added method to create PolicyExceptions via values.yaml
- LEGACY files: charts/kyverno-policies/templates/_helpers.tpl, charts/kyverno-policies/templates/other/custom-policyexceptions.yaml, charts/kyverno-policies/values.yaml
- Content signal: `MIXED_CONTENT` (legacy diff lines: 1, cel diff lines: 1)

### #15412 fix(cli/apply): use namespace from flag for manifests without namespace
- Content signal: `MIXED_CONTENT` (legacy diff lines: 4, cel diff lines: 13)

### #15402 feat(reports): respect webhook namespace/object selectors in background scan
- Content signal: `MIXED_CONTENT` (legacy diff lines: 22, cel diff lines: 6)

### #15064 refactor: add context propagation to resource fetching functions
- LEGACY files: pkg/background/common/resource.go, pkg/background/generate/controller.go, pkg/background/mutate/mutate.go
- CEL files: pkg/background/gpol/generate_controller.go, pkg/cel/policies/vpol/engine/reconciler.go

### #14904 fix(cli:test): make results.rule act as a filter
- Content signal: `LEGACY_CONTENT` (legacy diff lines: 3, cel diff lines: 0)

### #14621 fix: wildcard-policy-new-cr-match
- LEGACY files: pkg/controllers/policycache/controller.go, test/conformance/chainsaw/policy-validation/cluster-policy/crd-created-after-policy/chainsaw-test.yaml, test/conformance/chainsaw/policy-validation/cluster-policy/crd-created-after-policy/crd-ready.yaml, test/conformance/chainsaw/policy-validation/cluster-policy/crd-created-after-policy/crd.yaml, test/conformance/chainsaw/policy-validation/cluster-policy/crd-created-after-policy/custom-resource-ready.yaml, test/conformance/chainsaw/policy-validation/cluster-policy/crd-created-after-policy/custom-resource.yaml, test/conformance/chainsaw/policy-validation/cluster-policy/crd-created-after-policy/policy.yaml
- CEL files: test/conformance/chainsaw/validating-policies/crd-created-after-policy/chainsaw-test.yaml, test/conformance/chainsaw/validating-policies/crd-created-after-policy/crd-ready.yaml, test/conformance/chainsaw/validating-policies/crd-created-after-policy/crd.yaml, test/conformance/chainsaw/validating-policies/crd-created-after-policy/custom-resource-ready.yaml, test/conformance/chainsaw/validating-policies/crd-created-after-policy/custom-resource.yaml, test/conformance/chainsaw/validating-policies/crd-created-after-policy/policy.yaml, test/conformance/chainsaw/validating-policies/crd-created-after-policy/rbac.yaml

</details>

## CEL_ONLY (74) — label `type_cel`

| PR | Author | Title | Draft | Fork | CanModify | Mergeable | Files (L/C/S) | Content signal | Probe | Proposed label | Proposed Action |
|---|---|---|---|---|---|---|---|---|---|---|---|
| [#17561](https://github.com/kyverno/kyverno/pull/17561) | @pgmpofu | fix: avoid image validating policy webhook name collisions |  | Y | Y | MERGEABLE | 0/0/3 | CEL_CONTENT | SKIPPED | `type_cel` | Keep on main |
| [#17558](https://github.com/kyverno/kyverno/pull/17558) | @Ahmad-Faraj | fix(webhook): avoid image validating policy webhook name collisions |  | Y | Y | MERGEABLE | 0/0/3 | CEL_CONTENT | SKIPPED | `type_cel` | Keep on main |
| [#17555](https://github.com/kyverno/kyverno/pull/17555) | @jzeng4 | feat: verify OCI image volume references by default |  | Y | Y | MERGEABLE | 0/2/2 |  | SKIPPED | `type_cel` | Keep on main |
| [#17545](https://github.com/kyverno/kyverno/pull/17545) | @tendinginfinity24 | fix(background): guard ListResource nil dereference in handleUpdate |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17543](https://github.com/kyverno/kyverno/pull/17543) | @ShreyanshK1103 | fix: ignore no-op updates for synchronized generating policies |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17538](https://github.com/kyverno/kyverno/pull/17538) | @badnikhil | fix: return from the policystatus watchdog when its context is cancell |  | Y | Y | MERGEABLE | 0/0/2 | CEL_CONTENT | SKIPPED | `type_cel` | Keep on main |
| [#17518](https://github.com/kyverno/kyverno/pull/17518) | @shashankvarma499 | fix(cli): resource.List returns empty list when no resources of the ki |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17511](https://github.com/kyverno/kyverno/pull/17511) | @shashankvarma499 | fix(ivpol): check cosign annotations against signature payload optiona |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17497](https://github.com/kyverno/kyverno/pull/17497) | @hugolevino | fix(ivpol): surface image-verification setup errors instead of masking | Y | Y | Y | MERGEABLE | 0/8/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17496](https://github.com/kyverno/kyverno/pull/17496) | @pyd-07 | fix: evaluate ImageValidatingPolicies concurrently |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17495](https://github.com/kyverno/kyverno/pull/17495) | @anushkagupta200615-jpg | fix(policy): resolve subresource triggers in mutating policy matchCons |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17479](https://github.com/kyverno/kyverno/pull/17479) | @anushkagupta200615-jpg | fix(webhook): enforce exceptionNamespace for CEL PolicyExceptions |  | Y | Y | CONFLICTING | 0/2/1 |  | SKIPPED | `type_cel` | Keep on main |
| [#17474](https://github.com/kyverno/kyverno/pull/17474) | @itsvishalyadav | fix(exceptions): honour expiresAt in the CEL policy providers |  | Y | Y | MERGEABLE | 0/9/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17458](https://github.com/kyverno/kyverno/pull/17458) | @pyd-07 | fix(cli): report excluded DeletingPolicy resources as skipped |  | Y | Y | MERGEABLE | 0/1/3 |  | SKIPPED | `type_cel` | Keep on main |
| [#17440](https://github.com/kyverno/kyverno/pull/17440) | @pyd-07 | fix: correct Object.metadata type in MPOL autogen |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17433](https://github.com/kyverno/kyverno/pull/17433) | @pujitha24 | fix(mpol): evaluate matchConditions during background mutateExisting e |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17388](https://github.com/kyverno/kyverno/pull/17388) | @Om-Beast | fix(cli): preserve GeneratingPolicy skips in test results |  | Y | Y | MERGEABLE | 0/4/1 |  | SKIPPED | `type_cel` | Keep on main |
| [#17353](https://github.com/kyverno/kyverno/pull/17353) | @yashrajshuklaaa | fix(cel): set rule name for validate.cel results |  | Y | Y | MERGEABLE | 0/1/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17344](https://github.com/kyverno/kyverno/pull/17344) | @smkspurti | fix(cli): propagate messageExpression errors to fail cli tests |  | Y | Y | MERGEABLE | 0/3/6 |  | SKIPPED | `type_cel` | Keep on main |
| [#17324](https://github.com/kyverno/kyverno/pull/17324) | @pyd-07 | fix: preserve MutatingPolicy reinvocation policy |  | Y | Y | MERGEABLE | 0/0/2 | CEL_CONTENT | SKIPPED | `type_cel` | Keep on main |
| [#17320](https://github.com/kyverno/kyverno/pull/17320) | @itsvishalyadav | Fix silent dropping of negative priorities in CEL reconcilers |  | Y | Y | MERGEABLE | 0/3/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17281](https://github.com/kyverno/kyverno/pull/17281) | @vishalmore90 | Fix Notary image verifier silently dropping subsequent attestations |  | Y | Y | MERGEABLE | 0/1/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17270](https://github.com/kyverno/kyverno/pull/17270) | @tendinginfinity24 | fix(gpol): re-register downstream UID in metadataCache after recreatio |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17263](https://github.com/kyverno/kyverno/pull/17263) | @ShreyanshK1103 | fix(deleting): process namespaced resources with pagination |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17255](https://github.com/kyverno/kyverno/pull/17255) | @itsvishalyadav | [Bug] Fix CEL policy reconcilers to correctly handle cache invalidatio |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17219](https://github.com/kyverno/kyverno/pull/17219) | @Bhuvanesh66 | docs(agents): fix stale paths and API group in AGENTS.md |  | Y | Y | MERGEABLE | 0/0/1 | CEL_CONTENT | SKIPPED | `type_cel` | Keep on main |
| [#17217](https://github.com/kyverno/kyverno/pull/17217) | @pujitha24 | fix(ivpol): honor insecureIgnoreTlog/insecureIgnoreSCT in attestation  |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17181](https://github.com/kyverno/kyverno/pull/17181) | @cavemansatyn-design | fix(cel): scope blocked status per policy |  | Y | Y | MERGEABLE | 0/2/1 |  | SKIPPED | `type_cel` | Keep on main |
| [#17176](https://github.com/kyverno/kyverno/pull/17176) | @cavemansatyn-design | fix(gpol): preserve DELETE trigger metadata |  | Y | Y | MERGEABLE | 0/1/1 |  | SKIPPED | `type_cel` | Keep on main |
| [#17164](https://github.com/kyverno/kyverno/pull/17164) | @Gagansharma-code | fix(imageverify): fail closed on Referrer/Notary attestation cache hit |  | Y | Y | MERGEABLE | 0/6/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#17063](https://github.com/kyverno/kyverno/pull/17063) | @SashaMIT | fix(mpol): fail UpdateRequest when background rules Error/Fail |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16956](https://github.com/kyverno/kyverno/pull/16956) | @beep-boopp | fix: apply autogen path rewrite to PolicyException matchConditions |  | Y | Y | CONFLICTING | 0/11/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16942](https://github.com/kyverno/kyverno/pull/16942) | @tendinginfinity24 | feat(cel): add time extension library for CEL policies | Y | Y | Y | MERGEABLE | 0/8/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16842](https://github.com/kyverno/kyverno/pull/16842) | @ANAMASGARD | test(ivpol): fix Sigstore bundle discovery in separate signature repos |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16839](https://github.com/kyverno/kyverno/pull/16839) | @Elvand-Lie | fix: report passing CEL deny policies | Y | Y | Y | MERGEABLE | 0/3/1 |  | SKIPPED | `type_cel` | Keep on main |
| [#16827](https://github.com/kyverno/kyverno/pull/16827) | @karthik120710 | feat : Add test cases for name matching and exclude matching in CEL ma |  | Y | Y | MERGEABLE | 0/1/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16814](https://github.com/kyverno/kyverno/pull/16814) | @Aneesh-Hegde | feat: parallelize CEL policy evaluation in vpol and ivpol engines |  | Y | Y | CONFLICTING | 0/4/1 |  | SKIPPED | `type_cel` | Keep on main |
| [#16748](https://github.com/kyverno/kyverno/pull/16748) | @rx18-eng | fix(generate): fix GeneratingPolicy synchronization for custom resourc |  | Y | Y | CONFLICTING | 0/2/3 |  | SKIPPED | `type_cel` | Keep on main |
| [#16707](https://github.com/kyverno/kyverno/pull/16707) | @ANAMASGARD | fix(autogen): preserve controller metadata paths in VPol and IVPol |  | Y | Y | MERGEABLE | 0/5/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16696](https://github.com/kyverno/kyverno/pull/16696) | @aditip149209 | Added logic for toggling autogen |  | Y | Y | MERGEABLE | 0/12/6 |  | SKIPPED | `type_cel` | Keep on main |
| [#16651](https://github.com/kyverno/kyverno/pull/16651) | @Goutham-Annem | test(mpol/engine): add tests for ClusteredPolicy, NamespacedPolicy, No |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16650](https://github.com/kyverno/kyverno/pull/16650) | @Goutham-Annem | test(cel/libs/imageverify): add tests for utils functions |  | Y | Y | MERGEABLE | 0/1/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16649](https://github.com/kyverno/kyverno/pull/16649) | @Goutham-Annem | test(vpol/engine): add tests for predicate functions |  | Y | Y | MERGEABLE | 0/1/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16648](https://github.com/kyverno/kyverno/pull/16648) | @Goutham-Annem | test(cel/compiler): add tests for CompileMatchConditionsWithKubernetes |  | Y | Y | MERGEABLE | 0/1/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16645](https://github.com/kyverno/kyverno/pull/16645) | @Goutham-Annem | test: add unit tests for vpol validate and compiler packages |  | Y | Y | MERGEABLE | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16642](https://github.com/kyverno/kyverno/pull/16642) | @aditip149209 | feat: stable autogen rule names via optional validation identifier |  | Y | Y | CONFLICTING | 0/10/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16627](https://github.com/kyverno/kyverno/pull/16627) | @AYUSH-P-SINGH | fix(cli): remove double slashes for cluster-scoped resources in kyvern |  | Y | Y | MERGEABLE | 0/0/2 | CEL_CONTENT | SKIPPED | `type_cel` | Keep on main |
| [#16565](https://github.com/kyverno/kyverno/pull/16565) | @AYUSH-P-SINGH | fix(policy): evaluate CEL matchConditions before creating UpdateReques |  | Y | Y | CONFLICTING | 0/1/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16524](https://github.com/kyverno/kyverno/pull/16524) | @ANAMASGARD | fix(cel/autogen): preserve object.metadata.name in autogen messageExpr |  | Y | Y | MERGEABLE | 0/3/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16481](https://github.com/kyverno/kyverno/pull/16481) | @naseyro | Enable validationActions override through PolicyException |  | Y | Y | MERGEABLE | 0/29/2 |  | SKIPPED | `type_cel` | Keep on main |
| [#16463](https://github.com/kyverno/kyverno/pull/16463) | @atharrva01 | Add status conditions to DeletingPolicy |  | Y | Y | MERGEABLE | 0/6/8 |  | SKIPPED | `type_cel` | Keep on main |
| [#16335](https://github.com/kyverno/kyverno/pull/16335) | @LoveChauhan-18 | Fix ImageValidatingPolicy bypass for ephemeral containers |  | Y | Y | CONFLICTING | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16288](https://github.com/kyverno/kyverno/pull/16288) | @ANAMASGARD | fix: always re-arm background scan and TTL controllers after transient |  | Y | Y | MERGEABLE | 0/0/4 | CEL_CONTENT | SKIPPED | `type_cel` | Keep on main |
| [#16271](https://github.com/kyverno/kyverno/pull/16271) | @JimBugwadia | Add standalone attestation CEL library for ValidatingPolicy |  | Y | Y | CONFLICTING | 0/6/5 |  | SKIPPED | `type_cel` | Keep on main |
| [#16253](https://github.com/kyverno/kyverno/pull/16253) | @JimBugwadia | feat: add standalone attestation CEL library for ValidatingPolicy |  | Y | Y | CONFLICTING | 0/6/5 |  | SKIPPED | `type_cel` | Keep on main |
| [#16172](https://github.com/kyverno/kyverno/pull/16172) | @karthikmanam | feat: emit events for report kind resolution failures |  | Y | Y | CONFLICTING | 0/0/4 | CEL_CONTENT | SKIPPED | `type_cel` | Keep on main |
| [#16118](https://github.com/kyverno/kyverno/pull/16118) | @krrishverma1805-web | fix(webhooks): propagate request context for toggle resolution in mpol |  | Y | Y | MERGEABLE | 0/1/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16104](https://github.com/kyverno/kyverno/pull/16104) | @tejassinghbhati | Fix/ivpol cel logging |  | Y | Y | CONFLICTING | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16084](https://github.com/kyverno/kyverno/pull/16084) | @jimgus | fix(ivpol): verify sigstore bundles via sigstore-go to honour user-pro |  | Y | Y | CONFLICTING | 0/3/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16061](https://github.com/kyverno/kyverno/pull/16061) | @aaa-aashna | feat: improve image verification debug logging |  | Y | Y | CONFLICTING | 0/3/4 |  | SKIPPED | `type_cel` | Keep on main |
| [#16049](https://github.com/kyverno/kyverno/pull/16049) | @aaa-aashna | feat: improve image verification debug logging |  | Y | Y | CONFLICTING | 0/3/6 |  | SKIPPED | `type_cel` | Keep on main |
| [#16048](https://github.com/kyverno/kyverno/pull/16048) | @jimgus | fix(ivpol): merge user-provided TSA chain into TrustedMaterial for bun | Y | Y | Y | CONFLICTING | 0/3/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16035](https://github.com/kyverno/kyverno/pull/16035) | @ZhangDT-sky | fix: add IVPol image verification logs |  | Y | Y | CONFLICTING | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16022](https://github.com/kyverno/kyverno/pull/16022) | @madmecodes | fix: add success logging for MutatingPolicy mutations |  | Y | Y | MERGEABLE | 0/1/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16013](https://github.com/kyverno/kyverno/pull/16013) | @jimgus | fix(ivpol): don't require rekor URL when InsecureIgnoreTlog is true |  | Y | Y | CONFLICTING | 0/2/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#16008](https://github.com/kyverno/kyverno/pull/16008) | @naseyro | feat: Add support for `additionalExtensions` for cosign keyless signin | Y | Y | Y | CONFLICTING | 0/8/1 |  | SKIPPED | `type_cel` | Keep on main |
| [#15903](https://github.com/kyverno/kyverno/pull/15903) | @vivekmaurya001 | test: add vpol engine coverage |  | Y | Y | CONFLICTING | 0/4/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#15889](https://github.com/kyverno/kyverno/pull/15889) | @sandert-k8s | feat: add observedGeneration in status | Y | Y | Y | CONFLICTING | 0/3/1 |  | SKIPPED | `type_cel` | Keep on main |
| [#15794](https://github.com/kyverno/kyverno/pull/15794) | @vivekmaurya001 | test: add vpol engine coverage |  | Y | Y | CONFLICTING | 0/1/0 |  | SKIPPED | `type_cel` | Keep on main |
| [#15783](https://github.com/kyverno/kyverno/pull/15783) | @LukeTimeWalker | feat(helm): add new cel policy types to clusterroles admin:policies an |  | Y | Y | CONFLICTING | 0/0/1 | CEL_CONTENT | SKIPPED | `type_cel` | Keep on main |
| [#15695](https://github.com/kyverno/kyverno/pull/15695) | @eddycharly | refactor: decouple polex source/compiler | Y | Y | Y | CONFLICTING | 0/10/7 |  | SKIPPED | `type_cel` | Keep on main |
| [#15479](https://github.com/kyverno/kyverno/pull/15479) | @jzeng4 | feat(ivpol): add CEL expression support for keyless identity fields |  | Y | Y | CONFLICTING | 0/3/3 |  | SKIPPED | `type_cel` | Keep on main |
| [#15388](https://github.com/kyverno/kyverno/pull/15388) | @doom | Fix mapping of CRDs from ClusterResource in CLI tests |  | Y | Y | CONFLICTING | 0/16/3 |  | SKIPPED | `type_cel` | Keep on main |
| [#15234](https://github.com/kyverno/kyverno/pull/15234) | @JagjeevanAK | feat: add fine-grained CEL exceptions (Images/AllowedValues) for Mutat |  | Y | Y | CONFLICTING | 0/28/1 |  | SKIPPED | `type_cel` | Keep on main |

## SHARED_ONLY (96) — label `type_shared`

| PR | Author | Title | Draft | Fork | CanModify | Mergeable | Files (L/C/S) | Content signal | Probe | Proposed label | Proposed Action |
|---|---|---|---|---|---|---|---|---|---|---|---|
| [#17564](https://github.com/kyverno/kyverno/pull/17564) | @sapnilbiswas | Fix structured logging anti-patterns |  | Y | Y | MERGEABLE | 0/0/4 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17560](https://github.com/kyverno/kyverno/pull/17560) | @Ahmad-Faraj | fix(cli): close the junit failure element without detailed results |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17552](https://github.com/kyverno/kyverno/pull/17552) | @yshngg | refactor: 'Got empty response for' no longer returned by methods of ca |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17540](https://github.com/kyverno/kyverno/pull/17540) | @shashankvarma499 | fix(helm): null out PDB minAvailable default so maxUnavailable overrid |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17533](https://github.com/kyverno/kyverno/pull/17533) | @gdziwoki | fix(helm): grant reports-controller RBAC on DeletingPolicy/NamespacedD |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17524](https://github.com/kyverno/kyverno/pull/17524) | @badnikhil | fix: stop leaking the aggregate report cleanup goroutine |  | Y | Y | MERGEABLE | 0/0/3 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17506](https://github.com/kyverno/kyverno/pull/17506) | @SumitDalavi | fix(cli): evaluate JMESPath syntax errors at compile time in jp |  | Y | Y | MERGEABLE | 0/0/4 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17481](https://github.com/kyverno/kyverno/pull/17481) | @ANAMASGARD | fix(helm): allow OpenShift to assign container UID and GID |  | Y | Y | MERGEABLE | 0/0/5 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17469](https://github.com/kyverno/kyverno/pull/17469) | @tendinginfinity24 | fix(webhook): fix concurrent map race in expressionCache.GetOrCompile |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17457](https://github.com/kyverno/kyverno/pull/17457) | @7se7en72025 | fix(webhook): guard nil Scope dereference in sortedRules to prevent co |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17407](https://github.com/kyverno/kyverno/pull/17407) | @AYUSH-P-SINGH | fix(config): reset matchConditions and updateRequestThreshold on unloa |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17404](https://github.com/kyverno/kyverno/pull/17404) | @ShreyanshK1103 | fix: reconcile validating policy reports on match constraint changes |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17338](https://github.com/kyverno/kyverno/pull/17338) | @1PoPTRoN | fix: prevent command injection via PR title in cherry-pick action |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17333](https://github.com/kyverno/kyverno/pull/17333) | @waterWang | fix: use correct function name constant in jpTimeAdd error messages |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17315](https://github.com/kyverno/kyverno/pull/17315) | @Hiten0305l | fix(jmespath): correct function name for invalid time_add arguments |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17289](https://github.com/kyverno/kyverno/pull/17289) | @waterWang | fix: return after writing 500 in Probe health check handler |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17259](https://github.com/kyverno/kyverno/pull/17259) | @waterWang | fix: fallback to direct client lookup for secrets in namespaces not co |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17207](https://github.com/kyverno/kyverno/pull/17207) | @myukitty | fix(webhook): avoid nil pointer dereference in sortedRules when rule S |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17193](https://github.com/kyverno/kyverno/pull/17193) | @waterWang | fix: correct GetResourceGVR boundary length calculation for 64-65 char |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17192](https://github.com/kyverno/kyverno/pull/17192) | @Hiten0305l | fix(reports): resolve GVR split-label reconstruction length mismatch |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17179](https://github.com/kyverno/kyverno/pull/17179) | @vishalmore90 | fix(controllers/ttl): fix resController data race and premature loop e |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17177](https://github.com/kyverno/kyverno/pull/17177) | @Manoj-Kumar-Selvaraj | ci: automate KyvernoLatest SDK bump PRs on release |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17172](https://github.com/kyverno/kyverno/pull/17172) | @rajanpanth | docs: remove stale ko dev publish targets |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17170](https://github.com/kyverno/kyverno/pull/17170) | @rajanpanth | docs: fix deepcopy codegen target |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17169](https://github.com/kyverno/kyverno/pull/17169) | @rajanpanth | docs: fix policy report CRD codegen target |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17166](https://github.com/kyverno/kyverno/pull/17166) | @vishalmore90 | fix(cli): prevent silent failures when loading invalid cluster resourc |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17143](https://github.com/kyverno/kyverno/pull/17143) | @Abhinash-Singh | fix: default helm test image tag to chart appVersion |  | Y | Y | MERGEABLE | 0/0/3 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17137](https://github.com/kyverno/kyverno/pull/17137) | @pujitha24 | fix(cli): stop WorkerPool from silently dropping tasks in resource loa |  | Y | Y | MERGEABLE | 0/0/4 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17133](https://github.com/kyverno/kyverno/pull/17133) | @Hiten0305l | fix(webhook): prevent nil scope panic when sorting rules |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17113](https://github.com/kyverno/kyverno/pull/17113) | @Hiten0305l | fix(jmespath): correct time_add validation errors |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17111](https://github.com/kyverno/kyverno/pull/17111) | @AYUSH-P-SINGH | fix(webhooks): dynamically evaluate Kyverno namespace in protection ha |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17090](https://github.com/kyverno/kyverno/pull/17090) | @Hiten0305l | fix: handle semver parsing errors |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17083](https://github.com/kyverno/kyverno/pull/17083) | @itsvishalyadav | Fix uncancellable DNS lookup DoS vector in JMESPath |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17044](https://github.com/kyverno/kyverno/pull/17044) | @itsvishalyadav | Fix time.After leak and select break in resource loader |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17041](https://github.com/kyverno/kyverno/pull/17041) | @alina-cloudsec | docs: update copyright year range in root readme |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#17022](https://github.com/kyverno/kyverno/pull/17022) | @Mahmoud-Khawaja | fix: goroutine leak when reports counter init partially fails |  | Y | Y | CONFLICTING | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16996](https://github.com/kyverno/kyverno/pull/16996) | @KeerthiKumarR | test: add comprehensive unit tests for pkg/utils/runtime |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16986](https://github.com/kyverno/kyverno/pull/16986) | @ABHIGYA0205 | fix(cli): show syntax highlight for JMESPath compile errors in jp quer |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16985](https://github.com/kyverno/kyverno/pull/16985) | @ABHIGYA0205 | fix(cli): remove unreachable dead code in createRowsAccordingToResults |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16981](https://github.com/kyverno/kyverno/pull/16981) | @karthikmanam | fix(cli): aggregate test case selector errors |  | Y | Y | CONFLICTING | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16951](https://github.com/kyverno/kyverno/pull/16951) | @SashaMIT | ci: bind cherry-pick composite inputs via env before shell |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16931](https://github.com/kyverno/kyverno/pull/16931) | @omlahore | fix(jmespath): guard random() argument type |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16930](https://github.com/kyverno/kyverno/pull/16930) | @YuvrajVerma09arch | fix(make): correct go mod tidy check in unused-package-check target |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16927](https://github.com/kyverno/kyverno/pull/16927) | @falloficaruss | fix: lookup background scan reports by resource UID label selector |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16899](https://github.com/kyverno/kyverno/pull/16899) | @RajdeepKushwaha5 | ci: run the orphaned conformance suites and guard against new ones |  | Y | Y | MERGEABLE | 0/0/5 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16898](https://github.com/kyverno/kyverno/pull/16898) | @FirePheonix | fix: correctly skip YAML document separators and clean up Makefile TOD |  | Y | Y | MERGEABLE | 0/0/3 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16882](https://github.com/kyverno/kyverno/pull/16882) | @CodingRI | feat(cli): display policy validation warnings in apply command |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16876](https://github.com/kyverno/kyverno/pull/16876) | @FirePheonix | fix: parseKinds should return an empty slice for empty array string |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16871](https://github.com/kyverno/kyverno/pull/16871) | @aryanghai12 | fix: reverse iteration in deleteAnchorsInList to prevent container del |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16848](https://github.com/kyverno/kyverno/pull/16848) | @AYUSH-P-SINGH | fix(webhooks): support RFC-compliant Content-Type media type parsing |  | Y | Y | MERGEABLE | 0/0/3 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16811](https://github.com/kyverno/kyverno/pull/16811) | @akshita317 | fix: match wildcard groups in GroupVersionMatches |  | Y | Y | MERGEABLE | 0/0/3 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16796](https://github.com/kyverno/kyverno/pull/16796) | @AyachiMishra | fix(jsonpointer): prevent backing array aliasing in Append, Prepend an |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16783](https://github.com/kyverno/kyverno/pull/16783) | @Iqrima | fix(helm): use block-style rendering for podAnnotations in controller… |  | Y | Y | MERGEABLE | 0/0/4 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16780](https://github.com/kyverno/kyverno/pull/16780) | @Iqrima | fix(helm): remove redundant metricsService guard from background-cont… |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16758](https://github.com/kyverno/kyverno/pull/16758) | @Iqrima | fix: resolve unnamed jsonpointer test cases and correct comment typo |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16750](https://github.com/kyverno/kyverno/pull/16750) | @jojinkb | fix: panic in RedactSecret on empty annotations and unreachable error  |  | Y | Y | CONFLICTING | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16714](https://github.com/kyverno/kyverno/pull/16714) | @Iqrima | fix: make git policy file extension check case-insensitive |  | Y | Y | MERGEABLE | 0/0/4 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16712](https://github.com/kyverno/kyverno/pull/16712) | @n0liu | fix: drop empty patch arrays in JoinPatches |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16711](https://github.com/kyverno/kyverno/pull/16711) | @Iqrima | fix: ignore empty bracket patches in JoinPatches |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16702](https://github.com/kyverno/kyverno/pull/16702) | @Iqrima | fix: enforce policy and rule matching in policy exception selector |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16646](https://github.com/kyverno/kyverno/pull/16646) | @Goutham-Annem | test: add unit tests for ext/file-info, ext/yaml, CLI deprecations, an |  | Y | Y | MERGEABLE | 0/0/4 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16582](https://github.com/kyverno/kyverno/pull/16582) | @ShubhamArora073 | fix(cli): improve JMESPath syntax error messages in jp query | Y | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16568](https://github.com/kyverno/kyverno/pull/16568) | @aerosouund | fix: endpoint slice pass by value bug in the readiness checker |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16548](https://github.com/kyverno/kyverno/pull/16548) | @mailnike | fix: prevent panic in `kyverno test` on a git URL without a path |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16519](https://github.com/kyverno/kyverno/pull/16519) | @rohithnarasimha | feat: support updated SDK image library |  | Y | Y | CONFLICTING | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16514](https://github.com/kyverno/kyverno/pull/16514) | @ShubhamArora073 | fix: align maxAPICallResponseLength flag description across controller |  | Y | Y | CONFLICTING | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16513](https://github.com/kyverno/kyverno/pull/16513) | @ShubhamArora073 | fix: make unused-package-check detect go.mod/go.sum changes |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16512](https://github.com/kyverno/kyverno/pull/16512) | @filariow | chore(deps): bump github.com/go-git/go-billy/v5 from 5.8.0 to 5.9.1 |  | Y | Y | CONFLICTING | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16496](https://github.com/kyverno/kyverno/pull/16496) | @CodeBlackwell | Attribute time_add argument errors to time_add, not time_to_cron |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16440](https://github.com/kyverno/kyverno/pull/16440) | @ANAMASGARD | fix(reports): prevent orphan PolicyReports by setting ownerReferences  |  | Y | Y | MERGEABLE | 0/0/3 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16437](https://github.com/kyverno/kyverno/pull/16437) | @jsabalete | fix: truncate ValidatingAdmissionPolicyBinding names that exceed the 6 |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16430](https://github.com/kyverno/kyverno/pull/16430) | @sage-mode-hunter | fix(jmespath): treat link-local IPs as internal in is_external_url |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16428](https://github.com/kyverno/kyverno/pull/16428) | @somaz94 | feat(helm): add opt-in schedulerName and runtimeClassName to the contr |  | Y | Y | MERGEABLE | 0/0/6 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16364](https://github.com/kyverno/kyverno/pull/16364) | @D-source1602 | fix: close file descriptor in printOutput to prevent handle leak |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16294](https://github.com/kyverno/kyverno/pull/16294) | @sage-mode-hunter | contain policy name when writing pulled oci policies |  | Y | Y | CONFLICTING | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16286](https://github.com/kyverno/kyverno/pull/16286) | @Prachidg | feat: add text-nocolor logging format option |  | Y | Y | MERGEABLE | 0/0/4 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16265](https://github.com/kyverno/kyverno/pull/16265) | @karthikmanam | feat(cli): support cross-resource policy evaluation |  | Y | Y | CONFLICTING | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16248](https://github.com/kyverno/kyverno/pull/16248) | @Moglum | improve handling of empty/comments-only files |  | Y | Y | MERGEABLE | 0/0/6 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16223](https://github.com/kyverno/kyverno/pull/16223) | @CygnusMaximillian | Fix: stop policy cache invalidation storm and support plural resource  |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16204](https://github.com/kyverno/kyverno/pull/16204) | @vigneshakaviki | fix: make unused-package-check fail when go mod tidy changes files (#1 |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16202](https://github.com/kyverno/kyverno/pull/16202) | @ObaidAbdullah16 | [docs] clarify Proof Manifest expectations for contributors |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16201](https://github.com/kyverno/kyverno/pull/16201) | @lmasaya | fix: handle multi-cert PEM bundle in decodeTLSSecret |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16120](https://github.com/kyverno/kyverno/pull/16120) | @krrishverma1805-web | fix: align maxAPICallResponseLength flag description with other contro |  | Y | Y | CONFLICTING | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16092](https://github.com/kyverno/kyverno/pull/16092) | @WildTrio | fix: make unused-package-check fail on tidy changes |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16007](https://github.com/kyverno/kyverno/pull/16007) | @jakharmonika364 | fix(cli): show mutate test diff without --detailed-results |  | Y | Y | MERGEABLE | 0/0/3 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#16006](https://github.com/kyverno/kyverno/pull/16006) | @apshada | test(utils/git): add IsYaml unit test |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#15941](https://github.com/kyverno/kyverno/pull/15941) | @Famous077 | feat(cli): add sysdump command for diagnostic data collection |  | Y | Y | CONFLICTING | 0/0/3 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#15895](https://github.com/kyverno/kyverno/pull/15895) | @malsomesh9 | [codex] Report skipped policy load errors |  | Y | Y | CONFLICTING | 0/0/3 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#15819](https://github.com/kyverno/kyverno/pull/15819) | @Rama542 | docs: fix ArgoCD platform notes (kyverno/website#1925) |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#15785](https://github.com/kyverno/kyverno/pull/15785) | @vikash232 | fix(logging): reduce klog Event occurred noise at -v 0–1 |  | Y | Y | MERGEABLE | 0/0/3 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#15720](https://github.com/kyverno/kyverno/pull/15720) | @yashaswikakumanu | Revise ArgoCD sync options and resource exclusions |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#15711](https://github.com/kyverno/kyverno/pull/15711) | @pierluigilenoci | Honor stderrthreshold when logtostderr is enabled |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#15650](https://github.com/kyverno/kyverno/pull/15650) | @omad | fix: handle wildcard `match.resource` selectors in background policy p |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#15633](https://github.com/kyverno/kyverno/pull/15633) | @raajheshkannaa | fix: enable hook-delete-policy on migrate-resources post-upgrade Job |  | Y | Y | MERGEABLE | 0/0/1 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#15269](https://github.com/kyverno/kyverno/pull/15269) | @fe80 | fix: correct empty image repository with library |  | Y | Y | MERGEABLE | 0/0/2 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |
| [#14685](https://github.com/kyverno/kyverno/pull/14685) | @hiirrxnn | fix: add subPath support to global.caCertificates.volume |  | Y | Y | MERGEABLE | 0/0/4 | NEUTRAL | SKIPPED | `type_shared` | Keep on main (verify after legacy removal lands) |

## REVIEW-MIGRATION (5) — label none (human review)

| PR | Author | Title | Draft | Fork | CanModify | Mergeable | Files (L/C/S) | Content signal | Probe | Proposed label | Proposed Action |
|---|---|---|---|---|---|---|---|---|---|---|---|
| [#17554](https://github.com/kyverno/kyverno/pull/17554) | @raunak-nirmata | fix: preserve legacy report sources and webhook API versions in 1.20 |  | Y | Y | MERGEABLE | 0/0/5 | MIXED_CONTENT | SKIPPED | — | Keep on main — requires human confirmation (migration-grace, see §1.5) |
| [#17553](https://github.com/kyverno/kyverno/pull/17553) | @Rohanraj123 | feat(cli): block legacy policy manifests in apply/test, add migration  |  | Y | Y | MERGEABLE | 1/0/24 | MIXED_CONTENT | SKIPPED | — | Keep on main — requires human confirmation (migration-grace, see §1.5) |
| [#17534](https://github.com/kyverno/kyverno/pull/17534) | @raunak-nirmata | feat(helm): add required default-on gate for legacy policy CRs on inst |  | Y | Y | MERGEABLE | 0/0/14 | MIXED_CONTENT | SKIPPED | — | Keep on main — requires human confirmation (migration-grace, see §1.5) |
| [#17519](https://github.com/kyverno/kyverno/pull/17519) | @raunak-nirmata | feat: signal remaining legacy kyverno.io policies on startup and via m |  | Y | Y | MERGEABLE | 1/0/16 | MIXED_CONTENT | SKIPPED | — | Keep on main — requires human confirmation (migration-grace, see §1.5) |
| [#17494](https://github.com/kyverno/kyverno/pull/17494) | @thisis-manan | feat(helm): protect legacy policy CRDs from Helm prune with resource-p |  | Y | Y | MERGEABLE | 11/0/15 | MIXED_CONTENT | SKIPPED | — | Keep on main — requires human confirmation (migration-grace, see §1.5) |
