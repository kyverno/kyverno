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

package etcd

import "github.com/minio/minio/cmd/config"

// etcd config documented in default config
var (
	Help = config.HelpKVS{
		config.HelpKV{
			Key:         Endpoints,
<<<<<<< HEAD
			Description: `comma separated list of etcd endpoints e.g. "http://localhost:2379"`,
=======
			Description: `Comma separated list of etcd endpoints eg: "http://localhost:2379"`,
>>>>>>> 524_bug
			Type:        "csv",
		},
		config.HelpKV{
			Key:         PathPrefix,
<<<<<<< HEAD
			Description: `default etcd path prefix to populate all IAM assets eg: "customer/"`,
			Optional:    true,
=======
			Description: `Default etcd path prefix to populate all IAM assets eg: "customer/"`,
>>>>>>> 524_bug
			Type:        "path",
		},
		config.HelpKV{
			Key:         CoreDNSPath,
<<<<<<< HEAD
			Description: `default etcd path location to populate bucket DNS srv records eg: "/skydns"`,
=======
			Description: `Default etcd path location to populate bucket DNS srv records eg: "/skydns"`,
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         ClientCert,
<<<<<<< HEAD
			Description: `client cert for mTLS authentication`,
=======
			Description: `Etcd client cert for mTLS authentication`,
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         ClientCertKey,
<<<<<<< HEAD
			Description: `client cert key for mTLS authentication`,
=======
			Description: `Etcd client cert key for mTLS authentication`,
>>>>>>> 524_bug
			Optional:    true,
			Type:        "path",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the etcd settings",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}
)
