/*
 * MinIO Cloud Storage, (C) 2016-2019 MinIO, Inc.
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
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/gorilla/mux"

	"github.com/minio/minio/cmd/config"
	"github.com/minio/minio/cmd/config/notify"
	"github.com/minio/minio/cmd/crypto"
	xhttp "github.com/minio/minio/cmd/http"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/cpu"
	"github.com/minio/minio/pkg/event/target"
	"github.com/minio/minio/pkg/handlers"
	iampolicy "github.com/minio/minio/pkg/iam/policy"
	"github.com/minio/minio/pkg/madmin"
	"github.com/minio/minio/pkg/mem"
	xnet "github.com/minio/minio/pkg/net"
	trace "github.com/minio/minio/pkg/trace"
)

const (
	maxEConfigJSONSize = 262272
	defaultNetPerfSize = 100 * humanize.MiByte
)

// Type-safe query params.
type mgmtQueryKey string

// Only valid query params for mgmt admin APIs.
const (
	mgmtBucket      mgmtQueryKey = "bucket"
	mgmtPrefix                   = "prefix"
	mgmtClientToken              = "clientToken"
	mgmtForceStart               = "forceStart"
	mgmtForceStop                = "forceStop"
)

func updateServer(updateURL, sha256Hex string, latestReleaseTime time.Time) (us madmin.ServerUpdateStatus, err error) {
	minioMode := getMinioMode()
	// No inputs provided we should try to update using the default URL.
	if updateURL == "" && sha256Hex == "" && latestReleaseTime.IsZero() {
		var updateMsg string
		updateMsg, sha256Hex, _, latestReleaseTime, err = getUpdateInfo(updateTimeout, minioMode)
		if err != nil {
			return us, err
		}
		if updateMsg == "" {
			us.CurrentVersion = Version
			us.UpdatedVersion = Version
			return us, nil
		}
		if runtime.GOOS == "windows" {
			updateURL = minioReleaseURL + "minio.exe"
		} else {
			updateURL = minioReleaseURL + "minio"
		}
	}
	if err = doUpdate(updateURL, sha256Hex, minioMode); err != nil {
		return us, err
	}
	us.CurrentVersion = Version
	us.UpdatedVersion = latestReleaseTime.Format(minioReleaseTagTimeLayout)
	return us, nil
}

// ServerUpdateHandler - POST /minio/admin/v2/update?updateURL={updateURL}
// ----------
// updates all minio servers and restarts them gracefully.
func (a adminAPIHandlers) ServerUpdateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "ServerUpdate")

	objectAPI, _ := validateAdminReq(ctx, w, r, iampolicy.ServerUpdateAdminAction)
	if objectAPI == nil {
		return
	}

	if globalInplaceUpdateDisabled {
		// if MINIO_UPDATE=off - inplace update is disabled, mostly
		// in containers.
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrMethodNotAllowed), r.URL)
		return
	}

	vars := mux.Vars(r)
	updateURL := vars[peerRESTUpdateURL]
	mode := getMinioMode()
	var sha256Hex string
	var latestReleaseTime time.Time
	if updateURL != "" {
		u, err := url.Parse(updateURL)
		if err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}

		content, err := downloadReleaseURL(updateURL, updateTimeout, mode)
		if err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}

		sha256Hex, latestReleaseTime, err = parseReleaseData(content)
		if err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}

		if runtime.GOOS == "windows" {
			u.Path = path.Dir(u.Path) + "minio.exe"
		} else {
			u.Path = path.Dir(u.Path) + "minio"
		}

		updateURL = u.String()
	}

	for _, nerr := range globalNotificationSys.ServerUpdate(updateURL, sha256Hex, latestReleaseTime) {
		if nerr.Err != nil {
			logger.GetReqInfo(ctx).SetTags("peerAddress", nerr.Host.String())
			logger.LogIf(ctx, nerr.Err)
		}
	}

	updateStatus, err := updateServer(updateURL, sha256Hex, latestReleaseTime)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	// Marshal API response
	jsonBytes, err := json.Marshal(updateStatus)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	writeSuccessResponseJSON(w, jsonBytes)

	if updateStatus.CurrentVersion != updateStatus.UpdatedVersion {
		// We did upgrade - restart all services.
		globalServiceSignalCh <- serviceRestart
	}
}

// ServiceActionHandler - POST /minio/admin/v2/service?action={action}
// ----------
// restarts/stops minio server gracefully. In a distributed setup,
func (a adminAPIHandlers) ServiceActionHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "ServiceAction")

	vars := mux.Vars(r)
	action := vars["action"]

	objectAPI, _ := validateAdminReq(ctx, w, r, "")
	if objectAPI == nil {
		return
	}

	var serviceSig serviceSignal
	switch madmin.ServiceAction(action) {
	case madmin.ServiceActionRestart:
		serviceSig = serviceRestart
	case madmin.ServiceActionStop:
		serviceSig = serviceStop
	default:
		logger.LogIf(ctx, fmt.Errorf("Unrecognized service action %s requested", action), logger.Application)
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrMalformedPOSTRequest), r.URL)
		return
	}

	// Notify all other MinIO peers signal service.
	for _, nerr := range globalNotificationSys.SignalService(serviceSig) {
		if nerr.Err != nil {
			logger.GetReqInfo(ctx).SetTags("peerAddress", nerr.Host.String())
			logger.LogIf(ctx, nerr.Err)
		}
	}

	// Reply to the client before restarting, stopping MinIO server.
	writeSuccessResponseHeadersOnly(w)

	globalServiceSignalCh <- serviceSig
}

// ServerProperties holds some server information such as, version, region
// uptime, etc..
type ServerProperties struct {
	Uptime       int64    `json:"uptime"`
	Version      string   `json:"version"`
	CommitID     string   `json:"commitID"`
	DeploymentID string   `json:"deploymentID"`
	Region       string   `json:"region"`
	SQSARN       []string `json:"sqsARN"`
}

// ServerConnStats holds transferred bytes from/to the server
type ServerConnStats struct {
	TotalInputBytes  uint64 `json:"transferred"`
	TotalOutputBytes uint64 `json:"received"`
	Throughput       uint64 `json:"throughput,omitempty"`
	S3InputBytes     uint64 `json:"transferredS3"`
	S3OutputBytes    uint64 `json:"receivedS3"`
}

// ServerHTTPAPIStats holds total number of HTTP operations from/to the server,
// including the average duration the call was spent.
type ServerHTTPAPIStats struct {
	APIStats map[string]int `json:"apiStats"`
}

// ServerHTTPStats holds all type of http operations performed to/from the server
// including their average execution time.
type ServerHTTPStats struct {
	CurrentS3Requests ServerHTTPAPIStats `json:"currentS3Requests"`
	TotalS3Requests   ServerHTTPAPIStats `json:"totalS3Requests"`
	TotalS3Errors     ServerHTTPAPIStats `json:"totalS3Errors"`
}

// ServerInfoData holds storage, connections and other
// information of a given server.
type ServerInfoData struct {
	ConnStats  ServerConnStats  `json:"network"`
	HTTPStats  ServerHTTPStats  `json:"http"`
	Properties ServerProperties `json:"server"`
}

// ServerInfo holds server information result of one node
type ServerInfo struct {
	Error string          `json:"error"`
	Addr  string          `json:"addr"`
	Data  *ServerInfoData `json:"data"`
}

// StorageInfoHandler - GET /minio/admin/v2/storageinfo
// ----------
// Get server information
func (a adminAPIHandlers) StorageInfoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "StorageInfo")
	objectAPI, _ := validateAdminReq(ctx, w, r, iampolicy.ListServerInfoAdminAction)
	if objectAPI == nil {
		return
	}

	storageInfo := objectAPI.StorageInfo(ctx)

	// Marshal API response
	jsonBytes, err := json.Marshal(storageInfo)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	// Reply with storage information (across nodes in a
	// distributed setup) as json.
	writeSuccessResponseJSON(w, jsonBytes)

}

// DataUsageInfoHandler - GET /minio/admin/v2/datausage
// ----------
// Get server/cluster data usage info
func (a adminAPIHandlers) DataUsageInfoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "DataUsageInfo")
	objectAPI, _ := validateAdminReq(ctx, w, r, iampolicy.ListServerInfoAdminAction)
	if objectAPI == nil {
		return
	}

	dataUsageInfo, err := loadDataUsageFromBackend(ctx, objectAPI)
	if err != nil {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrInternalError), r.URL)
		return
	}

	dataUsageInfoJSON, err := json.Marshal(dataUsageInfo)
	if err != nil {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrInternalError), r.URL)
		return
	}

	writeSuccessResponseJSON(w, dataUsageInfoJSON)
}

// ServerCPULoadInfo holds informantion about cpu utilization
// of one minio node. It also reports any errors if encountered
// while trying to reach this server.
type ServerCPULoadInfo struct {
	Addr         string     `json:"addr"`
	Error        string     `json:"error,omitempty"`
	Load         []cpu.Load `json:"load"`
	HistoricLoad []cpu.Load `json:"historicLoad"`
}

// ServerMemUsageInfo holds informantion about memory utilization
// of one minio node. It also reports any errors if encountered
// while trying to reach this server.
type ServerMemUsageInfo struct {
	Addr          string      `json:"addr"`
	Error         string      `json:"error,omitempty"`
	Usage         []mem.Usage `json:"usage"`
	HistoricUsage []mem.Usage `json:"historicUsage"`
}

// ServerNetReadPerfInfo network read performance information.
type ServerNetReadPerfInfo struct {
	Addr           string `json:"addr"`
	ReadThroughput uint64 `json:"readThroughput"`
	Error          string `json:"error,omitempty"`
}

// PerfInfoHandler - GET /minio/admin/v2/performance?perfType={perfType}
// ----------
// Get all performance information based on input type
// Supported types = drive
func (a adminAPIHandlers) PerfInfoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "PerfInfo")

	objectAPI, _ := validateAdminReq(ctx, w, r, iampolicy.ListServerInfoAdminAction)
	if objectAPI == nil {
		return
	}

	vars := mux.Vars(r)
	switch perfType := vars["perfType"]; perfType {
	case "net":
		var size int64 = defaultNetPerfSize
		if sizeStr, found := vars["size"]; found {
			var err error
			if size, err = strconv.ParseInt(sizeStr, 10, 64); err != nil || size < 0 {
				writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrBadRequest), r.URL)
				return
			}
		}

		if !globalIsDistXL {
			writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrMethodNotAllowed), r.URL)
			return
		}

		addr := r.Host
		if globalIsDistXL {
			addr = GetLocalPeer(globalEndpoints)
		}

		infos := map[string][]ServerNetReadPerfInfo{}
		infos[addr] = globalNotificationSys.NetReadPerfInfo(size)
		for peer, info := range globalNotificationSys.CollectNetPerfInfo(size) {
			infos[peer] = info
		}

		// Marshal API response
		jsonBytes, err := json.Marshal(infos)
		if err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}

		// Reply with performance information (across nodes in a
		// distributed setup) as json.
		writeSuccessResponseJSON(w, jsonBytes)

	case "drive":
		// Drive Perf is only implemented for Erasure coded backends
		if !globalIsXL {
			writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrMethodNotAllowed), r.URL)
			return
		}

		var size int64 = madmin.DefaultDrivePerfSize
		if sizeStr, found := vars["size"]; found {
			var err error
			if size, err = strconv.ParseInt(sizeStr, 10, 64); err != nil || size <= 0 {
				writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrBadRequest), r.URL)
				return
			}
		}
		// Get drive performance details from local server's drive(s)
		dp := getLocalDrivesPerf(globalEndpoints, size, r)

		// Notify all other MinIO peers to report drive performance numbers
		dps := globalNotificationSys.DrivePerfInfo(size)
		dps = append(dps, dp)

		// Marshal API response
		jsonBytes, err := json.Marshal(dps)
		if err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}

		// Reply with performance information (across nodes in a
		// distributed setup) as json.
		writeSuccessResponseJSON(w, jsonBytes)
	case "cpu":
		// Get CPU load details from local server's cpu(s)
		cpu := getLocalCPULoad(globalEndpoints, r)
		// Notify all other MinIO peers to report cpu load numbers
		cpus := globalNotificationSys.CPULoadInfo()
		cpus = append(cpus, cpu)

		// Marshal API response
		jsonBytes, err := json.Marshal(cpus)
		if err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}

		// Reply with cpu load information (across nodes in a
		// distributed setup) as json.
		writeSuccessResponseJSON(w, jsonBytes)
	case "mem":
		// Get mem usage details from local server(s)
		m := getLocalMemUsage(globalEndpoints, r)
		// Notify all other MinIO peers to report mem usage numbers
		mems := globalNotificationSys.MemUsageInfo()
		mems = append(mems, m)

		// Marshal API response
		jsonBytes, err := json.Marshal(mems)
		if err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}

		// Reply with mem usage information (across nodes in a
		// distributed setup) as json.
		writeSuccessResponseJSON(w, jsonBytes)
	default:
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrMethodNotAllowed), r.URL)
	}
}

func newLockEntry(l lockRequesterInfo, resource, server string) *madmin.LockEntry {
	entry := &madmin.LockEntry{
		Timestamp:  l.Timestamp,
		Resource:   resource,
		ServerList: []string{server},
		Source:     l.Source,
		ID:         l.UID,
	}
	if l.Writer {
		entry.Type = "Write"
	} else {
		entry.Type = "Read"
	}
	return entry
}

func topLockEntries(peerLocks []*PeerLocks) madmin.LockEntries {
	entryMap := make(map[string]*madmin.LockEntry)
	for _, peerLock := range peerLocks {
		if peerLock == nil {
			continue
		}
		for _, locks := range peerLock.Locks {
			for k, v := range locks {
				for _, lockReqInfo := range v {
					if val, ok := entryMap[lockReqInfo.UID]; ok {
						val.ServerList = append(val.ServerList, peerLock.Addr)
					} else {
						entryMap[lockReqInfo.UID] = newLockEntry(lockReqInfo, k, peerLock.Addr)
					}
				}
			}
		}
	}
	var lockEntries = make(madmin.LockEntries, 0)
	for _, v := range entryMap {
		lockEntries = append(lockEntries, *v)
	}
	sort.Sort(lockEntries)
	const listCount int = 10
	if len(lockEntries) > listCount {
		lockEntries = lockEntries[:listCount]
	}
	return lockEntries
}

// PeerLocks holds server information result of one node
type PeerLocks struct {
	Addr  string
	Locks GetLocksResp
}

// TopLocksHandler Get list of locks in use
func (a adminAPIHandlers) TopLocksHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "TopLocks")

	objectAPI, _ := validateAdminReq(ctx, w, r, iampolicy.ListServerInfoAdminAction)
	if objectAPI == nil {
		return
	}

	peerLocks := globalNotificationSys.GetLocks(ctx)
	// Once we have received all the locks currently used from peers
	// add the local peer locks list as well.
	var getRespLocks GetLocksResp
	for _, llocker := range globalLockServers {
		getRespLocks = append(getRespLocks, llocker.DupLockMap())
	}
	peerLocks = append(peerLocks, &PeerLocks{
		Addr:  getHostName(r),
		Locks: getRespLocks,
	})

	topLocks := topLockEntries(peerLocks)

	// Marshal API response
	jsonBytes, err := json.Marshal(topLocks)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	// Reply with storage information (across nodes in a
	// distributed setup) as json.
	writeSuccessResponseJSON(w, jsonBytes)
}

// StartProfilingResult contains the status of the starting
// profiling action in a given server
type StartProfilingResult struct {
	NodeName string `json:"nodeName"`
	Success  bool   `json:"success"`
	Error    string `json:"error"`
}

// StartProfilingHandler - POST /minio/admin/v2/profiling/start?profilerType={profilerType}
// ----------
// Enable server profiling
func (a adminAPIHandlers) StartProfilingHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "StartProfiling")

	objectAPI, _ := validateAdminReq(ctx, w, r, iampolicy.ListServerInfoAdminAction)
	if objectAPI == nil {
		return
	}

	vars := mux.Vars(r)
	profiler := vars["profilerType"]

	thisAddr, err := xnet.ParseHost(GetLocalPeer(globalEndpoints))
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	// Start profiling on remote servers.
	hostErrs := globalNotificationSys.StartProfiling(profiler)

	// Start profiling locally as well.
	{
		if globalProfiler != nil {
			globalProfiler.Stop()
		}
		prof, err := startProfiler(profiler, "")
		if err != nil {
			hostErrs = append(hostErrs, NotificationPeerErr{
				Host: *thisAddr,
				Err:  err,
			})
		} else {
			globalProfiler = prof
			hostErrs = append(hostErrs, NotificationPeerErr{
				Host: *thisAddr,
			})
		}
	}

	var startProfilingResult []StartProfilingResult

	for _, nerr := range hostErrs {
		result := StartProfilingResult{NodeName: nerr.Host.String()}
		if nerr.Err != nil {
			result.Error = nerr.Err.Error()
		} else {
			result.Success = true
		}
		startProfilingResult = append(startProfilingResult, result)
	}

	// Create JSON result and send it to the client
	startProfilingResultInBytes, err := json.Marshal(startProfilingResult)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	writeSuccessResponseJSON(w, []byte(startProfilingResultInBytes))
}

// dummyFileInfo represents a dummy representation of a profile data file
// present only in memory, it helps to generate the zip stream.
type dummyFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
	sys     interface{}
}

func (f dummyFileInfo) Name() string       { return f.name }
func (f dummyFileInfo) Size() int64        { return f.size }
func (f dummyFileInfo) Mode() os.FileMode  { return f.mode }
func (f dummyFileInfo) ModTime() time.Time { return f.modTime }
func (f dummyFileInfo) IsDir() bool        { return f.isDir }
func (f dummyFileInfo) Sys() interface{}   { return f.sys }

// DownloadProfilingHandler - POST /minio/admin/v2/profiling/download
// ----------
// Download profiling information of all nodes in a zip format
func (a adminAPIHandlers) DownloadProfilingHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "DownloadProfiling")

	objectAPI, _ := validateAdminReq(ctx, w, r, iampolicy.ListServerInfoAdminAction)
	if objectAPI == nil {
		return
	}

	if !globalNotificationSys.DownloadProfilingData(ctx, w) {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrAdminProfilerNotEnabled), r.URL)
		return
	}
}

type healInitParams struct {
	bucket, objPrefix     string
	hs                    madmin.HealOpts
	clientToken           string
	forceStart, forceStop bool
}

// extractHealInitParams - Validates params for heal init API.
func extractHealInitParams(vars map[string]string, qParms url.Values, r io.Reader) (hip healInitParams, err APIErrorCode) {
	hip.bucket = vars[string(mgmtBucket)]
	hip.objPrefix = vars[string(mgmtPrefix)]

	if hip.bucket == "" {
		if hip.objPrefix != "" {
			// Bucket is required if object-prefix is given
			err = ErrHealMissingBucket
			return
		}
	} else if isReservedOrInvalidBucket(hip.bucket, false) {
		err = ErrInvalidBucketName
		return
	}

	// empty prefix is valid.
	if !IsValidObjectPrefix(hip.objPrefix) {
		err = ErrInvalidObjectName
		return
	}

	if len(qParms[string(mgmtClientToken)]) > 0 {
		hip.clientToken = qParms[string(mgmtClientToken)][0]
	}
	if _, ok := qParms[string(mgmtForceStart)]; ok {
		hip.forceStart = true
	}
	if _, ok := qParms[string(mgmtForceStop)]; ok {
		hip.forceStop = true
	}

	// Invalid request conditions:
	//
	//   Cannot have both forceStart and forceStop in the same
	//   request; If clientToken is provided, request can only be
	//   to continue receiving logs, so it cannot be start or
	//   stop;
	if (hip.forceStart && hip.forceStop) ||
		(hip.clientToken != "" && (hip.forceStart || hip.forceStop)) {
		err = ErrInvalidRequest
		return
	}

	// ignore body if clientToken is provided
	if hip.clientToken == "" {
		jerr := json.NewDecoder(r).Decode(&hip.hs)
		if jerr != nil {
			logger.LogIf(context.Background(), jerr, logger.Application)
			err = ErrRequestBodyParse
			return
		}
	}

	err = ErrNone
	return
}

// HealHandler - POST /minio/admin/v2/heal/
// -----------
// Start heal processing and return heal status items.
//
// On a successful heal sequence start, a unique client token is
// returned. Subsequent requests to this endpoint providing the client
// token will receive heal status records from the running heal
// sequence.
//
// If no client token is provided, and a heal sequence is in progress
// an error is returned with information about the running heal
// sequence. However, if the force-start flag is provided, the server
// aborts the running heal sequence and starts a new one.
func (a adminAPIHandlers) HealHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "Heal")

	objectAPI, _ := validateAdminReq(ctx, w, r, iampolicy.HealAdminAction)
	if objectAPI == nil {
		return
	}

	// Check if this setup has an erasure coded backend.
	if !globalIsXL {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrHealNotImplemented), r.URL)
		return
	}

	hip, errCode := extractHealInitParams(mux.Vars(r), r.URL.Query(), r.Body)
	if errCode != ErrNone {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(errCode), r.URL)
		return
	}

	type healResp struct {
		respBytes []byte
		apiErr    APIError
		errBody   string
	}

	// Define a closure to start sending whitespace to client
	// after 10s unless a response item comes in
	keepConnLive := func(w http.ResponseWriter, respCh chan healResp) {
		ticker := time.NewTicker(time.Second * 10)
		defer ticker.Stop()
		started := false
	forLoop:
		for {
			select {
			case <-ticker.C:
				if !started {
					// Start writing response to client
					started = true
					setCommonHeaders(w)
					w.Header().Set(xhttp.ContentType, "text/event-stream")
					// Set 200 OK status
					w.WriteHeader(200)
				}
				// Send whitespace and keep connection open
				w.Write([]byte(" "))
				w.(http.Flusher).Flush()
			case hr := <-respCh:
				switch hr.apiErr {
				case noError:
					if started {
						w.Write(hr.respBytes)
						w.(http.Flusher).Flush()
					} else {
						writeSuccessResponseJSON(w, hr.respBytes)
					}
				default:
					var errorRespJSON []byte
					if hr.errBody == "" {
						errorRespJSON = encodeResponseJSON(getAPIErrorResponse(ctx, hr.apiErr,
							r.URL.Path, w.Header().Get(xhttp.AmzRequestID),
							globalDeploymentID))
					} else {
						errorRespJSON = encodeResponseJSON(APIErrorResponse{
							Code:      hr.apiErr.Code,
							Message:   hr.errBody,
							Resource:  r.URL.Path,
							RequestID: w.Header().Get(xhttp.AmzRequestID),
							HostID:    globalDeploymentID,
						})
					}
					if !started {
						setCommonHeaders(w)
						w.Header().Set(xhttp.ContentType, string(mimeJSON))
						w.WriteHeader(hr.apiErr.HTTPStatusCode)
					}
					w.Write(errorRespJSON)
					w.(http.Flusher).Flush()
				}
				break forLoop
			}
		}
	}

	// find number of disks in the setup
	info := objectAPI.StorageInfo(ctx)
	numDisks := info.Backend.OfflineDisks.Sum() + info.Backend.OnlineDisks.Sum()

	healPath := pathJoin(hip.bucket, hip.objPrefix)
	if hip.clientToken == "" && !hip.forceStart && !hip.forceStop {
		nh, exists := globalAllHealState.getHealSequence(healPath)
		if exists && !nh.hasEnded() && len(nh.currentStatus.Items) > 0 {
			b, err := json.Marshal(madmin.HealStartSuccess{
				ClientToken:   nh.clientToken,
				ClientAddress: nh.clientAddress,
				StartTime:     nh.startTime,
			})
			if err != nil {
				writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
				return
			}
			// Client token not specified but a heal sequence exists on a path,
			// Send the token back to client.
			writeSuccessResponseJSON(w, b)
			return
		}
	}

	if hip.clientToken != "" && !hip.forceStart && !hip.forceStop {
		// Since clientToken is given, fetch heal status from running
		// heal sequence.
		respBytes, errCode := globalAllHealState.PopHealStatusJSON(
			healPath, hip.clientToken)
		if errCode != ErrNone {
			writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(errCode), r.URL)
		} else {
			writeSuccessResponseJSON(w, respBytes)
		}
		return
	}

	respCh := make(chan healResp)
	switch {
	case hip.forceStop:
		go func() {
			respBytes, apiErr := globalAllHealState.stopHealSequence(healPath)
			hr := healResp{respBytes: respBytes, apiErr: apiErr}
			respCh <- hr
		}()
	case hip.clientToken == "":
		nh := newHealSequence(hip.bucket, hip.objPrefix, handlers.GetSourceIP(r), numDisks, hip.hs, hip.forceStart)
		go func() {
			respBytes, apiErr, errMsg := globalAllHealState.LaunchNewHealSequence(nh)
			hr := healResp{respBytes, apiErr, errMsg}
			respCh <- hr
		}()
	}

	// Due to the force-starting functionality, the Launch
	// call above can take a long time - to keep the
	// connection alive, we start sending whitespace
	keepConnLive(w, respCh)
}

func (a adminAPIHandlers) BackgroundHealStatusHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "HealBackgroundStatus")

	objectAPI, _ := validateAdminReq(ctx, w, r, iampolicy.HealAdminAction)
	if objectAPI == nil {
		return
	}

	// Check if this setup has an erasure coded backend.
	if !globalIsXL {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrHealNotImplemented), r.URL)
		return
	}

	var bgHealStates []madmin.BgHealState

	// Get local heal status first
	bgHealStates = append(bgHealStates, getLocalBackgroundHealStatus())

	if globalIsDistXL {
		// Get heal status from other peers
		peersHealStates := globalNotificationSys.BackgroundHealStatus()
		bgHealStates = append(bgHealStates, peersHealStates...)
	}

	// Aggregate healing result
	var aggregatedHealStateResult = madmin.BgHealState{}
	for _, state := range bgHealStates {
		aggregatedHealStateResult.ScannedItemsCount += state.ScannedItemsCount
		if aggregatedHealStateResult.LastHealActivity.Before(state.LastHealActivity) {
			aggregatedHealStateResult.LastHealActivity = state.LastHealActivity
		}

	}

	if err := json.NewEncoder(w).Encode(aggregatedHealStateResult); err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	w.(http.Flusher).Flush()
}

func validateAdminReq(ctx context.Context, w http.ResponseWriter, r *http.Request, action iampolicy.AdminAction) (ObjectLayer, auth.Credentials) {
	var cred auth.Credentials
	var adminAPIErr APIErrorCode
	// Get current object layer instance.
	objectAPI := newObjectLayerWithoutSafeModeFn()
	if objectAPI == nil || globalNotificationSys == nil {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrServerNotInitialized), r.URL)
		return nil, cred
	}

	// Validate request signature.
	cred, adminAPIErr = checkAdminRequestAuthType(ctx, r, action, "")
	if adminAPIErr != ErrNone {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(adminAPIErr), r.URL)
		return nil, cred
	}

	return objectAPI, cred
}

// AdminError - is a generic error for all admin APIs.
type AdminError struct {
	Code       string
	Message    string
	StatusCode int
}

func (ae AdminError) Error() string {
	return ae.Message
}

// Admin API errors
const (
	AdminUpdateUnexpectedFailure = "XMinioAdminUpdateUnexpectedFailure"
	AdminUpdateURLNotReachable   = "XMinioAdminUpdateURLNotReachable"
	AdminUpdateApplyFailure      = "XMinioAdminUpdateApplyFailure"
)

// toAdminAPIErrCode - converts errXLWriteQuorum error to admin API
// specific error.
func toAdminAPIErrCode(ctx context.Context, err error) APIErrorCode {
	switch err {
	case errXLWriteQuorum:
		return ErrAdminConfigNoQuorum
	default:
		return toAPIErrorCode(ctx, err)
	}
}

func toAdminAPIErr(ctx context.Context, err error) APIError {
	if err == nil {
		return noError
	}

	var apiErr APIError
	switch e := err.(type) {
	case config.Error:
		apiErr = APIError{
			Code:           "XMinioConfigError",
			Description:    e.Error(),
			HTTPStatusCode: http.StatusBadRequest,
		}
	case AdminError:
		apiErr = APIError{
			Code:           e.Code,
			Description:    e.Message,
			HTTPStatusCode: e.StatusCode,
		}
	default:
		if err == errConfigNotFound {
			apiErr = APIError{
				Code:           "XMinioConfigError",
				Description:    err.Error(),
				HTTPStatusCode: http.StatusNotFound,
			}
		} else {
			apiErr = errorCodes.ToAPIErr(toAdminAPIErrCode(ctx, err))
		}
	}
	return apiErr
}

// Returns true if the trace.Info should be traced,
// false if certain conditions are not met.
// - input entry is not of the type *trace.Info*
// - errOnly entries are to be traced, not status code 2xx, 3xx.
// - all entries to be traced, if not trace only S3 API requests.
func mustTrace(entry interface{}, trcAll, errOnly bool) bool {
	trcInfo, ok := entry.(trace.Info)
	if !ok {
		return false
	}
	trace := trcAll || !HasPrefix(trcInfo.ReqInfo.Path, minioReservedBucketPath+SlashSeparator)
	if errOnly {
		return trace && trcInfo.RespInfo.StatusCode >= http.StatusBadRequest
	}
	return trace
}

// TraceHandler - POST /minio/admin/v2/trace
// ----------
// The handler sends http trace to the connected HTTP client.
func (a adminAPIHandlers) TraceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "HTTPTrace")
	trcAll := r.URL.Query().Get("all") == "true"
	trcErr := r.URL.Query().Get("err") == "true"

	// Validate request signature.
	_, adminAPIErr := checkAdminRequestAuthType(ctx, r, iampolicy.ListServerInfoAdminAction, "")
	if adminAPIErr != ErrNone {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(adminAPIErr), r.URL)
		return
	}

	w.Header().Set(xhttp.ContentType, "text/event-stream")

	doneCh := make(chan struct{})
	defer close(doneCh)

	// Trace Publisher and peer-trace-client uses nonblocking send and hence does not wait for slow receivers.
	// Use buffered channel to take care of burst sends or slow w.Write()
	traceCh := make(chan interface{}, 4000)

	peers := getRestClients(globalEndpoints)

	globalHTTPTrace.Subscribe(traceCh, doneCh, func(entry interface{}) bool {
		return mustTrace(entry, trcAll, trcErr)
	})

	for _, peer := range peers {
		if peer == nil {
			continue
		}
		peer.Trace(traceCh, doneCh, trcAll, trcErr)
	}

	keepAliveTicker := time.NewTicker(500 * time.Millisecond)
	defer keepAliveTicker.Stop()

	enc := json.NewEncoder(w)
	for {
		select {
		case entry := <-traceCh:
			if err := enc.Encode(entry); err != nil {
				return
			}
			w.(http.Flusher).Flush()
		case <-keepAliveTicker.C:
			if _, err := w.Write([]byte(" ")); err != nil {
				return
			}
			w.(http.Flusher).Flush()
		case <-GlobalServiceDoneCh:
			return
		}
	}
}

// The handler sends console logs to the connected HTTP client.
func (a adminAPIHandlers) ConsoleLogHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "ConsoleLog")

	objectAPI, _ := validateAdminReq(ctx, w, r, iampolicy.ListServerInfoAdminAction)
	if objectAPI == nil {
		return
	}
	node := r.URL.Query().Get("node")
	// limit buffered console entries if client requested it.
	limitStr := r.URL.Query().Get("limit")
	limitLines, err := strconv.Atoi(limitStr)
	if err != nil {
		limitLines = 10
	}

	logKind := r.URL.Query().Get("logType")
	if logKind == "" {
		logKind = string(logger.All)
	}
	logKind = strings.ToUpper(logKind)

	// Avoid reusing tcp connection if read timeout is hit
	// This is needed to make r.Context().Done() work as
	// expected in case of read timeout
	w.Header().Add("Connection", "close")
	w.Header().Set(xhttp.ContentType, "text/event-stream")

	doneCh := make(chan struct{})
	defer close(doneCh)
	logCh := make(chan interface{}, 4000)

	peers := getRestClients(globalEndpoints)

	globalConsoleSys.Subscribe(logCh, doneCh, node, limitLines, logKind, nil)

	for _, peer := range peers {
		if peer == nil {
			continue
		}
		if node == "" || strings.EqualFold(peer.host.Name, node) {
			peer.ConsoleLog(logCh, doneCh)
		}
	}

	enc := json.NewEncoder(w)

	keepAliveTicker := time.NewTicker(500 * time.Millisecond)
	defer keepAliveTicker.Stop()

	for {
		select {
		case entry := <-logCh:
			log := entry.(madmin.LogInfo)
			if log.SendLog(node, logKind) {
				if err := enc.Encode(log); err != nil {
					return
				}
				w.(http.Flusher).Flush()
			}
		case <-keepAliveTicker.C:
			if _, err := w.Write([]byte(" ")); err != nil {
				return
			}
			w.(http.Flusher).Flush()
		case <-GlobalServiceDoneCh:
			return
		}
	}
}

// KMSKeyStatusHandler - GET /minio/admin/v2/kms/key/status?key-id=<master-key-id>
func (a adminAPIHandlers) KMSKeyStatusHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "KMSKeyStatusHandler")

	objectAPI, _ := validateAdminReq(ctx, w, r, iampolicy.ListServerInfoAdminAction)
	if objectAPI == nil {
		return
	}

	if GlobalKMS == nil {
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrKMSNotConfigured), r.URL)
		return
	}

	keyID := r.URL.Query().Get("key-id")
	if keyID == "" {
		keyID = GlobalKMS.KeyID()
	}
	var response = madmin.KMSKeyStatus{
		KeyID: keyID,
	}

	kmsContext := crypto.Context{"MinIO admin API": "KMSKeyStatusHandler"} // Context for a test key operation
	// 1. Generate a new key using the KMS.
	key, sealedKey, err := GlobalKMS.GenerateKey(keyID, kmsContext)
	if err != nil {
		response.EncryptionErr = err.Error()
		resp, err := json.Marshal(response)
		if err != nil {
			writeCustomErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrInternalError), err.Error(), r.URL)
			return
		}
		writeSuccessResponseJSON(w, resp)
		return
	}

	// 2. Check whether we can update / re-wrap the sealed key.
	sealedKey, err = GlobalKMS.UpdateKey(keyID, sealedKey, kmsContext)
	if err != nil {
		response.UpdateErr = err.Error()
		resp, err := json.Marshal(response)
		if err != nil {
			writeCustomErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrInternalError), err.Error(), r.URL)
			return
		}
		writeSuccessResponseJSON(w, resp)
		return
	}

	// 3. Verify that we can indeed decrypt the (encrypted) key
	decryptedKey, err := GlobalKMS.UnsealKey(keyID, sealedKey, kmsContext)
	if err != nil {
		response.DecryptionErr = err.Error()
		resp, err := json.Marshal(response)
		if err != nil {
			writeCustomErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrInternalError), err.Error(), r.URL)
			return
		}
		writeSuccessResponseJSON(w, resp)
		return
	}

	// 4. Compare generated key with decrypted key
	if subtle.ConstantTimeCompare(key[:], decryptedKey[:]) != 1 {
		response.DecryptionErr = "The generated and the decrypted data key do not match"
		resp, err := json.Marshal(response)
		if err != nil {
			writeCustomErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrInternalError), err.Error(), r.URL)
			return
		}
		writeSuccessResponseJSON(w, resp)
		return
	}

	resp, err := json.Marshal(response)
	if err != nil {
		writeCustomErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrInternalError), err.Error(), r.URL)
		return
	}
	writeSuccessResponseJSON(w, resp)
}

// ServerHardwareInfoHandler - GET /minio/admin/v2/hardwareinfo?Type={hwType}
// ----------
// Get all hardware information based on input type
// Supported types = cpu
func (a adminAPIHandlers) ServerHardwareInfoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "HardwareInfo")

	objectAPI, _ := validateAdminReq(ctx, w, r, iampolicy.ListServerInfoAdminAction)
	if objectAPI == nil {
		return
	}

	vars := mux.Vars(r)
	hardware := vars[madmin.HARDWARE]

	switch madmin.HardwareType(hardware) {
	case madmin.CPU:
		// Get CPU hardware details from local server's cpu(s)
		cpu := getLocalCPUInfo(globalEndpoints, r)
		// Notify all other MinIO peers to report cpu hardware
		cpus := globalNotificationSys.CPUInfo()
		cpus = append(cpus, cpu)

		// Marshal API response
		jsonBytes, err := json.Marshal(cpus)
		if err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}

		// Reply with cpu hardware information (across nodes in a
		// distributed setup) as json.
		writeSuccessResponseJSON(w, jsonBytes)

	case madmin.NETWORK:
		// Get Network hardware details from local server's network(s)
		network := getLocalNetworkInfo(globalEndpoints, r)
		// Notify all other MinIO peers to report network hardware
		networks := globalNotificationSys.NetworkInfo()
		networks = append(networks, network)

		// Marshal API response
		jsonBytes, err := json.Marshal(networks)
		if err != nil {
			writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
			return
		}

		// Reply with cpu network information (across nodes in a
		// distributed setup) as json.
		writeSuccessResponseJSON(w, jsonBytes)

	default:
		writeErrorResponseJSON(ctx, w, errorCodes.ToAPIErr(ErrBadRequest), r.URL)
	}
}

// ServerInfoHandler - GET /minio/admin/v2/info
// ----------
// Get server information
func (a adminAPIHandlers) ServerInfoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "ServerInfo")
	objectAPI, _ := validateAdminReq(ctx, w, r, "")
	if objectAPI == nil {
		return
	}

	cfg, err := readServerConfig(ctx, objectAPI)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	buckets := madmin.Buckets{}
	objects := madmin.Objects{}
	usage := madmin.Usage{}

	dataUsageInfo, err := loadDataUsageFromBackend(ctx, objectAPI)
	if err == nil {
		buckets = madmin.Buckets{Count: dataUsageInfo.BucketsCount}
		objects = madmin.Objects{Count: dataUsageInfo.ObjectsCount}
		usage = madmin.Usage{Size: dataUsageInfo.ObjectsTotalSize}
	}

	infoMsg := madmin.InfoMessage{}
	vault := fetchVaultStatus(cfg)

	ldap := madmin.LDAP{}
	if globalLDAPConfig.Enabled {
		ldapConn, err := globalLDAPConfig.Connect()
		if err != nil {
			ldap.Status = "offline"
		} else if ldapConn == nil {
			ldap.Status = "Not Configured"
		} else {
			ldap.Status = "online"
		}
		// Close ldap connection to avoid leaks.
		defer ldapConn.Close()
	}

	log, audit := fetchLoggerInfo(cfg)

	// Get the notification target info
	notifyTarget := fetchLambdaInfo(cfg)

	// Fetching the Storage information
	storageInfo := objectAPI.StorageInfo(ctx)

	var OnDisks int
	var OffDisks int
	var backend interface{}

	if storageInfo.Backend.Type == BackendType(madmin.Erasure) {

		for _, v := range storageInfo.Backend.OnlineDisks {
			OnDisks += v
		}
		for _, v := range storageInfo.Backend.OfflineDisks {
			OffDisks += v
		}

		backend = madmin.XlBackend{
			Type:             madmin.ErasureType,
			OnlineDisks:      OnDisks,
			OfflineDisks:     OffDisks,
			StandardSCData:   storageInfo.Backend.StandardSCData,
			StandardSCParity: storageInfo.Backend.StandardSCParity,
			RRSCData:         storageInfo.Backend.RRSCData,
			RRSCParity:       storageInfo.Backend.RRSCParity,
		}
	} else {
		backend = madmin.FsBackend{
			Type: madmin.FsType,
		}
	}

	mode := ""
	if globalSafeMode {
		mode = "safe"
	} else {
		mode = "online"
	}

	server := getLocalServerProperty(globalEndpoints, r)
	servers := globalNotificationSys.ServerInfo()
	servers = append(servers, server)

	for _, sp := range servers {
		for i, di := range sp.Disks {
			path := ""
			if globalIsXL {
				path = di.DrivePath
			}
			if globalIsDistXL {
				path = sp.Endpoint + di.DrivePath
			}
			// For distributed
			for a := range storageInfo.Backend.Sets {
				for b := range storageInfo.Backend.Sets[a] {
					ep := storageInfo.Backend.Sets[a][b].Endpoint

					if globalIsDistXL {
						if strings.Replace(ep, "http://", "", -1) == path || strings.Replace(ep, "https://", "", -1) == path {
							sp.Disks[i].State = storageInfo.Backend.Sets[a][b].State
							sp.Disks[i].UUID = storageInfo.Backend.Sets[a][b].UUID
						}
					}
					if globalIsXL {
						if ep == path {
							sp.Disks[i].State = storageInfo.Backend.Sets[a][b].State
							sp.Disks[i].UUID = storageInfo.Backend.Sets[a][b].UUID
						}
					}
				}
			}

		}
	}

	domain := globalDomainNames
	services := madmin.Services{
		Vault:         vault,
		LDAP:          ldap,
		Logger:        log,
		Audit:         audit,
		Notifications: notifyTarget,
	}

	infoMsg = madmin.InfoMessage{
		Mode:         mode,
		Domain:       domain,
		Region:       globalServerRegion,
		SQSARN:       globalNotificationSys.GetARNList(),
		DeploymentID: globalDeploymentID,
		Buckets:      buckets,
		Objects:      objects,
		Usage:        usage,
		Services:     services,
		Backend:      backend,
		Servers:      servers,
	}

	// Marshal API response
	jsonBytes, err := json.Marshal(infoMsg)
	if err != nil {
		writeErrorResponseJSON(ctx, w, toAdminAPIErr(ctx, err), r.URL)
		return
	}

	//Reply with storage information (across nodes in a
	// distributed setup) as json.
	writeSuccessResponseJSON(w, jsonBytes)
}

func fetchLambdaInfo(cfg config.Config) []map[string][]madmin.TargetIDStatus {
	lambdaMap := make(map[string][]madmin.TargetIDStatus)
	targetList, _ := notify.GetNotificationTargets(cfg, GlobalServiceDoneCh, NewCustomHTTPTransport())

	for targetID, target := range targetList.TargetMap() {
		targetIDStatus := make(map[string]madmin.Status)
		active, _ := target.IsActive()
		if active {
			targetIDStatus[targetID.ID] = madmin.Status{Status: "Online"}
		} else {
			targetIDStatus[targetID.ID] = madmin.Status{Status: "Offline"}
		}
		list := lambdaMap[targetID.Name]
		list = append(list, targetIDStatus)
		lambdaMap[targetID.Name] = list
	}

	notify := make([]map[string][]madmin.TargetIDStatus, len(lambdaMap))
	counter := 0
	for key, value := range lambdaMap {
		v := make(map[string][]madmin.TargetIDStatus)
		v[key] = value
		notify[counter] = v
		counter++
	}
	return notify
}

// fetchVaultStatus fetches Vault Info
func fetchVaultStatus(cfg config.Config) madmin.Vault {
	vault := madmin.Vault{}
	if GlobalKMS == nil {
		vault.Status = "disabled"
		return vault
	}
	keyID := GlobalKMS.KeyID()
	kmsInfo := GlobalKMS.Info()

	if kmsInfo.Endpoint == "" {
		vault.Status = "KMS configured using master key"
		return vault
	}

	if err := checkConnection(kmsInfo.Endpoint); err != nil {

		vault.Status = "offline"
	} else {
		vault.Status = "online"

		kmsContext := crypto.Context{"MinIO admin API": "KMSKeyStatusHandler"} // Context for a test key operation
		// 1. Generate a new key using the KMS.
		key, sealedKey, err := GlobalKMS.GenerateKey(keyID, kmsContext)
		if err != nil {
			vault.Encrypt = "Encryption failed"
		} else {
			vault.Encrypt = "Ok"
		}

		// 2. Check whether we can update / re-wrap the sealed key.
		sealedKey, err = GlobalKMS.UpdateKey(keyID, sealedKey, kmsContext)
		if err != nil {
			vault.Update = "Re-wrap failed:"
		} else {
			vault.Update = "Ok"
		}

		// 3. Verify that we can indeed decrypt the (encrypted) key
		decryptedKey, decryptErr := GlobalKMS.UnsealKey(keyID, sealedKey, kmsContext)

		// 4. Compare generated key with decrypted key
		if subtle.ConstantTimeCompare(key[:], decryptedKey[:]) != 1 || decryptErr != nil {
			vault.Decrypt = "Re-wrap failed:"
		} else {
			vault.Decrypt = "Ok"
		}
	}
	return vault
}

// fetchLoggerDetails return log info
func fetchLoggerInfo(cfg config.Config) ([]madmin.Logger, []madmin.Audit) {
	loggerCfg, _ := logger.LookupConfig(cfg)

	var logger []madmin.Logger
	var auditlogger []madmin.Audit
	for log, l := range loggerCfg.HTTP {
		if l.Enabled {
			err := checkConnection(l.Endpoint)
			if err == nil {
				mapLog := make(map[string]madmin.Status)
				mapLog[log] = madmin.Status{Status: "Online"}
				logger = append(logger, mapLog)
			} else {
				mapLog := make(map[string]madmin.Status)
				mapLog[log] = madmin.Status{Status: "offline"}
				logger = append(logger, mapLog)
			}
		}
	}

	for audit, l := range loggerCfg.Audit {
		if l.Enabled {
			err := checkConnection(l.Endpoint)
			if err == nil {
				mapAudit := make(map[string]madmin.Status)
				mapAudit[audit] = madmin.Status{Status: "Online"}
				auditlogger = append(auditlogger, mapAudit)
			} else {
				mapAudit := make(map[string]madmin.Status)
				mapAudit[audit] = madmin.Status{Status: "Offline"}
				auditlogger = append(auditlogger, mapAudit)
			}
		}
	}
	return logger, auditlogger
}

// checkConnection - ping an endpoint , return err in case of no connection
func checkConnection(endpointStr string) error {
	u, pErr := xnet.ParseURL(endpointStr)
	if pErr != nil {
		return pErr
	}
	if dErr := u.DialHTTP(); dErr != nil {
		if urlErr, ok := dErr.(*url.Error); ok {
			// To treat "connection refused" errors as un reachable endpoint.
			if target.IsConnRefusedErr(urlErr.Err) {
				return errors.New("endpoint unreachable, please check your endpoint")
			}
		}
		return dErr
	}
	return nil
}
