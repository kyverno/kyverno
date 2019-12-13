/*
 * MinIO Cloud Storage, (C) 2019 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package crypto

import "github.com/minio/minio/cmd/config"

// Help template for KMS vault
var (
	Help = config.HelpKVS{
		config.HelpKV{
			Key:         KMSVaultEndpoint,
<<<<<<< HEAD
			Description: `HashiCorp Vault API endpoint e.g. "http://vault-endpoint-ip:8200"`,
=======
			Description: `Points to Vault API endpoint eg: "http://vault-endpoint-ip:8200"`,
>>>>>>> 524_bug
			Type:        "url",
		},
		config.HelpKV{
			Key:         KMSVaultKeyName,
<<<<<<< HEAD
			Description: `transit key name used in vault policy, must be unique name e.g. "my-minio-key"`,
=======
			Description: `Transit key name used in vault policy, must be unique name eg: "my-minio-key"`,
>>>>>>> 524_bug
			Type:        "string",
		},
		config.HelpKV{
			Key:         KMSVaultAuthType,
<<<<<<< HEAD
			Description: `authentication type to Vault API endpoint e.g. "approle"`,
=======
			Description: `Authentication type to Vault API endpoint eg: "approle"`,
>>>>>>> 524_bug
			Type:        "string",
		},
		config.HelpKV{
			Key:         KMSVaultAppRoleID,
<<<<<<< HEAD
			Description: `unique role ID created for AppRole`,
=======
			Description: `Unique role ID created for AppRole`,
>>>>>>> 524_bug
			Type:        "string",
		},
		config.HelpKV{
			Key:         KMSVaultAppRoleSecret,
<<<<<<< HEAD
			Description: `unique secret ID created for AppRole`,
=======
			Description: `Unique secret ID created for AppRole`,
>>>>>>> 524_bug
			Type:        "string",
		},
		config.HelpKV{
			Key:         KMSVaultNamespace,
<<<<<<< HEAD
			Description: `only needed if AppRole engine is scoped to Vault Namespace e.g. "ns1"`,
=======
			Description: `Only needed if AppRole engine is scoped to Vault Namespace eg: "ns1"`,
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         KMSVaultKeyVersion,
			Description: `KMS Vault key version`,
			Optional:    true,
			Type:        "number",
		},
		config.HelpKV{
			Key:         KMSVaultCAPath,
<<<<<<< HEAD
			Description: `path to PEM-encoded CA cert files to use mTLS authentication (optional) e.g. "/home/user/custom-certs"`,
=======
			Description: `Path to PEM-encoded CA cert files to use mTLS authentication (optional) eg: "/home/user/custom-certs"`,
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the KMS Vault setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}
)
