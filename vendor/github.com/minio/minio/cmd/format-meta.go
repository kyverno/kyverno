/*
 * MinIO Cloud Storage, (C) 2017 MinIO, Inc.
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

package cmd

// Format related consts
const (
	// Format config file carries backend format specific details.
	formatConfigFile = "format.json"
)

const (
	// Version of the formatMetaV1
	formatMetaVersionV1 = "1"
)

// format.json currently has the format:
// {
//   "version": "1",
//   "format": "XXXXX",
//   "XXXXX": {
//
//   }
// }
// Here "XXXXX" depends on the backend, currently we have "fs" and "xl" implementations.
// formatMetaV1 should be inherited by backend format structs. Please look at format-fs.go
// and format-xl.go for details.

// Ideally we will never have a situation where we will have to change the
// fields of this struct and deal with related migration.
type formatMetaV1 struct {
	// Version of the format config.
	Version string `json:"version"`
	// Format indicates the backend format type, supports two values 'xl' and 'fs'.
	Format string `json:"format"`
	// ID is the identifier for the minio deployment
	ID string `json:"id"`
}
