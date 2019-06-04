/*
 * MinIO Cloud Storage, (C) 2016 MinIO, Inc.
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
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	minio "github.com/minio/minio-go/v6"
)

// Test Post Policy parsing and checking conditions
func TestPostPolicyForm(t *testing.T) {
	pp := minio.NewPostPolicy()
	pp.SetBucket("testbucket")
	pp.SetContentType("image/jpeg")
	pp.SetUserMetadata("uuid", "14365123651274")
	pp.SetKeyStartsWith("user/user1/filename")
	pp.SetContentLengthRange(1048579, 10485760)
	pp.SetSuccessStatusAction("201")

	type testCase struct {
		Bucket              string
		Key                 string
		XAmzDate            string
		XAmzAlgorithm       string
		XAmzCredential      string
		XAmzMetaUUID        string
		ContentType         string
		SuccessActionStatus string
		Policy              string
		Expired             bool
		expectedErr         error
	}

	testCases := []testCase{
		// Everything is fine with this test
		{Bucket: "testbucket", Key: "user/user1/filename/${filename}/myfile.txt", XAmzMetaUUID: "14365123651274", SuccessActionStatus: "201", XAmzCredential: "KVGKMDUQ23TCZXTLTHLP/20160727/us-east-1/s3/aws4_request", XAmzDate: "20160727T000000Z", XAmzAlgorithm: "AWS4-HMAC-SHA256", ContentType: "image/jpeg", expectedErr: nil},
		// Expired policy document
		{Bucket: "testbucket", Key: "user/user1/filename/${filename}/myfile.txt", XAmzMetaUUID: "14365123651274", SuccessActionStatus: "201", XAmzCredential: "KVGKMDUQ23TCZXTLTHLP/20160727/us-east-1/s3/aws4_request", XAmzDate: "20160727T000000Z", XAmzAlgorithm: "AWS4-HMAC-SHA256", ContentType: "image/jpeg", Expired: true, expectedErr: fmt.Errorf("Invalid according to Policy: Policy expired")},
		// Different AMZ date
		{Bucket: "testbucket", Key: "user/user1/filename/${filename}/myfile.txt", XAmzMetaUUID: "14365123651274", XAmzDate: "2017T000000Z", XAmzAlgorithm: "AWS4-HMAC-SHA256", ContentType: "image/jpeg", expectedErr: fmt.Errorf("Invalid according to Policy: Policy Condition failed")},
		// Key which doesn't start with user/user1/filename
		{Bucket: "testbucket", Key: "myfile.txt", XAmzDate: "20160727T000000Z", XAmzMetaUUID: "14365123651274", XAmzAlgorithm: "AWS4-HMAC-SHA256", ContentType: "image/jpeg", expectedErr: fmt.Errorf("Invalid according to Policy: Policy Condition failed")},
		// Incorrect bucket name.
		{Bucket: "incorrect", Key: "user/user1/filename/myfile.txt", XAmzMetaUUID: "14365123651274", XAmzDate: "20160727T000000Z", XAmzAlgorithm: "AWS4-HMAC-SHA256", ContentType: "image/jpeg", expectedErr: fmt.Errorf("Invalid according to Policy: Policy Condition failed")},
		// Incorrect key name
		{Bucket: "testbucket", Key: "incorrect", XAmzDate: "20160727T000000Z", XAmzMetaUUID: "14365123651274", XAmzAlgorithm: "AWS4-HMAC-SHA256", ContentType: "image/jpeg", expectedErr: fmt.Errorf("Invalid according to Policy: Policy Condition failed")},
		// Incorrect date
		{Bucket: "testbucket", Key: "user/user1/filename/${filename}/myfile.txt", XAmzMetaUUID: "14365123651274", XAmzDate: "incorrect", XAmzAlgorithm: "AWS4-HMAC-SHA256", ContentType: "image/jpeg", expectedErr: fmt.Errorf("Invalid according to Policy: Policy Condition failed")},
		// Incorrect ContentType
		{Bucket: "testbucket", Key: "user/user1/filename/${filename}/myfile.txt", XAmzMetaUUID: "14365123651274", XAmzDate: "20160727T000000Z", XAmzAlgorithm: "AWS4-HMAC-SHA256", ContentType: "incorrect", expectedErr: fmt.Errorf("Invalid according to Policy: Policy Condition failed")},
		// Incorrect Metadata
		{Bucket: "testbucket", Key: "user/user1/filename/${filename}/myfile.txt", XAmzMetaUUID: "151274", SuccessActionStatus: "201", XAmzCredential: "KVGKMDUQ23TCZXTLTHLP/20160727/us-east-1/s3/aws4_request", XAmzDate: "20160727T000000Z", XAmzAlgorithm: "AWS4-HMAC-SHA256", ContentType: "image/jpeg", expectedErr: fmt.Errorf("Invalid according to Policy: Policy Condition failed: [eq, $x-amz-meta-uuid, 14365123651274]")},
	}
	// Validate all the test cases.
	for i, tt := range testCases {
		formValues := make(http.Header)
		formValues.Set("Bucket", tt.Bucket)
		formValues.Set("Key", tt.Key)
		formValues.Set("Content-Type", tt.ContentType)
		formValues.Set("X-Amz-Date", tt.XAmzDate)
		formValues.Set("X-Amz-Meta-Uuid", tt.XAmzMetaUUID)
		formValues.Set("X-Amz-Algorithm", tt.XAmzAlgorithm)
		formValues.Set("X-Amz-Credential", tt.XAmzCredential)
		if tt.Expired {
			// Expired already.
			pp.SetExpires(UTCNow().AddDate(0, 0, -10))
		} else {
			// Expires in 10 days.
			pp.SetExpires(UTCNow().AddDate(0, 0, 10))
		}

		formValues.Set("Policy", base64.StdEncoding.EncodeToString([]byte(pp.String())))
		formValues.Set("Success_action_status", tt.SuccessActionStatus)
		policyBytes, err := base64.StdEncoding.DecodeString(base64.StdEncoding.EncodeToString([]byte(pp.String())))
		if err != nil {
			t.Fatal(err)
		}

		postPolicyForm, err := parsePostPolicyForm(string(policyBytes))
		if err != nil {
			t.Fatal(err)
		}

		err = checkPostPolicy(formValues, postPolicyForm)
		if err != nil && tt.expectedErr != nil && err.Error() != tt.expectedErr.Error() {
			t.Fatalf("Test %d:, Expected %s, got %s", i+1, tt.expectedErr.Error(), err.Error())
		}
	}
}
