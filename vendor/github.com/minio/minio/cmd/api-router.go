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
	"net/http"

	"github.com/gorilla/mux"
)

// objectAPIHandler implements and provides http handlers for S3 API.
type objectAPIHandlers struct {
	ObjectAPI func() ObjectLayer
	CacheAPI  func() CacheObjectLayer
	// Returns true of handlers should interpret encryption.
	EncryptionEnabled func() bool
}

// registerAPIRouter - registers S3 compatible APIs.
func registerAPIRouter(router *mux.Router, encryptionEnabled bool) {
	// Initialize API.
	api := objectAPIHandlers{
		ObjectAPI: newObjectLayerFn,
		CacheAPI:  newCacheObjectsFn,
		EncryptionEnabled: func() bool {
			return encryptionEnabled
		},
	}

	// API Router
	apiRouter := router.PathPrefix("/").Subrouter()
	var routers []*mux.Router
	for _, domainName := range globalDomainNames {
		routers = append(routers, apiRouter.Host("{bucket:.+}."+domainName).Subrouter())
		routers = append(routers, apiRouter.Host("{bucket:.+}."+domainName+":{port:.*}").Subrouter())
	}
	routers = append(routers, apiRouter.PathPrefix("/{bucket}").Subrouter())

	for _, bucket := range routers {
		// Object operations
		// HeadObject
		bucket.Methods("HEAD").Path("/{object:.+}").HandlerFunc(httpTraceAll(api.HeadObjectHandler))
		// CopyObjectPart
		bucket.Methods("PUT").Path("/{object:.+}").HeadersRegexp("X-Amz-Copy-Source", ".*?(\\/|%2F).*?").HandlerFunc(httpTraceAll(api.CopyObjectPartHandler)).Queries("partNumber", "{partNumber:[0-9]+}", "uploadId", "{uploadId:.*}")
		// PutObjectPart
		bucket.Methods("PUT").Path("/{object:.+}").HandlerFunc(httpTraceHdrs(api.PutObjectPartHandler)).Queries("partNumber", "{partNumber:[0-9]+}", "uploadId", "{uploadId:.*}")
		// ListObjectPxarts
		bucket.Methods("GET").Path("/{object:.+}").HandlerFunc(httpTraceAll(api.ListObjectPartsHandler)).Queries("uploadId", "{uploadId:.*}")
		// CompleteMultipartUpload
		bucket.Methods("POST").Path("/{object:.+}").HandlerFunc(httpTraceAll(api.CompleteMultipartUploadHandler)).Queries("uploadId", "{uploadId:.*}")
		// NewMultipartUpload
		bucket.Methods("POST").Path("/{object:.+}").HandlerFunc(httpTraceAll(api.NewMultipartUploadHandler)).Queries("uploads", "")
		// AbortMultipartUpload
		bucket.Methods("DELETE").Path("/{object:.+}").HandlerFunc(httpTraceAll(api.AbortMultipartUploadHandler)).Queries("uploadId", "{uploadId:.*}")
		// GetObjectACL - this is a dummy call.
		bucket.Methods("GET").Path("/{object:.+}").HandlerFunc(httpTraceHdrs(api.GetObjectACLHandler)).Queries("acl", "")
		// GetObjectTagging - this is a dummy call.
		bucket.Methods("GET").Path("/{object:.+}").HandlerFunc(httpTraceHdrs(api.GetObjectTaggingHandler)).Queries("tagging", "")
		// SelectObjectContent
		bucket.Methods("POST").Path("/{object:.+}").HandlerFunc(httpTraceHdrs(api.SelectObjectContentHandler)).Queries("select", "").Queries("select-type", "2")
		// GetObject
		bucket.Methods("GET").Path("/{object:.+}").HandlerFunc(httpTraceHdrs(api.GetObjectHandler))
		// CopyObject
		bucket.Methods("PUT").Path("/{object:.+}").HeadersRegexp("X-Amz-Copy-Source", ".*?(\\/|%2F).*?").HandlerFunc(httpTraceAll(api.CopyObjectHandler))
		// PutObject
		bucket.Methods("PUT").Path("/{object:.+}").HandlerFunc(httpTraceHdrs(api.PutObjectHandler))
		// DeleteObject
		bucket.Methods("DELETE").Path("/{object:.+}").HandlerFunc(httpTraceAll(api.DeleteObjectHandler))

		/// Bucket operations
		// GetBucketLocation
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.GetBucketLocationHandler)).Queries("location", "")
		// GetBucketPolicy
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.GetBucketPolicyHandler)).Queries("policy", "")

		// Dummy Bucket Calls
		// GetBucketACL -- this is a dummy call.
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.GetBucketACLHandler)).Queries("acl", "")
		// GetBucketCors - this is a dummy call.
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.GetBucketCorsHandler)).Queries("cors", "")
		// GetBucketWebsiteHandler - this is a dummy call.
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.GetBucketWebsiteHandler)).Queries("website", "")
		// GetBucketVersioningHandler - this is a dummy call.
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.GetBucketVersioningHandler)).Queries("versioning", "")
		// GetBucketAccelerateHandler - this is a dummy call.
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.GetBucketAccelerateHandler)).Queries("accelerate", "")
		// GetBucketRequestPaymentHandler - this is a dummy call.
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.GetBucketRequestPaymentHandler)).Queries("requestPayment", "")
		// GetBucketLoggingHandler - this is a dummy call.
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.GetBucketLoggingHandler)).Queries("logging", "")
		// GetBucketLifecycleHandler - this is a dummy call.
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.GetBucketLifecycleHandler)).Queries("lifecycle", "")
		// GetBucketReplicationHandler - this is a dummy call.
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.GetBucketReplicationHandler)).Queries("replication", "")
		// GetBucketTaggingHandler - this is a dummy call.
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.GetBucketTaggingHandler)).Queries("tagging", "")
		//DeleteBucketWebsiteHandler
		bucket.Methods("DELETE").HandlerFunc(httpTraceAll(api.DeleteBucketWebsiteHandler)).Queries("website", "")
		// DeleteBucketTaggingHandler
		bucket.Methods("DELETE").HandlerFunc(httpTraceAll(api.DeleteBucketTaggingHandler)).Queries("tagging", "")

		// GetBucketNotification
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.GetBucketNotificationHandler)).Queries("notification", "")
		// ListenBucketNotification
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.ListenBucketNotificationHandler)).Queries("events", "{events:.*}")
		// ListMultipartUploads
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.ListMultipartUploadsHandler)).Queries("uploads", "")
		// ListObjectsV2
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.ListObjectsV2Handler)).Queries("list-type", "2")
		// ListObjectsV1 (Legacy)
		bucket.Methods("GET").HandlerFunc(httpTraceAll(api.ListObjectsV1Handler))
		// PutBucketPolicy
		bucket.Methods("PUT").HandlerFunc(httpTraceAll(api.PutBucketPolicyHandler)).Queries("policy", "")
		// PutBucketNotification
		bucket.Methods("PUT").HandlerFunc(httpTraceAll(api.PutBucketNotificationHandler)).Queries("notification", "")
		// PutBucket
		bucket.Methods("PUT").HandlerFunc(httpTraceAll(api.PutBucketHandler))
		// HeadBucket
		bucket.Methods("HEAD").HandlerFunc(httpTraceAll(api.HeadBucketHandler))
		// PostPolicy
		bucket.Methods("POST").HeadersRegexp("Content-Type", "multipart/form-data*").HandlerFunc(httpTraceHdrs(api.PostPolicyBucketHandler))
		// DeleteMultipleObjects
		bucket.Methods("POST").HandlerFunc(httpTraceAll(api.DeleteMultipleObjectsHandler)).Queries("delete", "")
		// DeleteBucketPolicy
		bucket.Methods("DELETE").HandlerFunc(httpTraceAll(api.DeleteBucketPolicyHandler)).Queries("policy", "")
		// DeleteBucket
		bucket.Methods("DELETE").HandlerFunc(httpTraceAll(api.DeleteBucketHandler))
	}

	/// Root operation

	// ListBuckets
	apiRouter.Methods("GET").Path("/").HandlerFunc(httpTraceAll(api.ListBucketsHandler))

	// If none of the routes match.
	apiRouter.NotFoundHandler = http.HandlerFunc(httpTraceAll(notFoundHandler))
}
