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

package hash

import (
	"bytes"
	"encoding/hex"
	"io"
	"io/ioutil"
	"testing"
)

// Tests functions like Size(), MD5*(), SHA256*()
func TestHashReaderHelperMethods(t *testing.T) {
	r, err := NewReader(bytes.NewReader([]byte("abcd")), 4, "e2fc714c4727ee9395f324cd2e7f331f", "88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589", 4, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(ioutil.Discard, r)
	if err != nil {
		t.Fatal(err)
	}
	if r.MD5HexString() != "e2fc714c4727ee9395f324cd2e7f331f" {
		t.Errorf("Expected md5hex \"e2fc714c4727ee9395f324cd2e7f331f\", got %s", r.MD5HexString())
	}
	if r.SHA256HexString() != "88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589" {
		t.Errorf("Expected sha256hex \"88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589\", got %s", r.SHA256HexString())
	}
	if r.MD5Base64String() != "4vxxTEcn7pOV8yTNLn8zHw==" {
		t.Errorf("Expected md5base64 \"4vxxTEcn7pOV8yTNLn8zHw==\", got \"%s\"", r.MD5Base64String())
	}
	if r.Size() != 4 {
		t.Errorf("Expected size 4, got %d", r.Size())
	}
	if r.ActualSize() != 4 {
		t.Errorf("Expected size 4, got %d", r.ActualSize())
	}
	expectedMD5, err := hex.DecodeString("e2fc714c4727ee9395f324cd2e7f331f")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.MD5(), expectedMD5) {
		t.Errorf("Expected md5hex \"e2fc714c4727ee9395f324cd2e7f331f\", got %s", r.MD5HexString())
	}
	if !bytes.Equal(r.MD5Current(), expectedMD5) {
		t.Errorf("Expected md5hex \"e2fc714c4727ee9395f324cd2e7f331f\", got %s", hex.EncodeToString(r.MD5Current()))
	}
	expectedSHA256, err := hex.DecodeString("88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.SHA256(), expectedSHA256) {
		t.Errorf("Expected md5hex \"88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589\", got %s", r.SHA256HexString())
	}
}

// Tests hash reader checksum verification.
func TestHashReaderVerification(t *testing.T) {
	testCases := []struct {
		src               io.Reader
		size              int64
		actualSize        int64
		md5hex, sha256hex string
		err               error
	}{
		// Success, no checksum verification provided.
		{
			src:        bytes.NewReader([]byte("abcd")),
			size:       4,
			actualSize: 4,
		},
		// Failure md5 mismatch.
		{
			src:        bytes.NewReader([]byte("abcd")),
			size:       4,
			actualSize: 4,
			md5hex:     "d41d8cd98f00b204e9800998ecf8427f",
			err: BadDigest{
				"d41d8cd98f00b204e9800998ecf8427f",
				"e2fc714c4727ee9395f324cd2e7f331f",
			},
		},
		// Failure sha256 mismatch.
		{
			src:        bytes.NewReader([]byte("abcd")),
			size:       4,
			actualSize: 4,
			sha256hex:  "88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031580",
			err: SHA256Mismatch{
				"88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031580",
				"88d4266fd4e6338d13b845fcf289579d209c897823b9217da3e161936f031589",
			},
		},
	}
	for i, testCase := range testCases {
		r, err := NewReader(testCase.src, testCase.size, testCase.md5hex, testCase.sha256hex, testCase.actualSize, false)
		if err != nil {
			t.Fatalf("Test %d: Initializing reader failed %s", i+1, err)
		}
		_, err = io.Copy(ioutil.Discard, r)
		if err != nil {
			if err.Error() != testCase.err.Error() {
				t.Errorf("Test %d: Expected error %s, got error %s", i+1, testCase.err, err)
			}
		}
	}
}

// Tests NewReader() constructor with invalid arguments.
func TestHashReaderInvalidArguments(t *testing.T) {
	testCases := []struct {
		src               io.Reader
		size              int64
		actualSize        int64
		md5hex, sha256hex string
		success           bool
		expectedErr       error
	}{
		// Invalid md5sum NewReader() will fail.
		{
			src:         bytes.NewReader([]byte("abcd")),
			size:        4,
			actualSize:  4,
			md5hex:      "invalid-md5",
			success:     false,
			expectedErr: BadDigest{},
		},
		// Invalid sha256 NewReader() will fail.
		{
			src:         bytes.NewReader([]byte("abcd")),
			size:        4,
			actualSize:  4,
			sha256hex:   "invalid-sha256",
			success:     false,
			expectedErr: SHA256Mismatch{},
		},
		// Nested hash reader NewReader() will fail.
		{
			src:         &Reader{src: bytes.NewReader([]byte("abcd"))},
			size:        4,
			actualSize:  4,
			success:     false,
			expectedErr: errNestedReader,
		},
		// Expected inputs, NewReader() will succeed.
		{
			src:        bytes.NewReader([]byte("abcd")),
			size:       4,
			actualSize: 4,
			success:    true,
		},
	}

	for i, testCase := range testCases {
		_, err := NewReader(testCase.src, testCase.size, testCase.md5hex, testCase.sha256hex, testCase.actualSize, false)
		if err != nil && testCase.success {
			t.Errorf("Test %d: Expected success, but got error %s instead", i+1, err)
		}
		if err == nil && !testCase.success {
			t.Errorf("Test %d: Expected error, but got success", i+1)
		}
		if err != testCase.expectedErr {
			t.Errorf("Test %d: Expected error %v, but got %v", i+1, testCase.expectedErr, err)
		}
	}
}
