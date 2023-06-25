package engine

import (
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/kyverno/kyverno/pkg/utils/image"
	"gotest.tools/assert"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var scanPredicate = `
{
    "predicate": {
        "matches": [
            {
                "vulnerability": {
                    "id": "CVE-2021-22946",
                    "dataSource": "http://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-22946",
                    "namespace": "alpine:3.11",
                    "severity": "High",
                    "urls": [
                        "http://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-22946"
                    ],
                    "cvss": [],
                    "fix": {
                        "versions": [
                            "7.79.0-r0"
                        ],
                        "state": "fixed"
                    },
                    "advisories": []
                },
                "relatedVulnerabilities": [
                    {
                        "id": "CVE-2021-22946",
                        "dataSource": "https://nvd.nist.gov/vuln/detail/CVE-2021-22946",
                        "namespace": "nvd",
                        "severity": "High",
                        "urls": [
                            "https://hackerone.com/reports/1334111",
                            "https://lists.debian.org/debian-lts-announce/2021/09/msg00022.html",
                            "https://lists.fedoraproject.org/archives/list/package-announce@lists.fedoraproject.org/message/RWLEC6YVEM2HWUBX67SDGPSY4CQB72OE/",
                            "https://www.oracle.com/security-alerts/cpuoct2021.html",
                            "https://security.netapp.com/advisory/ntap-20211029-0003/",
                            "https://lists.fedoraproject.org/archives/list/package-announce@lists.fedoraproject.org/message/APOAK4X73EJTAPTSVT7IRVDMUWVXNWGD/",
                            "https://security.netapp.com/advisory/ntap-20220121-0008/",
                            "https://www.oracle.com/security-alerts/cpujan2022.html",
                            "https://cert-portal.siemens.com/productcert/pdf/ssa-389290.pdf",
                            "https://support.apple.com/kb/HT213183",
                            "http://seclists.org/fulldisclosure/2022/Mar/29",
                            "https://www.oracle.com/security-alerts/cpuapr2022.html"
                        ],
                        "description": "A user can tell curl >= 7.20.0 and <= 7.78.0 to require a successful upgrade to TLS when speaking to an IMAP, POP3 or FTP server (--ssl-reqd on the command line or CURLOPT_USE_SSL set to CURLUSESSL_CONTROL or CURLUSESSL_ALL withlibcurl). This requirement could be bypassed if the server would return a properly crafted but perfectly legitimate response.This flaw would then make curl silently continue its operations **withoutTLS** contrary to the instructions and expectations, exposing possibly sensitive data in clear text over the network.",
                        "cvss": [
                            {
                                "version": "2.0",
                                "vector": "AV:N/AC:L/Au:N/C:P/I:N/A:N",
                                "metrics": {
                                    "baseScore": 5,
                                    "exploitabilityScore": 10,
                                    "impactScore": 2.9
                                },
                                "vendorMetadata": {}
                            },
                            {
                                "version": "3.1",
                                "vector": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:N",
                                "metrics": {
                                    "baseScore": 7.5,
                                    "exploitabilityScore": 3.9,
                                    "impactScore": 3.6
                                },
                                "vendorMetadata": {}
                            }
                        ]
                    }
                ],
                "matchDetails": [
                    {
                        "matcher": "apk-matcher",
                        "searchedBy": {
                            "distro": {
                                "type": "alpine",
                                "version": "3.11.7"
                            },
                            "namespace": "alpine:3.11",
                            "package": {
                                "name": "curl",
                                "version": "7.67.0-r3"
                            }
                        },
                        "found": {
                            "versionConstraint": "< 7.79.0-r0 (apk)"
                        }
                    }
                ],
                "artifact": {
                    "name": "libcurl",
                    "version": "7.67.0-r3",
                    "type": "apk",
                    "locations": [
                        {
                            "path": "/lib/apk/db/installed",
                            "layerID": "sha256:165c22a332e306497ffa210ce9f284906fe0bf6340d20c5f8521e064323ba52a"
                        }
                    ],
                    "language": "",
                    "licenses": [
                        "MIT"
                    ],
                    "cpes": [
                        "cpe:2.3:a:libcurl:libcurl:7.67.0-r3:*:*:*:*:*:*:*"
                    ],
                    "purl": "pkg:alpine/libcurl@7.67.0-r3?arch=x86_64",
                    "metadata": {
                        "OriginPackage": "curl"
                    }
                }
            }
        ],
        "source": {
            "type": "image",
            "target": {
                "userInput": "ghcr.io/tap8stry/git-init:v0.21.0@sha256:322e3502c1e6fba5f1869efb55cfd998a3679e073840d33eb0e3c482b5d5609b",
                "imageID": "sha256:ebbe9df4abf4dd9a739b33ab75d1fee2086713829a437f9d1e5e3de7b21e8d5f",
                "manifestDigest": "sha256:5fe577767eba4cca2fe7594f6df94ca2b0f639a2ee8794f99f2ac49b81b5d419",
                "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
                "tags": [
                    "ghcr.io/tap8stry/git-init:v0.21.0"
                ],
                "imageSize": 81343568,
                "layers": [
                    {
                        "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
                        "digest": "sha256:0fcbbeeeb0d7fc5c06362d7a6717b999e605574c7210eff4f7418f6e9be9fbfe",
                        "size": 5610661
                    },
                    {
                        "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
                        "digest": "sha256:8f1d7de99bcffd39d4461b917a5313cfa0415f33eac9412a9b6138a27121c7e6",
                        "size": 4686
                    },
                    {
                        "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
                        "digest": "sha256:165c22a332e306497ffa210ce9f284906fe0bf6340d20c5f8521e064323ba52a",
                        "size": 37280295
                    },
                    {
                        "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
                        "digest": "sha256:0519ec3ee06ceaaa19b5682db6a01d408f9be6d74dc0f453e416fc92b654ce2f",
                        "size": 9503892
                    },
                    {
                        "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
                        "digest": "sha256:a79b016ff7714845775dd921c15fab70652344015e91ebff2ccec49d6792ac11",
                        "size": 28944034
                    }
                ],
                "manifest": "eyJzY2hlbWFWZXJzaW9uIjoyLCJtZWRpYVR5cGUiOiJhcHBsaWNhdGlvbi92bmQuZG9ja2VyLmRpc3RyaWJ1dGlvbi5tYW5pZmVzdC52Mitqc29uIiwiY29uZmlnIjp7Im1lZGlhVHlwZSI6ImFwcGxpY2F0aW9uL3ZuZC5kb2NrZXIuY29udGFpbmVyLmltYWdlLnYxK2pzb24iLCJzaXplIjoxNzM1LCJkaWdlc3QiOiJzaGEyNTY6ZWJiZTlkZjRhYmY0ZGQ5YTczOWIzM2FiNzVkMWZlZTIwODY3MTM4MjlhNDM3ZjlkMWU1ZTNkZTdiMjFlOGQ1ZiJ9LCJsYXllcnMiOlt7Im1lZGlhVHlwZSI6ImFwcGxpY2F0aW9uL3ZuZC5kb2NrZXIuaW1hZ2Uucm9vdGZzLmRpZmYudGFyLmd6aXAiLCJzaXplIjo1ODgxMzQ0LCJkaWdlc3QiOiJzaGEyNTY6MGZjYmJlZWViMGQ3ZmM1YzA2MzYyZDdhNjcxN2I5OTllNjA1NTc0YzcyMTBlZmY0Zjc0MThmNmU5YmU5ZmJmZSJ9LHsibWVkaWFUeXBlIjoiYXBwbGljYXRpb24vdm5kLmRvY2tlci5pbWFnZS5yb290ZnMuZGlmZi50YXIuZ3ppcCIsInNpemUiOjExNzc2LCJkaWdlc3QiOiJzaGEyNTY6OGYxZDdkZTk5YmNmZmQzOWQ0NDYxYjkxN2E1MzEzY2ZhMDQxNWYzM2VhYzk0MTJhOWI2MTM4YTI3MTIxYzdlNiJ9LHsibWVkaWFUeXBlIjoiYXBwbGljYXRpb24vdm5kLmRvY2tlci5pbWFnZS5yb290ZnMuZGlmZi50YXIuZ3ppcCIsInNpemUiOjM3NzQ5NzYwLCJkaWdlc3QiOiJzaGEyNTY6MTY1YzIyYTMzMmUzMDY0OTdmZmEyMTBjZTlmMjg0OTA2ZmUwYmY2MzQwZDIwYzVmODUyMWUwNjQzMjNiYTUyYSJ9LHsibWVkaWFUeXBlIjoiYXBwbGljYXRpb24vdm5kLmRvY2tlci5pbWFnZS5yb290ZnMuZGlmZi50YXIuZ3ppcCIsInNpemUiOjk2NTIyMjQsImRpZ2VzdCI6InNoYTI1NjowNTE5ZWMzZWUwNmNlYWFhMTliNTY4MmRiNmEwMWQ0MDhmOWJlNmQ3NGRjMGY0NTNlNDE2ZmM5MmI2NTRjZTJmIn0seyJtZWRpYVR5cGUiOiJhcHBsaWNhdGlvbi92bmQuZG9ja2VyLmltYWdlLnJvb3Rmcy5kaWZmLnRhci5nemlwIiwic2l6ZSI6Mjg5NDY0MzIsImRpZ2VzdCI6InNoYTI1NjphNzliMDE2ZmY3NzE0ODQ1Nzc1ZGQ5MjFjMTVmYWI3MDY1MjM0NDAxNWU5MWViZmYyY2NlYzQ5ZDY3OTJhYzExIn1dfQ==",
                "config": "eyJhcmNoaXRlY3R1cmUiOiJhbWQ2NCIsImF1dGhvciI6ImdpdGh1Yi5jb20vZ29vZ2xlL2tvIiwiY3JlYXRlZCI6IjIwMjEtMDItMTZUMTk6MzU6NDNaIiwiaGlzdG9yeSI6W3siY3JlYXRlZCI6IjIwMjAtMTItMTdUMDA6MTk6NDkuMTEyNDQ1OTY1WiIsImNyZWF0ZWRfYnkiOiIvYmluL3NoIC1jICMobm9wKSBBREQgZmlsZTo4ZWQ4MDAxMGU0NDNkYTE5ZDcyNTQ2YmNlZTlhMzVlMGE4ZDI0NGM3MjA1MmIxOTk0NjEwYmY1OTM5ZDQ3OWMyIGluIC8gIn0seyJjcmVhdGVkIjoiMjAyMC0xMi0xN1QwMDoxOTo0OS4yODQyMTExNDhaIiwiY3JlYXRlZF9ieSI6Ii9iaW4vc2ggLWMgIyhub3ApICBDTUQgW1wiL2Jpbi9zaFwiXSIsImVtcHR5X2xheWVyIjp0cnVlfSx7ImNyZWF0ZWQiOiIyMDIxLTAyLTE2VDE5OjM1OjI4LjkzODk1MjU1M1oiLCJjcmVhdGVkX2J5IjoiUlVOIC9iaW4vc2ggLWMgYWRkZ3JvdXAgLVMgLWcgNjU1MzIgbm9ucm9vdCBcdTAwMjZcdTAwMjYgYWRkdXNlciAtUyAtdSA2NTUzMiBub25yb290IC1HIG5vbnJvb3QgIyBidWlsZGtpdCIsImNvbW1lbnQiOiJidWlsZGtpdC5kb2NrZXJmaWxlLnYwIn0seyJjcmVhdGVkIjoiMjAyMS0wMi0xNlQxOTozNTozMi41NTU3NTk2NTRaIiwiY3JlYXRlZF9ieSI6IlJVTiAvYmluL3NoIC1jIGFwayBhZGQgLS11cGRhdGUgZ2l0IGdpdC1sZnMgb3BlbnNzaC1jbGllbnQgICAgIFx1MDAyNlx1MDAyNiBhcGsgdXBkYXRlICAgICBcdTAwMjZcdTAwMjYgYXBrIHVwZ3JhZGUgIyBidWlsZGtpdCIsImNvbW1lbnQiOiJidWlsZGtpdC5kb2NrZXJmaWxlLnYwIn0seyJhdXRob3IiOiJrbyIsImNyZWF0ZWQiOiIwMDAxLTAxLTAxVDAwOjAwOjAwWiIsImNyZWF0ZWRfYnkiOiJrbyBwdWJsaXNoIGtvOi8vZ2l0aHViLmNvbS90ZWt0b25jZC9waXBlbGluZS9jbWQvZ2l0LWluaXQiLCJjb21tZW50Ijoia29kYXRhIGNvbnRlbnRzLCBhdCAkS09fREFUQV9QQVRIIn0seyJhdXRob3IiOiJrbyIsImNyZWF0ZWQiOiIwMDAxLTAxLTAxVDAwOjAwOjAwWiIsImNyZWF0ZWRfYnkiOiJrbyBwdWJsaXNoIGtvOi8vZ2l0aHViLmNvbS90ZWt0b25jZC9waXBlbGluZS9jbWQvZ2l0LWluaXQiLCJjb21tZW50IjoiZ28gYnVpbGQgb3V0cHV0LCBhdCAva28tYXBwL2dpdC1pbml0In1dLCJvcyI6ImxpbnV4Iiwicm9vdGZzIjp7InR5cGUiOiJsYXllcnMiLCJkaWZmX2lkcyI6WyJzaGEyNTY6MGZjYmJlZWViMGQ3ZmM1YzA2MzYyZDdhNjcxN2I5OTllNjA1NTc0YzcyMTBlZmY0Zjc0MThmNmU5YmU5ZmJmZSIsInNoYTI1Njo4ZjFkN2RlOTliY2ZmZDM5ZDQ0NjFiOTE3YTUzMTNjZmEwNDE1ZjMzZWFjOTQxMmE5YjYxMzhhMjcxMjFjN2U2Iiwic2hhMjU2OjE2NWMyMmEzMzJlMzA2NDk3ZmZhMjEwY2U5ZjI4NDkwNmZlMGJmNjM0MGQyMGM1Zjg1MjFlMDY0MzIzYmE1MmEiLCJzaGEyNTY6MDUxOWVjM2VlMDZjZWFhYTE5YjU2ODJkYjZhMDFkNDA4ZjliZTZkNzRkYzBmNDUzZTQxNmZjOTJiNjU0Y2UyZiIsInNoYTI1NjphNzliMDE2ZmY3NzE0ODQ1Nzc1ZGQ5MjFjMTVmYWI3MDY1MjM0NDAxNWU5MWViZmYyY2NlYzQ5ZDY3OTJhYzExIl19LCJjb25maWciOnsiQ21kIjpbIi9iaW4vc2giXSwiRW50cnlwb2ludCI6WyIva28tYXBwL2dpdC1pbml0Il0sIkVudiI6WyJQQVRIPS91c3IvbG9jYWwvc2JpbjovdXNyL2xvY2FsL2JpbjovdXNyL3NiaW46L3Vzci9iaW46L3NiaW46L2Jpbjova28tYXBwIiwiS09fREFUQV9QQVRIPS92YXIvcnVuL2tvIl19fQ==",
                "repoDigests": [
                    "ghcr.io/tap8stry/git-init@sha256:322e3502c1e6fba5f1869efb55cfd998a3679e073840d33eb0e3c482b5d5609b"
                ]
            }
        },
        "distro": {
            "name": "Alpine Linux",
            "version": "",
            "idLike": null
        },
        "descriptor": {
            "name": "grype",
            "version": "0.32.0",
            "configuration": {
                "configPath": "",
                "output": "json",
                "file": "",
                "output-template-file": "",
                "quiet": false,
                "check-for-app-update": true,
                "only-fixed": false,
                "search": {
                    "scope": "Squashed",
                    "unindexed-archives": false,
                    "indexed-archives": true
                },
                "ignore": null,
                "exclude": [],
                "db": {
                    "cache-dir": "/home/jim/.cache/grype/db",
                    "update-url": "https://toolbox-data.anchore.io/grype/databases/listing.json",
                    "ca-cert": "",
                    "auto-update": true,
                    "validate-by-hash-on-start": false
                },
                "dev": {
                    "profile-cpu": false,
                    "profile-mem": false
                },
                "fail-on-severity": "",
                "registry": {
                    "insecure-skip-tls-verify": false,
                    "insecure-use-http": false,
                    "auth": []
                },
                "log": {
                    "structured": false,
                    "level": "",
                    "file": ""
                }
            },
            "db": {
                "built": "2022-05-15T08:15:19Z",
                "schemaVersion": 3,
                "location": "/home/jim/.cache/grype/db/3",
                "checksum": "sha256:4e6836ac8db4fbe1488c6a81f37cdc044e3d22041573c7a552aa0e053efa5c29",
                "error": null
            }
        }
    }
}
`

func Test_Conditions(t *testing.T) {
	conditions := []v1.AnyAllConditions{
		{
			AnyConditions: []v1.Condition{
				{
					RawKey:   &apiextv1.JSON{Raw: []byte("\"{{ matches[].vulnerability[].cvss[?metrics.impactScore > '8.0'][] | length(@) }}\"")},
					Operator: "Equals",
					RawValue: &apiextv1.JSON{Raw: []byte("\"0\"")},
				},
				{
					RawKey:   &apiextv1.JSON{Raw: []byte("\"{{ source.target.userInput }}\"")},
					Operator: "Equals",
					RawValue: &apiextv1.JSON{Raw: []byte("\"ghcr.io/tap8stry/git-init:v0.21.0@sha256:322e3502c1e6fba5f1869efb55cfd998a3679e073840d33eb0e3c482b5d5609b\"")},
				},
			},
		},
	}

	ctx := context.NewContext(jmespath.New(config.NewDefaultConfiguration(false)))
	img := api.ImageInfo{Pointer: "/spec/containers/0/image"}
	img.ImageInfo = image.ImageInfo{
		Registry: "docker.io",
		Name:     "nginx",
		Path:     "test/nginx",
		Tag:      "latest",
	}

	var dataMap map[string]interface{}
	err := json.Unmarshal([]byte(scanPredicate), &dataMap)
	assert.NilError(t, err)

	pass, _, err := internal.EvaluateConditions(conditions, ctx, dataMap, logr.Discard())
	assert.NilError(t, err)
	assert.Equal(t, pass, true)
}
