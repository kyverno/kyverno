/*
 * MinIO Cloud Storage, (C) 2018, 2019 MinIO, Inc.
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

package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	gohttp "net/http"
	"strings"

	xhttp "github.com/minio/minio/cmd/http"
)

// Target implements logger.Target and sends the json
// format of a log entry to the configured http endpoint.
// An internal buffer of logs is maintained but when the
// buffer is full, new logs are just ignored and an error
// is returned to the caller.
type Target struct {
	// Channel of log entries
	logCh chan interface{}

	// HTTP(s) endpoint
	endpoint string
	// User-Agent to be set on each log request sent to the `endpoint`
	userAgent string
	logKind   string
	client    gohttp.Client
}

func (h *Target) startHTTPLogger() {
	// Create a routine which sends json logs received
	// from an internal channel.
	go func() {
		for entry := range h.logCh {
			logJSON, err := json.Marshal(&entry)
			if err != nil {
				continue
			}

			req, err := gohttp.NewRequest(http.MethodPost, h.endpoint, bytes.NewBuffer(logJSON))
			if err != nil {
				continue
			}
			req.Header.Set(xhttp.ContentType, "application/json")

			// Set user-agent to indicate MinIO release
			// version to the configured log endpoint
			req.Header.Set("User-Agent", h.userAgent)

			resp, err := h.client.Do(req)
			if err != nil {
				continue
			}

			// Drain any response.
			xhttp.DrainBody(resp.Body)
		}
	}()
}

// New initializes a new logger target which
// sends log over http to the specified endpoint
func New(endpoint, userAgent, logKind string, transport *gohttp.Transport) *Target {
	h := Target{
		endpoint:  endpoint,
		userAgent: userAgent,
		logKind:   strings.ToUpper(logKind),
		client: gohttp.Client{
			Transport: transport,
		},
		logCh: make(chan interface{}, 10000),
	}

	h.startHTTPLogger()
	return &h
}

// Send log message 'e' to http target.
func (h *Target) Send(entry interface{}, errKind string) error {
	if h.logKind != errKind && h.logKind != "ALL" {
		return nil
	}
	select {
	case h.logCh <- entry:
	default:
		// log channel is full, do not wait and return
		// an error immediately to the caller
		return errors.New("log buffer full")
	}

	return nil
}
