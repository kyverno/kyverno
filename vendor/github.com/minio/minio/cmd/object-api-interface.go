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
	"context"
	"io"
	"net/http"

	"github.com/minio/minio-go/v6/pkg/encrypt"
	"github.com/minio/minio/pkg/lifecycle"
	"github.com/minio/minio/pkg/madmin"
	"github.com/minio/minio/pkg/policy"
)

// CheckCopyPreconditionFn returns true if copy precondition check failed.
type CheckCopyPreconditionFn func(o ObjectInfo, encETag string) bool

// GetObjectInfoFn is the signature of GetObjectInfo function.
type GetObjectInfoFn func(ctx context.Context, bucket, object string, opts ObjectOptions) (objInfo ObjectInfo, err error)

// ObjectOptions represents object options for ObjectLayer operations
type ObjectOptions struct {
	ServerSideEncryption encrypt.ServerSide
	UserDefined          map[string]string
	CheckCopyPrecondFn   CheckCopyPreconditionFn
}

// LockType represents required locking for ObjectLayer operations
type LockType int

const (
	noLock LockType = iota
	readLock
	writeLock
)

// ObjectLayer implements primitives for object API layer.
type ObjectLayer interface {
	// Locking operations on object.
	NewNSLock(ctx context.Context, bucket string, object string) RWLocker

	// Storage operations.
	Shutdown(context.Context) error
	StorageInfo(context.Context) StorageInfo

	// Bucket operations.
	MakeBucketWithLocation(ctx context.Context, bucket string, location string) error
	GetBucketInfo(ctx context.Context, bucket string) (bucketInfo BucketInfo, err error)
	ListBuckets(ctx context.Context) (buckets []BucketInfo, err error)
	DeleteBucket(ctx context.Context, bucket string) error
	ListObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result ListObjectsInfo, err error)
	ListObjectsV2(ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result ListObjectsV2Info, err error)

	// Object operations.

	// GetObjectNInfo returns a GetObjectReader that satisfies the
	// ReadCloser interface. The Close method unlocks the object
	// after reading, so it must always be called after usage.
	//
	// IMPORTANTLY, when implementations return err != nil, this
	// function MUST NOT return a non-nil ReadCloser.
	GetObjectNInfo(ctx context.Context, bucket, object string, rs *HTTPRangeSpec, h http.Header, lockType LockType, opts ObjectOptions) (reader *GetObjectReader, err error)
	GetObject(ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string, opts ObjectOptions) (err error)
	GetObjectInfo(ctx context.Context, bucket, object string, opts ObjectOptions) (objInfo ObjectInfo, err error)
	PutObject(ctx context.Context, bucket, object string, data *PutObjReader, opts ObjectOptions) (objInfo ObjectInfo, err error)
	CopyObject(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo ObjectInfo, srcOpts, dstOpts ObjectOptions) (objInfo ObjectInfo, err error)
	DeleteObject(ctx context.Context, bucket, object string) error
	DeleteObjects(ctx context.Context, bucket string, objects []string) ([]error, error)

	// Multipart operations.
	ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker, delimiter string, maxUploads int) (result ListMultipartsInfo, err error)
	NewMultipartUpload(ctx context.Context, bucket, object string, opts ObjectOptions) (uploadID string, err error)
	CopyObjectPart(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, uploadID string, partID int,
		startOffset int64, length int64, srcInfo ObjectInfo, srcOpts, dstOpts ObjectOptions) (info PartInfo, err error)
	PutObjectPart(ctx context.Context, bucket, object, uploadID string, partID int, data *PutObjReader, opts ObjectOptions) (info PartInfo, err error)
	ListObjectParts(ctx context.Context, bucket, object, uploadID string, partNumberMarker int, maxParts int, opts ObjectOptions) (result ListPartsInfo, err error)
	AbortMultipartUpload(ctx context.Context, bucket, object, uploadID string) error
	CompleteMultipartUpload(ctx context.Context, bucket, object, uploadID string, uploadedParts []CompletePart, opts ObjectOptions) (objInfo ObjectInfo, err error)

	// Healing operations.
	ReloadFormat(ctx context.Context, dryRun bool) error
	HealFormat(ctx context.Context, dryRun bool) (madmin.HealResultItem, error)
	HealBucket(ctx context.Context, bucket string, dryRun, remove bool) (madmin.HealResultItem, error)
	HealObject(ctx context.Context, bucket, object string, dryRun, remove bool, scanMode madmin.HealScanMode) (madmin.HealResultItem, error)
	HealObjects(ctx context.Context, bucket, prefix string, healObjectFn func(string, string) error) error

	ListBucketsHeal(ctx context.Context) (buckets []BucketInfo, err error)
	ListObjectsHeal(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result ListObjectsInfo, err error)

	// Policy operations
	SetBucketPolicy(context.Context, string, *policy.Policy) error
	GetBucketPolicy(context.Context, string) (*policy.Policy, error)
	DeleteBucketPolicy(context.Context, string) error

	// Supported operations check
	IsNotificationSupported() bool
	IsListenBucketSupported() bool
	IsEncryptionSupported() bool

	// Compression support check.
	IsCompressionSupported() bool

	// Lifecycle operations
	SetBucketLifecycle(context.Context, string, *lifecycle.Lifecycle) error
	GetBucketLifecycle(context.Context, string) (*lifecycle.Lifecycle, error)
	DeleteBucketLifecycle(context.Context, string) error

	// Backend related metrics
	GetMetrics(ctx context.Context) (*Metrics, error)
}
