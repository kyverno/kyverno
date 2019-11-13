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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/madmin"
)

// adminXLTestBed - encapsulates subsystems that need to be setup for
// admin-handler unit tests.
type adminXLTestBed struct {
	xlDirs   []string
	objLayer ObjectLayer
	router   *mux.Router
}

// prepareAdminXLTestBed - helper function that setups a single-node
// XL backend for admin-handler tests.
func prepareAdminXLTestBed() (*adminXLTestBed, error) {
	// reset global variables to start afresh.
	resetTestGlobals()

	// Initializing objectLayer for HealFormatHandler.
	objLayer, xlDirs, xlErr := initTestXLObjLayer()
	if xlErr != nil {
		return nil, xlErr
	}

	// Initialize minio server config.
	if err := newTestConfig(globalMinioDefaultRegion, objLayer); err != nil {
		return nil, err
	}

	// Initialize boot time
	globalBootTime = UTCNow()

	globalEndpoints = mustGetNewEndpointList(xlDirs...)

	// Set globalIsXL to indicate that the setup uses an erasure
	// code backend.
	globalIsXL = true

	// initialize NSLock.
	isDistXL := false
	initNSLock(isDistXL)

	// Init global heal state
	if globalIsXL {
		globalAllHealState = initHealState()
	}

	globalConfigSys = NewConfigSys()

	globalIAMSys = NewIAMSys()
	globalIAMSys.Init(objLayer)

	buckets, err := objLayer.ListBuckets(context.Background())
	if err != nil {
		return nil, err
	}

	globalPolicySys = NewPolicySys()
	globalPolicySys.Init(buckets, objLayer)

	globalNotificationSys = NewNotificationSys(globalEndpoints)
	globalNotificationSys.Init(buckets, objLayer)

	// Setup admin mgmt REST API handlers.
	adminRouter := mux.NewRouter()
	registerAdminRouter(adminRouter, true, true)

	return &adminXLTestBed{
		xlDirs:   xlDirs,
		objLayer: objLayer,
		router:   adminRouter,
	}, nil
}

// TearDown - method that resets the test bed for subsequent unit
// tests to start afresh.
func (atb *adminXLTestBed) TearDown() {
	removeRoots(atb.xlDirs)
	resetTestGlobals()
}

// initTestObjLayer - Helper function to initialize an XL-based object
// layer and set globalObjectAPI.
func initTestXLObjLayer() (ObjectLayer, []string, error) {
	xlDirs, err := getRandomDisks(16)
	if err != nil {
		return nil, nil, err
	}
	endpoints := mustGetNewEndpointList(xlDirs...)
	format, err := waitForFormatXL(true, endpoints, 1, 16)
	if err != nil {
		removeRoots(xlDirs)
		return nil, nil, err
	}

	globalPolicySys = NewPolicySys()
	objLayer, err := newXLSets(endpoints, format, 1, 16)
	if err != nil {
		return nil, nil, err
	}

	// Make objLayer available to all internal services via globalObjectAPI.
	globalObjLayerMutex.Lock()
	globalObjectAPI = objLayer
	globalObjLayerMutex.Unlock()
	return objLayer, xlDirs, nil
}

// cmdType - Represents different service subcomands like status, stop
// and restart.
type cmdType int

const (
	restartCmd cmdType = iota
	stopCmd
)

// toServiceSignal - Helper function that translates a given cmdType
// value to its corresponding serviceSignal value.
func (c cmdType) toServiceSignal() serviceSignal {
	switch c {
	case restartCmd:
		return serviceRestart
	case stopCmd:
		return serviceStop
	}
	return serviceRestart
}

func (c cmdType) toServiceAction() madmin.ServiceAction {
	switch c {
	case restartCmd:
		return madmin.ServiceActionRestart
	case stopCmd:
		return madmin.ServiceActionStop
	}
	return madmin.ServiceActionRestart
}

// testServiceSignalReceiver - Helper function that simulates a
// go-routine waiting on service signal.
func testServiceSignalReceiver(cmd cmdType, t *testing.T) {
	expectedCmd := cmd.toServiceSignal()
	serviceCmd := <-globalServiceSignalCh
	if serviceCmd != expectedCmd {
		t.Errorf("Expected service command %v but received %v", expectedCmd, serviceCmd)
	}
}

// getServiceCmdRequest - Constructs a management REST API request for service
// subcommands for a given cmdType value.
func getServiceCmdRequest(cmd cmdType, cred auth.Credentials) (*http.Request, error) {
	queryVal := url.Values{}
	queryVal.Set("action", string(cmd.toServiceAction()))
	resource := adminPathPrefix + adminAPIVersionPrefix + "/service?" + queryVal.Encode()
	req, err := newTestRequest(http.MethodPost, resource, 0, nil)
	if err != nil {
		return nil, err
	}

	// management REST API uses signature V4 for authentication.
	err = signRequestV4(req, cred.AccessKey, cred.SecretKey)
	if err != nil {
		return nil, err
	}
	return req, nil
}

// testServicesCmdHandler - parametrizes service subcommand tests on
// cmdType value.
func testServicesCmdHandler(cmd cmdType, t *testing.T) {
	adminTestBed, err := prepareAdminXLTestBed()
	if err != nil {
		t.Fatal("Failed to initialize a single node XL backend for admin handler tests.")
	}
	defer adminTestBed.TearDown()

	// Initialize admin peers to make admin RPC calls. Note: In a
	// single node setup, this degenerates to a simple function
	// call under the hood.
	globalMinioAddr = "127.0.0.1:9000"

	var wg sync.WaitGroup

	// Setting up a go routine to simulate ServerRouter's
	// handleServiceSignals for stop and restart commands.
	if cmd == restartCmd {
		wg.Add(1)
		go func() {
			defer wg.Done()
			testServiceSignalReceiver(cmd, t)
		}()
	}
	credentials := globalActiveCred

	req, err := getServiceCmdRequest(cmd, credentials)
	if err != nil {
		t.Fatalf("Failed to build service status request %v", err)
	}

	rec := httptest.NewRecorder()
	adminTestBed.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		resp, _ := ioutil.ReadAll(rec.Body)
		t.Errorf("Expected to receive %d status code but received %d. Body (%s)",
			http.StatusOK, rec.Code, string(resp))
	}

	// Wait until testServiceSignalReceiver() called in a goroutine quits.
	wg.Wait()
}

// Test for service restart management REST API.
func TestServiceRestartHandler(t *testing.T) {
	testServicesCmdHandler(restartCmd, t)
}

// buildAdminRequest - helper function to build an admin API request.
func buildAdminRequest(queryVal url.Values, method, path string,
	contentLength int64, bodySeeker io.ReadSeeker) (*http.Request, error) {

	req, err := newTestRequest(method,
		adminPathPrefix+adminAPIVersionPrefix+path+"?"+queryVal.Encode(),
		contentLength, bodySeeker)
	if err != nil {
		return nil, err
	}

	cred := globalActiveCred
	err = signRequestV4(req, cred.AccessKey, cred.SecretKey)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func TestAdminServerInfo(t *testing.T) {
	adminTestBed, err := prepareAdminXLTestBed()
	if err != nil {
		t.Fatal("Failed to initialize a single node XL backend for admin handler tests.")
	}
	defer adminTestBed.TearDown()

	// Initialize admin peers to make admin RPC calls.
	globalMinioAddr = "127.0.0.1:9000"

	// Prepare query params for set-config mgmt REST API.
	queryVal := url.Values{}
	queryVal.Set("info", "")

	req, err := buildAdminRequest(queryVal, http.MethodGet, "/info", 0, nil)
	if err != nil {
		t.Fatalf("Failed to construct get-config object request - %v", err)
	}

	rec := httptest.NewRecorder()
	adminTestBed.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected to succeed but failed with %d", rec.Code)
	}

	results := []ServerInfo{}
	err = json.NewDecoder(rec.Body).Decode(&results)
	if err != nil {
		t.Fatalf("Failed to decode set config result json %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected at least one server info result")
	}

	for _, serverInfo := range results {
		if serverInfo.Error != "" {
			t.Errorf("Unexpected error = %v\n", serverInfo.Error)
		}
		if serverInfo.Data.Properties.Region != globalMinioDefaultRegion {
			t.Errorf("Expected %s, got %s", globalMinioDefaultRegion, serverInfo.Data.Properties.Region)
		}
	}
}

// TestToAdminAPIErrCode - test for toAdminAPIErrCode helper function.
func TestToAdminAPIErrCode(t *testing.T) {
	testCases := []struct {
		err            error
		expectedAPIErr APIErrorCode
	}{
		// 1. Server not in quorum.
		{
			err:            errXLWriteQuorum,
			expectedAPIErr: ErrAdminConfigNoQuorum,
		},
		// 2. No error.
		{
			err:            nil,
			expectedAPIErr: ErrNone,
		},
		// 3. Non-admin API specific error.
		{
			err:            errDiskNotFound,
			expectedAPIErr: toAPIErrorCode(context.Background(), errDiskNotFound),
		},
	}

	for i, test := range testCases {
		actualErr := toAdminAPIErrCode(context.Background(), test.err)
		if actualErr != test.expectedAPIErr {
			t.Errorf("Test %d: Expected %v but received %v",
				i+1, test.expectedAPIErr, actualErr)
		}
	}
}

func TestTopLockEntries(t *testing.T) {
	t1 := UTCNow()
	t2 := UTCNow().Add(10 * time.Second)
	peerLocks := []*PeerLocks{
		{
			Addr: "1",
			Locks: map[string][]lockRequesterInfo{
				"1": {
					{false, "node2", "ep2", "2", t2, t2, ""},
					{true, "node1", "ep1", "1", t1, t1, ""},
				},
				"2": {
					{false, "node2", "ep2", "2", t2, t2, ""},
					{true, "node1", "ep1", "1", t1, t1, ""},
				},
			},
		},
		{
			Addr: "2",
			Locks: map[string][]lockRequesterInfo{
				"1": {
					{false, "node2", "ep2", "2", t2, t2, ""},
					{true, "node1", "ep1", "1", t1, t1, ""},
				},
				"2": {
					{false, "node2", "ep2", "2", t2, t2, ""},
					{true, "node1", "ep1", "1", t1, t1, ""},
				},
			},
		},
	}
	les := topLockEntries(peerLocks)
	if len(les) != 2 {
		t.Fatalf("Did not get 2 results")
	}
	if les[0].Timestamp.After(les[1].Timestamp) {
		t.Fatalf("Got wrong sorted value")
	}
}

func TestExtractHealInitParams(t *testing.T) {
	mkParams := func(clientToken string, forceStart, forceStop bool) url.Values {
		v := url.Values{}
		if clientToken != "" {
			v.Add(string(mgmtClientToken), clientToken)
		}
		if forceStart {
			v.Add(string(mgmtForceStart), "")
		}
		if forceStop {
			v.Add(string(mgmtForceStop), "")
		}
		return v
	}
	qParmsArr := []url.Values{
		// Invalid cases
		mkParams("", true, true),
		mkParams("111", true, true),
		mkParams("111", true, false),
		mkParams("111", false, true),
		// Valid cases follow
		mkParams("", true, false),
		mkParams("", false, true),
		mkParams("", false, false),
		mkParams("111", false, false),
	}
	varsArr := []map[string]string{
		// Invalid cases
		{string(mgmtPrefix): "objprefix"},
		// Valid cases
		{},
		{string(mgmtBucket): "bucket"},
		{string(mgmtBucket): "bucket", string(mgmtPrefix): "objprefix"},
	}

	// Body is always valid - we do not test JSON decoding.
	body := `{"recursive": false, "dryRun": true, "remove": false, "scanMode": 0}`

	// Test all combinations!
	for pIdx, parms := range qParmsArr {
		for vIdx, vars := range varsArr {
			_, err := extractHealInitParams(vars, parms, bytes.NewBuffer([]byte(body)))
			isErrCase := false
			if pIdx < 4 || vIdx < 1 {
				isErrCase = true
			}

			if err != ErrNone && !isErrCase {
				t.Errorf("Got unexpected error: %v %v %v", pIdx, vIdx, err)
			} else if err == ErrNone && isErrCase {
				t.Errorf("Got no error but expected one: %v %v", pIdx, vIdx)
			}
		}
	}

}
