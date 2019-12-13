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

package condition

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Key - conditional key which is used to fetch values for any condition.
// Refer https://docs.aws.amazon.com/IAM/latest/UserGuide/list_s3.html
// for more information about available condition keys.
type Key string

const (
	// S3XAmzCopySource - key representing x-amz-copy-source HTTP header applicable to PutObject API only.
	S3XAmzCopySource Key = "s3:x-amz-copy-source"

	// S3XAmzServerSideEncryption - key representing x-amz-server-side-encryption HTTP header applicable
	// to PutObject API only.
	S3XAmzServerSideEncryption Key = "s3:x-amz-server-side-encryption"

	// S3XAmzServerSideEncryptionCustomerAlgorithm - key representing
	// x-amz-server-side-encryption-customer-algorithm HTTP header applicable to PutObject API only.
	S3XAmzServerSideEncryptionCustomerAlgorithm Key = "s3:x-amz-server-side-encryption-customer-algorithm"

	// S3XAmzMetadataDirective - key representing x-amz-metadata-directive HTTP header applicable to
	// PutObject API only.
	S3XAmzMetadataDirective Key = "s3:x-amz-metadata-directive"

	// S3XAmzStorageClass - key representing x-amz-storage-class HTTP header applicable to PutObject API
	// only.
	S3XAmzStorageClass Key = "s3:x-amz-storage-class"

	// S3LocationConstraint - key representing LocationConstraint XML tag of CreateBucket API only.
	S3LocationConstraint Key = "s3:LocationConstraint"

	// S3Prefix - key representing prefix query parameter of ListBucket API only.
	S3Prefix Key = "s3:prefix"

	// S3Delimiter - key representing delimiter query parameter of ListBucket API only.
	S3Delimiter Key = "s3:delimiter"

	// S3MaxKeys - key representing max-keys query parameter of ListBucket API only.
	S3MaxKeys Key = "s3:max-keys"

	// AWSReferer - key representing Referer header of any API.
	AWSReferer Key = "aws:Referer"

	// AWSSourceIP - key representing client's IP address (not intermittent proxies) of any API.
	AWSSourceIP Key = "aws:SourceIp"

	// AWSUserAgent - key representing UserAgent header for any API.
	AWSUserAgent Key = "aws:UserAgent"

	// AWSSecureTransport - key representing if the clients request is authenticated or not.
	AWSSecureTransport Key = "aws:SecureTransport"

	// AWSCurrentTime - key representing the current time.
	AWSCurrentTime Key = "aws:CurrentTime"

	// AWSEpochTime - key representing the current epoch time.
	AWSEpochTime Key = "aws:EpochTime"

	// AWSPrincipalType - user principal type currently supported values are "User" and "Anonymous".
	AWSPrincipalType Key = "aws:principaltype"

	// AWSUserID - user unique ID, in MinIO this value is same as your user Access Key.
	AWSUserID Key = "aws:userid"

	// AWSUsername - user friendly name, in MinIO this value is same as your user Access Key.
	AWSUsername Key = "aws:username"

	// JWTSub - JWT subject claim substitution.
	JWTSub Key = "jwt:sub"

	// JWTIss issuer claim substitution.
	JWTIss Key = "jwt:iss"

	// JWTAud audience claim substitution.
	JWTAud Key = "jwt:aud"

	// JWTJti JWT unique identifier claim substitution.
	JWTJti Key = "jwt:jti"
)

// AllSupportedKeys - is list of all all supported keys.
var AllSupportedKeys = []Key{
	S3XAmzCopySource,
	S3XAmzServerSideEncryption,
	S3XAmzServerSideEncryptionCustomerAlgorithm,
	S3XAmzMetadataDirective,
	S3XAmzStorageClass,
	S3LocationConstraint,
	S3Prefix,
	S3Delimiter,
	S3MaxKeys,
	AWSReferer,
	AWSSourceIP,
	AWSUserAgent,
	AWSSecureTransport,
	AWSCurrentTime,
	AWSEpochTime,
	AWSPrincipalType,
	AWSUserID,
	AWSUsername,
	JWTSub,
	JWTIss,
	JWTAud,
	JWTJti,
	// Add new supported condition keys.
}

// CommonKeys - is list of all common condition keys.
var CommonKeys = []Key{
	AWSReferer,
	AWSSourceIP,
	AWSUserAgent,
	AWSSecureTransport,
	AWSCurrentTime,
	AWSEpochTime,
	AWSPrincipalType,
	AWSUserID,
	AWSUsername,
	JWTSub,
	JWTIss,
	JWTAud,
	JWTJti,
}

func substFuncFromValues(values map[string][]string) func(string) string {
	return func(v string) string {
		for _, key := range CommonKeys {
			// Empty values are not supported for policy variables.
			if rvalues, ok := values[key.Name()]; ok && rvalues[0] != "" {
				v = strings.Replace(v, key.VarName(), rvalues[0], -1)
			}
		}
		return v
	}
}

// IsValid - checks if key is valid or not.
func (key Key) IsValid() bool {
	for _, supKey := range AllSupportedKeys {
		if supKey == key {
			return true
		}
	}

	return false
}

// MarshalJSON - encodes Key to JSON data.
func (key Key) MarshalJSON() ([]byte, error) {
	if !key.IsValid() {
		return nil, fmt.Errorf("unknown key %v", key)
	}

	return json.Marshal(string(key))
}

// VarName - returns variable key name, such as "${aws:username}"
func (key Key) VarName() string {
	return fmt.Sprintf("${%s}", key)
}

// Name - returns key name which is stripped value of prefixes "aws:" and "s3:"
func (key Key) Name() string {
	keyString := string(key)

	if strings.HasPrefix(keyString, "aws:") {
		return strings.TrimPrefix(keyString, "aws:")
	} else if strings.HasPrefix(keyString, "jwt:") {
		return strings.TrimPrefix(keyString, "jwt:")
	}
	return strings.TrimPrefix(keyString, "s3:")
}

// UnmarshalJSON - decodes JSON data to Key.
func (key *Key) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	parsedKey, err := parseKey(s)
	if err != nil {
		return err
	}

	*key = parsedKey
	return nil
}

func parseKey(s string) (Key, error) {
	key := Key(s)

	if key.IsValid() {
		return key, nil
	}

	return key, fmt.Errorf("invalid condition key '%v'", s)
}

// KeySet - set representation of slice of keys.
type KeySet map[Key]struct{}

// Add - add a key to key set.
func (set KeySet) Add(key Key) {
	set[key] = struct{}{}
}

// Difference - returns a key set contains difference of two keys.
// Example:
//     keySet1 := ["one", "two", "three"]
//     keySet2 := ["two", "four", "three"]
//     keySet1.Difference(keySet2) == ["one"]
func (set KeySet) Difference(sset KeySet) KeySet {
	nset := make(KeySet)

	for k := range set {
		if _, ok := sset[k]; !ok {
			nset.Add(k)
		}
	}

	return nset
}

// IsEmpty - returns whether key set is empty or not.
func (set KeySet) IsEmpty() bool {
	return len(set) == 0
}

func (set KeySet) String() string {
	return fmt.Sprintf("%v", set.ToSlice())
}

// ToSlice - returns slice of keys.
func (set KeySet) ToSlice() []Key {
	keys := []Key{}

	for key := range set {
		keys = append(keys, key)
	}

	return keys
}

// NewKeySet - returns new KeySet contains given keys.
func NewKeySet(keys ...Key) KeySet {
	set := make(KeySet)
	for _, key := range keys {
		set.Add(key)
	}

	return set
}

// AllSupportedAdminKeys - is list of all admin supported keys.
var AllSupportedAdminKeys = []Key{
	AWSReferer,
	AWSSourceIP,
	AWSUserAgent,
	AWSSecureTransport,
	AWSCurrentTime,
	AWSEpochTime,
	// Add new supported condition keys.
}
