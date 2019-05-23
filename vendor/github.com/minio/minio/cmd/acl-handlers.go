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

package cmd

import (
	"encoding/xml"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/policy"
)

// Data types used for returning dummy access control
// policy XML, these variables shouldn't be used elsewhere
// they are only defined to be used in this file alone.
type grantee struct {
	XMLNS       string `xml:"xmlns:xsi,attr"`
	XMLXSI      string `xml:"xsi:type,attr"`
	Type        string `xml:"Type"`
	ID          string `xml:"ID,omitempty"`
	DisplayName string `xml:"DisplayName,omitempty"`
	URI         string `xml:"URI,omitempty"`
}

type grant struct {
	Grantee    grantee `xml:"Grantee"`
	Permission string  `xml:"Permission"`
}

type accessControlPolicy struct {
	XMLName           xml.Name `xml:"AccessControlPolicy"`
	Owner             Owner    `xml:"Owner"`
	AccessControlList struct {
		Grants []grant `xml:"Grant"`
	} `xml:"AccessControlList"`
}

// GetBucketACLHandler - GET Bucket ACL
// -----------------
// This operation uses the ACL
// subresource to return the ACL of a specified bucket.
func (api objectAPIHandlers) GetBucketACLHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "GetBucketACL")

	defer logger.AuditLog(w, r, "GetBucketACL", mustGetClaimsFromToken(r))

	vars := mux.Vars(r)
	bucket := vars["bucket"]

	objAPI := api.ObjectAPI()
	if objAPI == nil {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrServerNotInitialized), r.URL, guessIsBrowserReq(r))
		return
	}

	// Allow getBucketACL if policy action is set, since this is a dummy call
	// we are simply re-purposing the bucketPolicyAction.
	if s3Error := checkRequestAuthType(ctx, r, policy.GetBucketPolicyAction, bucket, ""); s3Error != ErrNone {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(s3Error), r.URL, guessIsBrowserReq(r))
		return
	}

	// Before proceeding validate if bucket exists.
	_, err := objAPI.GetBucketInfo(ctx, bucket)
	if err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	acl := &accessControlPolicy{}
	acl.AccessControlList.Grants = append(acl.AccessControlList.Grants, grant{
		Grantee: grantee{
			XMLNS:  "http://www.w3.org/2001/XMLSchema-instance",
			XMLXSI: "CanonicalUser",
			Type:   "CanonicalUser",
		},
		Permission: "FULL_CONTROL",
	})
	if err := xml.NewEncoder(w).Encode(acl); err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	w.(http.Flusher).Flush()
}

// GetObjectACLHandler - GET Object ACL
// -----------------
// This operation uses the ACL
// subresource to return the ACL of a specified object.
func (api objectAPIHandlers) GetObjectACLHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "GetObjectACL")

	defer logger.AuditLog(w, r, "GetObjectACL", mustGetClaimsFromToken(r))

	vars := mux.Vars(r)
	bucket := vars["bucket"]
	object := vars["object"]

	objAPI := api.ObjectAPI()
	if objAPI == nil {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrServerNotInitialized), r.URL, guessIsBrowserReq(r))
		return
	}

	// Allow getObjectACL if policy action is set, since this is a dummy call
	// we are simply re-purposing the bucketPolicyAction.
	if s3Error := checkRequestAuthType(ctx, r, policy.GetBucketPolicyAction, bucket, ""); s3Error != ErrNone {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(s3Error), r.URL, guessIsBrowserReq(r))
		return
	}

	// Before proceeding validate if object exists.
	_, err := objAPI.GetObjectInfo(ctx, bucket, object, ObjectOptions{})
	if err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	acl := &accessControlPolicy{}
	acl.AccessControlList.Grants = append(acl.AccessControlList.Grants, grant{
		Grantee: grantee{
			XMLNS:  "http://www.w3.org/2001/XMLSchema-instance",
			XMLXSI: "CanonicalUser",
			Type:   "CanonicalUser",
		},
		Permission: "FULL_CONTROL",
	})
	if err := xml.NewEncoder(w).Encode(acl); err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	w.(http.Flusher).Flush()
}
