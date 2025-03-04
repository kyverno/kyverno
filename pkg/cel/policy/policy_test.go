package policy

import (
	"context"
	"encoding/json"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"gotest.tools/assert"
)

func Test_evaluateJson(t *testing.T) {
	pRaw := []byte(`
	{
    "apiVersion": "policies.kyverno.io/v1alpha1",
    "kind": "ValidatingPolicy",
    "metadata": {
        "name": "check-dockerfile"
    },
    "spec": {
        "evaluation": {
            "mode": "JSON"
        },
        "validations": [
            {
                "message": "HTTP calls are not allowed",
                "expression": "!object.Stages.exists(s, \n  s.Commands.exists(c, \n    c.Args.exists(a, \n      a.Value.contains('http://') || a.Value.contains('https://')\n    )\n  )\n)"
            },
            {
                "message": "curl is not allowed",
                "expression": "!object.Stages.exists(s, \n  s.Commands.exists(c, \n    c.CmdLine.contains('curl')\n  )\n)"
            },
            {
                "message": "wget is not allowed",
                "expression": "!object.Stages.exists(s, \n  s.Commands.exists(c, \n    c.CmdLine.contains('wget')\n  )\n)"
            }
        ]
    }
}`)

	jsonRaw := []byte(`
{
  "MetaArgs": [
    {
      "Key": "BUILD_PLATFORM",
      "DefaultValue": "\"linux/amd64\"",
      "ProvidedValue": null,
      "Value": "\"linux/amd64\""
    },
    {
      "Key": "BUILDER_IMAGE",
      "DefaultValue": "\"golang:1.20.6-alpine3.18\"",
      "ProvidedValue": null,
      "Value": "\"golang:1.20.6-alpine3.18\""
    }
  ],
  "Stages": [
    {
      "Name": "builder",
      "BaseName": "\"golang:1.20.6-alpine3.18\"",
      "Platform": "$BUILD_PLATFORM",
      "Comment": "",
      "SourceCode": "FROM --platform=$BUILD_PLATFORM $BUILDER_IMAGE as builder",
      "Location": [
        {
          "Start": {
            "Line": 4,
            "Character": 0
          },
          "End": {
            "Line": 4,
            "Character": 0
          }
        }
      ],
      "As": "builder",
      "From": {
        "Image": "\"golang:1.20.6-alpine3.18\""
      },
      "Commands": [
        {
          "Name": "EXPOSE",
          "Ports": [
            "22/tcp/asd"
          ]
        },
        {
          "Name": "WORKDIR",
          "Path": "/"
        },
        {
          "Chmod": "",
          "Chown": "",
          "DestPath": "./",
          "From": "",
          "Link": false,
          "Name": "COPY",
          "SourceContents": null,
          "SourcePaths": [
            "."
          ]
        },
        {
          "Args": [
            {
              "Comment": "",
              "Key": "SIGNER_BINARY_LINK",
              "Value": "\"https://d2hvyiie56hcat.cloudfront.net/linux/amd64/plugin/latest/notation-aws-signer-plugin.zip\""
            }
          ],
          "Name": "ARG"
        },
        {
          "Args": [
            {
              "Comment": "",
              "Key": "SIGNER_BINARY_FILE",
              "Value": "\"notation-aws-signer-plugin.zip\""
            }
          ],
          "Name": "ARG"
        },
        {
          "CmdLine": [
            "wget -O ${SIGNER_BINARY_FILE} ${SIGNER_BINARY_LINK}"
          ],
          "Files": null,
          "FlagsUsed": [],
          "Name": "RUN",
          "PrependShell": true
        },
        {
          "CmdLine": [
            "apk update &&     apk add unzip &&     unzip -o ${SIGNER_BINARY_FILE}"
          ],
          "Files": null,
          "FlagsUsed": [],
          "Name": "RUN",
          "PrependShell": true
        },
        {
          "CmdLine": [
            "GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags=\"-w -s\" -o kyverno-notation-aws ."
          ],
          "Files": null,
          "FlagsUsed": [],
          "Name": "RUN",
          "PrependShell": true
        }
      ]
    },
    {
      "Name": "",
      "BaseName": "gcr.io/distroless/static:nonroot",
      "Platform": "",
      "Comment": "",
      "SourceCode": "FROM gcr.io/distroless/static:nonroot",
      "Location": [
        {
          "Start": {
            "Line": 20,
            "Character": 0
          },
          "End": {
            "Line": 20,
            "Character": 0
          }
        }
      ],
      "From": {
        "Image": "gcr.io/distroless/static:nonroot"
      },
      "Commands": [
        {
          "Name": "WORKDIR",
          "Path": "/"
        },
        {
          "Env": [
            {
              "Key": "PLUGINS_DIR",
              "Value": "/plugins"
            }
          ],
          "Name": "ENV"
        },
        {
          "Chmod": "",
          "Chown": "",
          "DestPath": "plugins/com.amazonaws.signer.notation.plugin/notation-com.amazonaws.signer.notation.plugin",
          "From": "builder",
          "Link": false,
          "Name": "COPY",
          "SourceContents": null,
          "SourcePaths": [
            "notation-com.amazonaws.signer.notation.plugin"
          ]
        },
        {
          "Chmod": "",
          "Chown": "",
          "DestPath": "kyverno-notation-aws",
          "From": "builder",
          "Link": false,
          "Name": "COPY",
          "SourceContents": null,
          "SourcePaths": [
            "kyverno-notation-aws"
          ]
        },
        {
          "CmdLine": [
            "/kyverno-notation-aws"
          ],
          "Files": null,
          "Name": "ENTRYPOINT",
          "PrependShell": false
        }
      ]
    }
  ]
}
`)

	var policy policiesv1alpha1.ValidatingPolicy
	err := json.Unmarshal(pRaw, &policy)
	assert.NilError(t, err)

	ctx := context.TODO()
	compiler := NewCompiler()
	compiledVpol, errList := compiler.CompileValidating(&policy, nil)
	if errList != nil {
		assert.NilError(t, errList.ToAggregate())
	}

	var data map[string]any
	err = json.Unmarshal(jsonRaw, &data)
	assert.NilError(t, err)

	result, err := compiledVpol.Evaluate(ctx, data, nil, nil, nil, nil, -1)
	if err != nil {
		assert.NilError(t, err)
	}

	t.Log(result)
}
