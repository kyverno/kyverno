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

package logger

import "github.com/minio/minio/cmd/config"

// Help template for logger http and audit
var (
	Help = config.HelpKVS{
		config.HelpKV{
			Key:         Endpoint,
<<<<<<< HEAD
			Description: `HTTP logger endpoint e.g. "http://localhost:8080/minio/logs/server"`,
=======
			Description: `HTTP logger endpoint eg: "http://localhost:8080/minio/logs/server"`,
>>>>>>> 524_bug
			Type:        "url",
		},
		config.HelpKV{
			Key:         AuthToken,
<<<<<<< HEAD
			Description: "authorization token for logger endpoint",
=======
			Description: "Authorization token for logger endpoint",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the HTTP logger setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}

	HelpAudit = config.HelpKVS{
		config.HelpKV{
			Key:         Endpoint,
<<<<<<< HEAD
			Description: `HTTP Audit logger endpoint e.g. "http://localhost:8080/minio/logs/audit"`,
=======
			Description: `HTTP Audit logger endpoint eg: "http://localhost:8080/minio/logs/audit"`,
>>>>>>> 524_bug
			Type:        "url",
		},
		config.HelpKV{
			Key:         AuthToken,
<<<<<<< HEAD
			Description: "authorization token for audit logger endpoint",
=======
			Description: "Authorization token for logger endpoint",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the HTTP Audit logger setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}
)
