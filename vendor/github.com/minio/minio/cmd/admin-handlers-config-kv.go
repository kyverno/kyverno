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

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/minio/minio/cmd/config"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/madmin"
)

func validateAdminReqConfigKV(ctx context.Context, w http.ResponseWriter, r *http.Request) ObjectLayer {
	// Get current object layer instance.
	objectAPI := newObjectLayerWithoutSafeModeFn()
	if objectAPI == nil {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrServerNotInitialized), r.URL)
		return nil
	}

	// Validate request signature.
	adminAPIErr := checkAdminRequestAuthType(ctx, r, "")
	if adminAPIErr != ErrNone {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(adminAPIErr), r.URL)
		return nil
	}

	return objectAPI
}

// DelConfigKVHandler - DELETE /minio/admin/v2/del-config-kv
func (a adminAPIHandlers) DelConfigKVHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "DelConfigKVHandler")

	objectAPI := validateAdminReqConfigKV(ctx, w, r)
	if objectAPI == nil {
		return
	}

	// Deny if WORM is enabled
	if globalWORMEnabled {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrMethodNotAllowed), r.URL)
		return
	}

	if r.ContentLength > maxEConfigJSONSize || r.ContentLength == -1 {
		// More than maxConfigSize bytes were available
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrAdminConfigTooLarge), r.URL)
		return
	}

	password := globalActiveCred.SecretKey
	kvBytes, err := madmin.DecryptData(password, io.LimitReader(r.Body, r.ContentLength))
	if err != nil {
		logger.LogIf(ctx, err, logger.Application)
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrAdminConfigBadJSON), r.URL)
		return
	}
	cfg, err := readServerConfig(ctx, objectAPI)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	oldCfg := cfg.Clone()
	scanner := bufio.NewScanner(bytes.NewReader(kvBytes))
	for scanner.Scan() {
		// Skip any empty lines
		if scanner.Text() == "" {
			continue
		}
		if err = cfg.DelKVS(scanner.Text()); err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}
	}
	if err = scanner.Err(); err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	if err = saveServerConfig(ctx, objectAPI, cfg, oldCfg); err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}
}

// SetConfigKVHandler - PUT /minio/admin/v2/set-config-kv
func (a adminAPIHandlers) SetConfigKVHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "SetConfigKVHandler")

	objectAPI := validateAdminReqConfigKV(ctx, w, r)
	if objectAPI == nil {
		return
	}

	// Deny if WORM is enabled
	if globalWORMEnabled {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrMethodNotAllowed), r.URL)
		return
	}

	if r.ContentLength > maxEConfigJSONSize || r.ContentLength == -1 {
		// More than maxConfigSize bytes were available
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrAdminConfigTooLarge), r.URL)
		return
	}

	password := globalActiveCred.SecretKey
	kvBytes, err := madmin.DecryptData(password, io.LimitReader(r.Body, r.ContentLength))
	if err != nil {
		logger.LogIf(ctx, err, logger.Application)
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrAdminConfigBadJSON), r.URL)
		return
	}
	cfg, err := readServerConfig(ctx, objectAPI)
	if err != nil {
		// Config not found for some reason, allow things to continue
		// by initializing a new fresh config in safe mode.
		if err == errConfigNotFound && globalSafeMode {
			cfg = newServerConfig()
			err = nil
		} else {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}
	}

	defaultKVS := configDefaultKVS()
	oldCfg := cfg.Clone()
	scanner := bufio.NewScanner(bytes.NewReader(kvBytes))
	for scanner.Scan() {
		// Skip any empty lines
		if scanner.Text() == "" {
			continue
		}
		if err = cfg.SetKVS(scanner.Text(), defaultKVS); err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}
	}
	if err = scanner.Err(); err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	if err = validateConfig(cfg); err != nil {
		writeCustomErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrAdminConfigBadJSON), err.Error(), r.URL)
		return
	}

	// Update the actual server config on disk.
	if err = saveServerConfig(ctx, objectAPI, cfg, oldCfg); err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	// Write to the config input KV to history.
	if err = saveServerConfigHistory(ctx, objectAPI, kvBytes); err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	// Make sure to write backend is encrypted
	if globalConfigEncrypted {
		saveConfig(context.Background(), objectAPI, backendEncryptedFile, backendEncryptedMigrationComplete)
	}
}

// GetConfigKVHandler - GET /minio/admin/v2/get-config-kv?key={key}
func (a adminAPIHandlers) GetConfigKVHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "GetConfigKVHandler")

	objectAPI := validateAdminReqConfigKV(ctx, w, r)
	if objectAPI == nil {
		return
	}

	cfg := globalServerConfig
	if globalSafeMode {
		var err error
		cfg, err = getValidConfig(objectAPI)
		if err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}
	}

	vars := mux.Vars(r)
	var buf = &bytes.Buffer{}
	key := vars["key"]
	if key != "" {
		kvs, err := cfg.GetKVS(key)
		if err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}
		for k, kv := range kvs {
			buf.WriteString(k)
			buf.WriteString(config.KvSpaceSeparator)
			buf.WriteString(kv.String())
			if len(kvs) > 1 {
				buf.WriteString(config.KvNewline)
			}
		}
	} else {
		buf.WriteString(cfg.String())
	}

	password := globalActiveCred.SecretKey
	econfigData, err := madmin.EncryptData(password, buf.Bytes())
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	writeSuccessResponseJSON(w, econfigData)
}

func (a adminAPIHandlers) ClearConfigHistoryKVHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "ClearConfigHistoryKVHandler")

	objectAPI := validateAdminReqConfigKV(ctx, w, r)
	if objectAPI == nil {
		return
	}

	vars := mux.Vars(r)
	restoreID := vars["restoreId"]
	if restoreID == "" {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrInvalidRequest), r.URL)
		return
	}
	if restoreID == "all" {
		chEntries, err := listServerConfigHistory(ctx, objectAPI, false, -1)
		if err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}
		for _, chEntry := range chEntries {
			if err = delServerConfigHistory(ctx, objectAPI, chEntry.RestoreID); err != nil {
				writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
				return
			}
		}
	} else {
		if err := delServerConfigHistory(ctx, objectAPI, restoreID); err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}
	}
}

// RestoreConfigHistoryKVHandler - restores a config with KV settings for the given KV id.
func (a adminAPIHandlers) RestoreConfigHistoryKVHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "RestoreConfigHistoryKVHandler")

	objectAPI := validateAdminReqConfigKV(ctx, w, r)
	if objectAPI == nil {
		return
	}

	vars := mux.Vars(r)
	restoreID := vars["restoreId"]
	if restoreID == "" {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrInvalidRequest), r.URL)
		return
	}

	kvBytes, err := readServerConfigHistory(ctx, objectAPI, restoreID)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	cfg, err := readServerConfig(ctx, objectAPI)
	if err != nil {
		// Config not found for some reason, allow things to continue
		// by initializing a new fresh config in safe mode.
		if err == errConfigNotFound && globalSafeMode {
			cfg = newServerConfig()
			err = nil
		} else {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}
	}

	defaultKVS := configDefaultKVS()
	oldCfg := cfg.Clone()
	scanner := bufio.NewScanner(bytes.NewReader(kvBytes))
	for scanner.Scan() {
		// Skip any empty lines
		if scanner.Text() == "" {
			continue
		}
		if err = cfg.SetKVS(scanner.Text(), defaultKVS); err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}
	}
	if err = scanner.Err(); err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	if err = validateConfig(cfg); err != nil {
		writeCustomErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrAdminConfigBadJSON), err.Error(), r.URL)
		return
	}

	if err = saveServerConfig(ctx, objectAPI, cfg, oldCfg); err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	delServerConfigHistory(ctx, objectAPI, restoreID)
}

// ListConfigHistoryKVHandler - lists all the KV ids.
func (a adminAPIHandlers) ListConfigHistoryKVHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "ListConfigHistoryKVHandler")

	objectAPI := validateAdminReqConfigKV(ctx, w, r)
	if objectAPI == nil {
		return
	}

	vars := mux.Vars(r)
	count, err := strconv.Atoi(vars["count"])
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	chEntries, err := listServerConfigHistory(ctx, objectAPI, true, count)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	data, err := json.Marshal(chEntries)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	password := globalActiveCred.SecretKey
	econfigData, err := madmin.EncryptData(password, data)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	writeSuccessResponseJSON(w, econfigData)
}

// HelpConfigKVHandler - GET /minio/admin/v2/help-config-kv?subSys={subSys}&key={key}
func (a adminAPIHandlers) HelpConfigKVHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "HelpConfigKVHandler")

	objectAPI := validateAdminReqConfigKV(ctx, w, r)
	if objectAPI == nil {
		return
	}

	vars := mux.Vars(r)

	subSys := vars["subSys"]
	key := vars["key"]

	_, envOnly := r.URL.Query()["env"]

	rd, err := GetHelp(subSys, key, envOnly)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	json.NewEncoder(w).Encode(rd)
	w.(http.Flusher).Flush()
}

// SetConfigHandler - PUT /minio/admin/v2/config
func (a adminAPIHandlers) SetConfigHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "SetConfigHandler")

	objectAPI := validateAdminReqConfigKV(ctx, w, r)
	if objectAPI == nil {
		return
	}

	// Deny if WORM is enabled
	if globalWORMEnabled {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrMethodNotAllowed), r.URL)
		return
	}

	if r.ContentLength > maxEConfigJSONSize || r.ContentLength == -1 {
		// More than maxConfigSize bytes were available
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrAdminConfigTooLarge), r.URL)
		return
	}

	password := globalActiveCred.SecretKey
	configBytes, err := madmin.DecryptData(password, io.LimitReader(r.Body, r.ContentLength))
	if err != nil {
		logger.LogIf(ctx, err, logger.Application)
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrAdminConfigBadJSON), r.URL)
		return
	}

	var cfg config.Config
	if err = json.Unmarshal(configBytes, &cfg); err != nil {
		logger.LogIf(ctx, err)
		writeCustomErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrAdminConfigBadJSON), err.Error(), r.URL)
		return
	}

	if err = validateConfig(cfg); err != nil {
		writeCustomErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrAdminConfigBadJSON), err.Error(), r.URL)
		return
	}

	if err = saveServerConfig(ctx, objectAPI, cfg, nil); err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	// Make sure to write backend is encrypted
	if globalConfigEncrypted {
		saveConfig(context.Background(), objectAPI, backendEncryptedFile, backendEncryptedMigrationComplete)
	}

	// Reply to the client before restarting minio server.
	writeSuccessResponseHeadersOnly(w)
}

// GetConfigHandler - GET /minio/admin/v2/config
// Get config.json of this minio setup.
func (a adminAPIHandlers) GetConfigHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "GetConfigHandler")

	objectAPI := validateAdminReqConfigKV(ctx, w, r)
	if objectAPI == nil {
		return
	}

	config, err := readServerConfig(ctx, objectAPI)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	configData, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	password := globalActiveCred.SecretKey
	econfigData, err := madmin.EncryptData(password, configData)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	writeSuccessResponseJSON(w, econfigData)
}
