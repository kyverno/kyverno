/*
 * MinIO Cloud Storage, (C) 2018 MinIO, Inc.
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

package log

// Args - defines the arguments for the API.
type Args struct {
	Bucket   string            `json:"bucket,omitempty"`
	Object   string            `json:"object,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Trace - defines the trace.
type Trace struct {
	Message   string            `json:"message,omitempty"`
	Source    []string          `json:"source,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
}

// API - defines the api type and its args.
type API struct {
	Name string `json:"name,omitempty"`
	Args *Args  `json:"args,omitempty"`
}

// Entry - defines fields and values of each log entry.
type Entry struct {
	DeploymentID string `json:"deploymentid,omitempty"`
	Level        string `json:"level"`
	Time         string `json:"time"`
	API          *API   `json:"api,omitempty"`
	RemoteHost   string `json:"remotehost,omitempty"`
	RequestID    string `json:"requestID,omitempty"`
	UserAgent    string `json:"userAgent,omitempty"`
	Message      string `json:"message,omitempty"`
	Trace        *Trace `json:"error,omitempty"`
}
