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
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/minio/minio/pkg/auth"
	iampolicy "github.com/minio/minio/pkg/iam/policy"
)

// Test get request auth type.
func TestGetRequestAuthType(t *testing.T) {
	type testCase struct {
		req   *http.Request
		authT authType
	}
	testCases := []testCase{
		// Test case - 1
		// Check for generic signature v4 header.
		{
			req: &http.Request{
				URL: &url.URL{
					Host:   "127.0.0.1:9000",
					Scheme: httpScheme,
					Path:   SlashSeparator,
				},
				Header: http.Header{
					"Authorization":        []string{"AWS4-HMAC-SHA256 <cred_string>"},
					"X-Amz-Content-Sha256": []string{streamingContentSHA256},
					"Content-Encoding":     []string{streamingContentEncoding},
				},
				Method: "PUT",
			},
			authT: authTypeStreamingSigned,
		},
		// Test case - 2
		// Check for JWT header.
		{
			req: &http.Request{
				URL: &url.URL{
					Host:   "127.0.0.1:9000",
					Scheme: httpScheme,
					Path:   SlashSeparator,
				},
				Header: http.Header{
					"Authorization": []string{"Bearer 12313123"},
				},
			},
			authT: authTypeJWT,
		},
		// Test case - 3
		// Empty authorization header.
		{
			req: &http.Request{
				URL: &url.URL{
					Host:   "127.0.0.1:9000",
					Scheme: httpScheme,
					Path:   SlashSeparator,
				},
				Header: http.Header{
					"Authorization": []string{""},
				},
			},
			authT: authTypeUnknown,
		},
		// Test case - 4
		// Check for presigned.
		{
			req: &http.Request{
				URL: &url.URL{
					Host:     "127.0.0.1:9000",
					Scheme:   httpScheme,
					Path:     SlashSeparator,
					RawQuery: "X-Amz-Credential=EXAMPLEINVALIDEXAMPL%2Fs3%2F20160314%2Fus-east-1",
				},
			},
			authT: authTypePresigned,
		},
		// Test case - 5
		// Check for post policy.
		{
			req: &http.Request{
				URL: &url.URL{
					Host:   "127.0.0.1:9000",
					Scheme: httpScheme,
					Path:   SlashSeparator,
				},
				Header: http.Header{
					"Content-Type": []string{"multipart/form-data"},
				},
				Method: "POST",
			},
			authT: authTypePostPolicy,
		},
	}

	// .. Tests all request auth type.
	for i, testc := range testCases {
		authT := getRequestAuthType(testc.req)
		if authT != testc.authT {
			t.Errorf("Test %d: Expected %d, got %d", i+1, testc.authT, authT)
		}
	}
}

// Test all s3 supported auth types.
func TestS3SupportedAuthType(t *testing.T) {
	type testCase struct {
		authT authType
		pass  bool
	}
	// List of all valid and invalid test cases.
	testCases := []testCase{
		// Test 1 - supported s3 type anonymous.
		{
			authT: authTypeAnonymous,
			pass:  true,
		},
		// Test 2 - supported s3 type presigned.
		{
			authT: authTypePresigned,
			pass:  true,
		},
		// Test 3 - supported s3 type signed.
		{
			authT: authTypeSigned,
			pass:  true,
		},
		// Test 4 - supported s3 type with post policy.
		{
			authT: authTypePostPolicy,
			pass:  true,
		},
		// Test 5 - supported s3 type with streaming signed.
		{
			authT: authTypeStreamingSigned,
			pass:  true,
		},
		// Test 6 - supported s3 type with signature v2.
		{
			authT: authTypeSignedV2,
			pass:  true,
		},
		// Test 7 - supported s3 type with presign v2.
		{
			authT: authTypePresignedV2,
			pass:  true,
		},
		// Test 8 - JWT is not supported s3 type.
		{
			authT: authTypeJWT,
			pass:  false,
		},
		// Test 9 - unknown auth header is not supported s3 type.
		{
			authT: authTypeUnknown,
			pass:  false,
		},
		// Test 10 - some new auth type is not supported s3 type.
		{
			authT: authType(9),
			pass:  false,
		},
	}
	// Validate all the test cases.
	for i, tt := range testCases {
		ok := isSupportedS3AuthType(tt.authT)
		if ok != tt.pass {
			t.Errorf("Test %d:, Expected %t, got %t", i+1, tt.pass, ok)
		}
	}
}

func TestIsRequestPresignedSignatureV2(t *testing.T) {
	testCases := []struct {
		inputQueryKey   string
		inputQueryValue string
		expectedResult  bool
	}{
		// Test case - 1.
		// Test case with query key "AWSAccessKeyId" set.
		{"", "", false},
		// Test case - 2.
		{"AWSAccessKeyId", "", true},
		// Test case - 3.
		{"X-Amz-Content-Sha256", "", false},
	}

	for i, testCase := range testCases {
		// creating an input HTTP request.
		// Only the query parameters are relevant for this particular test.
		inputReq, err := http.NewRequest("GET", "http://example.com", nil)
		if err != nil {
			t.Fatalf("Error initializing input HTTP request: %v", err)
		}
		q := inputReq.URL.Query()
		q.Add(testCase.inputQueryKey, testCase.inputQueryValue)
		inputReq.URL.RawQuery = q.Encode()

		actualResult := isRequestPresignedSignatureV2(inputReq)
		if testCase.expectedResult != actualResult {
			t.Errorf("Test %d: Expected the result to `%v`, but instead got `%v`", i+1, testCase.expectedResult, actualResult)
		}
	}
}

// TestIsRequestPresignedSignatureV4 - Test validates the logic for presign signature verision v4 detection.
func TestIsRequestPresignedSignatureV4(t *testing.T) {
	testCases := []struct {
		inputQueryKey   string
		inputQueryValue string
		expectedResult  bool
	}{
		// Test case - 1.
		// Test case with query key ""X-Amz-Credential" set.
		{"", "", false},
		// Test case - 2.
		{"X-Amz-Credential", "", true},
		// Test case - 3.
		{"X-Amz-Content-Sha256", "", false},
	}

	for i, testCase := range testCases {
		// creating an input HTTP request.
		// Only the query parameters are relevant for this particular test.
		inputReq, err := http.NewRequest("GET", "http://example.com", nil)
		if err != nil {
			t.Fatalf("Error initializing input HTTP request: %v", err)
		}
		q := inputReq.URL.Query()
		q.Add(testCase.inputQueryKey, testCase.inputQueryValue)
		inputReq.URL.RawQuery = q.Encode()

		actualResult := isRequestPresignedSignatureV4(inputReq)
		if testCase.expectedResult != actualResult {
			t.Errorf("Test %d: Expected the result to `%v`, but instead got `%v`", i+1, testCase.expectedResult, actualResult)
		}
	}
}

// Provides a fully populated http request instance, fails otherwise.
func mustNewRequest(method string, urlStr string, contentLength int64, body io.ReadSeeker, t *testing.T) *http.Request {
	req, err := newTestRequest(method, urlStr, contentLength, body)
	if err != nil {
		t.Fatalf("Unable to initialize new http request %s", err)
	}
	return req
}

// This is similar to mustNewRequest but additionally the request
// is signed with AWS Signature V4, fails if not able to do so.
func mustNewSignedRequest(method string, urlStr string, contentLength int64, body io.ReadSeeker, t *testing.T) *http.Request {
	req := mustNewRequest(method, urlStr, contentLength, body, t)
	cred := globalActiveCred
	if err := signRequestV4(req, cred.AccessKey, cred.SecretKey); err != nil {
		t.Fatalf("Unable to inititalized new signed http request %s", err)
	}
	return req
}

// This is similar to mustNewRequest but additionally the request
// is signed with AWS Signature V2, fails if not able to do so.
func mustNewSignedV2Request(method string, urlStr string, contentLength int64, body io.ReadSeeker, t *testing.T) *http.Request {
	req := mustNewRequest(method, urlStr, contentLength, body, t)
	cred := globalActiveCred
	if err := signRequestV2(req, cred.AccessKey, cred.SecretKey); err != nil {
		t.Fatalf("Unable to inititalized new signed http request %s", err)
	}
	return req
}

// This is similar to mustNewRequest but additionally the request
// is presigned with AWS Signature V2, fails if not able to do so.
func mustNewPresignedV2Request(method string, urlStr string, contentLength int64, body io.ReadSeeker, t *testing.T) *http.Request {
	req := mustNewRequest(method, urlStr, contentLength, body, t)
	cred := globalActiveCred
	if err := preSignV2(req, cred.AccessKey, cred.SecretKey, time.Now().Add(10*time.Minute).Unix()); err != nil {
		t.Fatalf("Unable to inititalized new signed http request %s", err)
	}
	return req
}

// This is similar to mustNewRequest but additionally the request
// is presigned with AWS Signature V4, fails if not able to do so.
func mustNewPresignedRequest(method string, urlStr string, contentLength int64, body io.ReadSeeker, t *testing.T) *http.Request {
	req := mustNewRequest(method, urlStr, contentLength, body, t)
	cred := globalActiveCred
	if err := preSignV4(req, cred.AccessKey, cred.SecretKey, time.Now().Add(10*time.Minute).Unix()); err != nil {
		t.Fatalf("Unable to inititalized new signed http request %s", err)
	}
	return req
}

func mustNewSignedShortMD5Request(method string, urlStr string, contentLength int64, body io.ReadSeeker, t *testing.T) *http.Request {
	req := mustNewRequest(method, urlStr, contentLength, body, t)
	req.Header.Set("Content-Md5", "invalid-digest")
	cred := globalActiveCred
	if err := signRequestV4(req, cred.AccessKey, cred.SecretKey); err != nil {
		t.Fatalf("Unable to initialized new signed http request %s", err)
	}
	return req
}

func mustNewSignedEmptyMD5Request(method string, urlStr string, contentLength int64, body io.ReadSeeker, t *testing.T) *http.Request {
	req := mustNewRequest(method, urlStr, contentLength, body, t)
	req.Header.Set("Content-Md5", "")
	cred := globalActiveCred
	if err := signRequestV4(req, cred.AccessKey, cred.SecretKey); err != nil {
		t.Fatalf("Unable to initialized new signed http request %s", err)
	}
	return req
}

func mustNewSignedBadMD5Request(method string, urlStr string, contentLength int64,
	body io.ReadSeeker, t *testing.T) *http.Request {
	req := mustNewRequest(method, urlStr, contentLength, body, t)
	req.Header.Set("Content-Md5", "YWFhYWFhYWFhYWFhYWFhCg==")
	cred := globalActiveCred
	if err := signRequestV4(req, cred.AccessKey, cred.SecretKey); err != nil {
		t.Fatalf("Unable to initialized new signed http request %s", err)
	}
	return req
}

// Tests is requested authenticated function, tests replies for s3 errors.
func TestIsReqAuthenticated(t *testing.T) {
	objLayer, fsDir, err := prepareFS()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fsDir)
	if err = newTestConfig(globalMinioDefaultRegion, objLayer); err != nil {
		t.Fatalf("unable initialize config file, %s", err)
	}

	creds, err := auth.CreateCredentials("myuser", "mypassword")
	if err != nil {
		t.Fatalf("unable create credential, %s", err)
	}

	globalActiveCred = creds

	// List of test cases for validating http request authentication.
	testCases := []struct {
		req     *http.Request
		s3Error APIErrorCode
	}{
		// When request is unsigned, access denied is returned.
		{mustNewRequest("GET", "http://127.0.0.1:9000", 0, nil, t), ErrAccessDenied},
		// Empty Content-Md5 header.
		{mustNewSignedEmptyMD5Request("PUT", "http://127.0.0.1:9000/", 5, bytes.NewReader([]byte("hello")), t), ErrInvalidDigest},
		// Short Content-Md5 header.
		{mustNewSignedShortMD5Request("PUT", "http://127.0.0.1:9000/", 5, bytes.NewReader([]byte("hello")), t), ErrInvalidDigest},
		// When request is properly signed, but has bad Content-MD5 header.
		{mustNewSignedBadMD5Request("PUT", "http://127.0.0.1:9000/", 5, bytes.NewReader([]byte("hello")), t), ErrBadDigest},
		// When request is properly signed, error is none.
		{mustNewSignedRequest("GET", "http://127.0.0.1:9000", 0, nil, t), ErrNone},
	}

	ctx := context.Background()
	// Validates all testcases.
	for i, testCase := range testCases {
		s3Error := isReqAuthenticated(ctx, testCase.req, globalServerRegion, serviceS3)
		if s3Error != testCase.s3Error {
			if _, err := ioutil.ReadAll(testCase.req.Body); toAPIErrorCode(ctx, err) != testCase.s3Error {
				t.Fatalf("Test %d: Unexpected S3 error: want %d - got %d (got after reading request %s)", i, testCase.s3Error, s3Error, toAPIError(ctx, err).Code)
			}
		}
	}
}
func TestCheckAdminRequestAuthType(t *testing.T) {
	objLayer, fsDir, err := prepareFS()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fsDir)

	if err = newTestConfig(globalMinioDefaultRegion, objLayer); err != nil {
		t.Fatalf("unable initialize config file, %s", err)
	}

	creds, err := auth.CreateCredentials("myuser", "mypassword")
	if err != nil {
		t.Fatalf("unable create credential, %s", err)
	}

	globalActiveCred = creds
	testCases := []struct {
		Request *http.Request
		ErrCode APIErrorCode
	}{
		{Request: mustNewRequest("GET", "http://127.0.0.1:9000", 0, nil, t), ErrCode: ErrAccessDenied},
		{Request: mustNewSignedRequest("GET", "http://127.0.0.1:9000", 0, nil, t), ErrCode: ErrNone},
		{Request: mustNewSignedV2Request("GET", "http://127.0.0.1:9000", 0, nil, t), ErrCode: ErrAccessDenied},
		{Request: mustNewPresignedV2Request("GET", "http://127.0.0.1:9000", 0, nil, t), ErrCode: ErrAccessDenied},
		{Request: mustNewPresignedRequest("GET", "http://127.0.0.1:9000", 0, nil, t), ErrCode: ErrAccessDenied},
	}
	ctx := context.Background()
	for i, testCase := range testCases {
		if _, s3Error := checkAdminRequestAuthType(ctx, testCase.Request, iampolicy.AllAdminActions, globalServerRegion); s3Error != testCase.ErrCode {
			t.Errorf("Test %d: Unexpected s3error returned wanted %d, got %d", i, testCase.ErrCode, s3Error)
		}
	}
}
