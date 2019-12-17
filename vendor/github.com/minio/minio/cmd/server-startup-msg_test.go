/*
 * MinIO Cloud Storage, (C) 2016, 2017 MinIO, Inc.
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
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio/pkg/color"
	"github.com/minio/minio/pkg/madmin"
)

// Tests if we generate storage info.
func TestStorageInfoMsg(t *testing.T) {
	infoStorage := StorageInfo{}
	infoStorage.Backend.Type = BackendErasure
	infoStorage.Backend.OnlineDisks = madmin.BackendDisks{"127.0.0.1:9000": 4, "127.0.0.1:9001": 3}
	infoStorage.Backend.OfflineDisks = madmin.BackendDisks{"127.0.0.1:9000": 0, "127.0.0.1:9001": 1}

	if msg := getStorageInfoMsg(infoStorage); !strings.Contains(msg, "7 Online, 1 Offline") {
		t.Fatal("Unexpected storage info message, found:", msg)
	}
}

// Tests if certificate expiry warning will be printed
func TestCertificateExpiryInfo(t *testing.T) {
	// given
	var expiredDate = time.Now().Add(time.Hour * 24 * (30 - 1)) // 29 days.

	var fakeCerts = []*x509.Certificate{
		{
			NotAfter: expiredDate,
			Subject: pkix.Name{
				CommonName: "Test cert",
			},
		},
	}

	expectedMsg := color.Blue("\nCertificate expiry info:\n") +
		color.Bold(fmt.Sprintf("#1 Test cert will expire on %s\n", expiredDate))

	// When
	msg := getCertificateChainMsg(fakeCerts)

	// Then
	if msg != expectedMsg {
		t.Fatalf("Expected message was: %s, got: %s", expectedMsg, msg)
	}
}

// Tests if certificate expiry warning will not be printed if certificate not expired
func TestCertificateNotExpired(t *testing.T) {
	// given
	var expiredDate = time.Now().Add(time.Hour * 24 * (30 + 1)) // 31 days.

	var fakeCerts = []*x509.Certificate{
		{
			NotAfter: expiredDate,
			Subject: pkix.Name{
				CommonName: "Test cert",
			},
		},
	}

	// when
	msg := getCertificateChainMsg(fakeCerts)

	// then
	if msg != "" {
		t.Fatalf("Expected empty message was: %s", msg)
	}
}

// Tests stripping standard ports from apiEndpoints.
func TestStripStandardPorts(t *testing.T) {
	apiEndpoints := []string{"http://127.0.0.1:9000", "http://127.0.0.2:80", "https://127.0.0.3:443"}
	expectedAPIEndpoints := []string{"http://127.0.0.1:9000", "http://127.0.0.2", "https://127.0.0.3"}
	newAPIEndpoints := stripStandardPorts(apiEndpoints)

	if !reflect.DeepEqual(expectedAPIEndpoints, newAPIEndpoints) {
		t.Fatalf("Expected %#v, got %#v", expectedAPIEndpoints, newAPIEndpoints)
	}

	apiEndpoints = []string{"http://%%%%%:9000"}
	newAPIEndpoints = stripStandardPorts(apiEndpoints)
	if !reflect.DeepEqual([]string{""}, newAPIEndpoints) {
		t.Fatalf("Expected %#v, got %#v", apiEndpoints, newAPIEndpoints)
	}

	apiEndpoints = []string{"http://127.0.0.1:443", "https://127.0.0.1:80"}
	newAPIEndpoints = stripStandardPorts(apiEndpoints)
	if !reflect.DeepEqual(apiEndpoints, newAPIEndpoints) {
		t.Fatalf("Expected %#v, got %#v", apiEndpoints, newAPIEndpoints)
	}
}

// Test printing server common message.
func TestPrintServerCommonMessage(t *testing.T) {
	obj, fsDir, err := prepareFS()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fsDir)
	if err = newTestConfig(globalMinioDefaultRegion, obj); err != nil {
		t.Fatal(err)
	}

	apiEndpoints := []string{"http://127.0.0.1:9000"}
	printServerCommonMsg(apiEndpoints)
}

// Tests print cli access message.
func TestPrintCLIAccessMsg(t *testing.T) {
	obj, fsDir, err := prepareFS()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fsDir)
	if err = newTestConfig(globalMinioDefaultRegion, obj); err != nil {
		t.Fatal(err)
	}

	apiEndpoints := []string{"http://127.0.0.1:9000"}
	printCLIAccessMsg(apiEndpoints[0], "myminio")
}

// Test print startup message.
func TestPrintStartupMessage(t *testing.T) {
	obj, fsDir, err := prepareFS()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fsDir)
	if err = newTestConfig(globalMinioDefaultRegion, obj); err != nil {
		t.Fatal(err)
	}

	apiEndpoints := []string{"http://127.0.0.1:9000"}
	printStartupMessage(apiEndpoints)
}
