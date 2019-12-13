/*
 * MinIO Cloud Storage, (C) 2018-2019 MinIO, Inc.
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

package openid

import (
	"crypto"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	xnet "github.com/minio/minio/pkg/net"
)

func TestUpdateClaimsExpiry(t *testing.T) {
	testCases := []struct {
		exp             interface{}
		dsecs           string
		expectedFailure bool
	}{
		{"", "", true},
		{"-1", "0", true},
		{"-1", "900", true},
		{"1574812326", "900", false},
		{1574812326, "900", false},
		{int64(1574812326), "900", false},
		{int(1574812326), "900", false},
		{uint(1574812326), "900", false},
		{uint64(1574812326), "900", false},
		{json.Number("1574812326"), "900", false},
		{1574812326.000, "900", false},
		{time.Duration(3) * time.Minute, "900", false},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run("", func(t *testing.T) {
			claims := map[string]interface{}{}
			claims["exp"] = testCase.exp
			err := updateClaimsExpiry(testCase.dsecs, claims)
			if err != nil && !testCase.expectedFailure {
				t.Errorf("Expected success, got failure %s", err)
			}
			if err == nil && testCase.expectedFailure {
				t.Error("Expected failure, got success")
			}
		})
	}
}

func TestJWT(t *testing.T) {
	const jsonkey = `{"keys":
       [
         {"kty":"RSA",
          "n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
          "e":"AQAB",
          "alg":"RS256",
          "kid":"2011-04-29"}
       ]
     }`

	var jk JWKS
	if err := json.Unmarshal([]byte(jsonkey), &jk); err != nil {
		t.Fatal("Unmarshal: ", err)
	} else if len(jk.Keys) != 1 {
		t.Fatalf("Expected 1 keys, got %d", len(jk.Keys))
	}

	keys := make(map[string]crypto.PublicKey, len(jk.Keys))
	for ii, jks := range jk.Keys {
		var err error
		keys[jks.Kid], err = jks.DecodePublicKey()
		if err != nil {
			t.Fatalf("Failed to decode key %d: %v", ii, err)
		}
	}

	u1, err := xnet.ParseHTTPURL("http://localhost:8443")
	if err != nil {
		t.Fatal(err)
	}

	cfg := Config{}
	cfg.JWKS.URL = u1
	cfg.publicKeys = keys
	jwt := NewJWT(cfg)
	if jwt.ID() != "jwt" {
		t.Fatalf("Uexpected id %s for the validator", jwt.ID())
	}

	u, err := url.Parse("http://localhost:8443/?Token=invalid")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jwt.Validate(u.Query().Get("Token"), ""); err == nil {
		t.Fatal(err)
	}
}

func TestDefaultExpiryDuration(t *testing.T) {
	testCases := []struct {
		reqURL    string
		duration  time.Duration
		expectErr bool
	}{
		{
			reqURL:   "http://localhost:8443/?Token=xxxxx",
			duration: time.Duration(60) * time.Minute,
		},
		{
			reqURL:    "http://localhost:8443/?DurationSeconds=9s",
			expectErr: true,
		},
		{
			reqURL:    "http://localhost:8443/?DurationSeconds=43201",
			expectErr: true,
		},
		{
			reqURL:    "http://localhost:8443/?DurationSeconds=800",
			expectErr: true,
		},
		{
			reqURL:   "http://localhost:8443/?DurationSeconds=901",
			duration: time.Duration(901) * time.Second,
		},
	}

	for i, testCase := range testCases {
		u, err := url.Parse(testCase.reqURL)
		if err != nil {
			t.Fatal(err)
		}
		d, err := GetDefaultExpiration(u.Query().Get("DurationSeconds"))
		gotErr := (err != nil)
		if testCase.expectErr != gotErr {
			t.Errorf("Test %d: Expected %v, got %v with error %s", i+1, testCase.expectErr, gotErr, err)
		}
		if d != testCase.duration {
			t.Errorf("Test %d: Expected duration %d, got %d", i+1, testCase.duration, d)
		}
	}
}
