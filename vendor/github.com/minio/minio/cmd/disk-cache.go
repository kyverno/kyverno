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
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/djherbis/atime"

	"github.com/minio/minio/cmd/crypto"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/hash"
	"github.com/minio/minio/pkg/wildcard"
)

const (
	// disk cache needs to have  object size space free for a cache entry to be created.
	cacheTrashDir        = "trash"
	cacheCleanupInterval = 10 // in minutes
)

// abstract slice of cache drives backed by FS.
type diskCache struct {
	cfs []*cacheFSObjects
}

// Abstracts disk caching - used by the S3 layer
type cacheObjects struct {
	// pointer to disk cache
	cache *diskCache
	// ListObjects pool management.
	listPool *TreeWalkPool
	// file path patterns to exclude from cache
	exclude []string
	// Object functions pointing to the corresponding functions of backend implementation.
	GetObjectNInfoFn          func(ctx context.Context, bucket, object string, rs *HTTPRangeSpec, h http.Header, lockType LockType, opts ObjectOptions) (gr *GetObjectReader, err error)
	GetObjectFn               func(ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string, opts ObjectOptions) (err error)
	GetObjectInfoFn           func(ctx context.Context, bucket, object string, opts ObjectOptions) (objInfo ObjectInfo, err error)
	PutObjectFn               func(ctx context.Context, bucket, object string, data *PutObjReader, opts ObjectOptions) (objInfo ObjectInfo, err error)
	DeleteObjectFn            func(ctx context.Context, bucket, object string) error
	DeleteObjectsFn           func(ctx context.Context, bucket string, objects []string) ([]error, error)
	ListObjectsFn             func(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result ListObjectsInfo, err error)
	ListObjectsV2Fn           func(ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result ListObjectsV2Info, err error)
	ListBucketsFn             func(ctx context.Context) (buckets []BucketInfo, err error)
	GetBucketInfoFn           func(ctx context.Context, bucket string) (bucketInfo BucketInfo, err error)
	NewMultipartUploadFn      func(ctx context.Context, bucket, object string, opts ObjectOptions) (uploadID string, err error)
	PutObjectPartFn           func(ctx context.Context, bucket, object, uploadID string, partID int, data *PutObjReader, opts ObjectOptions) (info PartInfo, err error)
	AbortMultipartUploadFn    func(ctx context.Context, bucket, object, uploadID string) error
	CompleteMultipartUploadFn func(ctx context.Context, bucket, object, uploadID string, uploadedParts []CompletePart, opts ObjectOptions) (objInfo ObjectInfo, err error)
	DeleteBucketFn            func(ctx context.Context, bucket string) error
}

// CacheStorageInfo - represents total, free capacity of
// underlying cache storage.
type CacheStorageInfo struct {
	Total uint64 // Total cache disk space.
	Free  uint64 // Free cache available space.
}

// CacheObjectLayer implements primitives for cache object API layer.
type CacheObjectLayer interface {
	// Bucket operations.
	ListObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result ListObjectsInfo, err error)
	ListObjectsV2(ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result ListObjectsV2Info, err error)
	GetBucketInfo(ctx context.Context, bucket string) (bucketInfo BucketInfo, err error)
	ListBuckets(ctx context.Context) (buckets []BucketInfo, err error)
	DeleteBucket(ctx context.Context, bucket string) error
	// Object operations.
	GetObjectNInfo(ctx context.Context, bucket, object string, rs *HTTPRangeSpec, h http.Header, lockType LockType, opts ObjectOptions) (gr *GetObjectReader, err error)
	GetObject(ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string, opts ObjectOptions) (err error)
	GetObjectInfo(ctx context.Context, bucket, object string, opts ObjectOptions) (objInfo ObjectInfo, err error)
	PutObject(ctx context.Context, bucket, object string, data *PutObjReader, opts ObjectOptions) (objInfo ObjectInfo, err error)
	DeleteObject(ctx context.Context, bucket, object string) error
	DeleteObjects(ctx context.Context, bucket string, objects []string) ([]error, error)

	// Multipart operations.
	NewMultipartUpload(ctx context.Context, bucket, object string, opts ObjectOptions) (uploadID string, err error)
	PutObjectPart(ctx context.Context, bucket, object, uploadID string, partID int, data *PutObjReader, opts ObjectOptions) (info PartInfo, err error)
	AbortMultipartUpload(ctx context.Context, bucket, object, uploadID string) error
	CompleteMultipartUpload(ctx context.Context, bucket, object, uploadID string, uploadedParts []CompletePart, opts ObjectOptions) (objInfo ObjectInfo, err error)

	// Storage operations.
	StorageInfo(ctx context.Context) CacheStorageInfo
}

// IsCacheable returns if the object should be saved in the cache.
func (o ObjectInfo) IsCacheable() bool {
	return !crypto.IsEncrypted(o.UserDefined)
}

// backendDownError returns true if err is due to backend failure or faulty disk if in server mode
func backendDownError(err error) bool {
	_, backendDown := err.(BackendDown)
	return backendDown || IsErr(err, baseErrs...)
}

// get cache disk where object is currently cached for a GET operation. If object does not exist at that location,
// treat the list of cache drives as a circular buffer and walk through them starting at hash index
// until an online drive is found.If object is not found, fall back to the first online cache drive
// closest to the hash index, so that object can be recached.
func (c diskCache) getCachedFSLoc(ctx context.Context, bucket, object string) (*cacheFSObjects, error) {
	index := c.hashIndex(bucket, object)
	numDisks := len(c.cfs)
	// save first online cache disk closest to the hint index
	var firstOnlineDisk *cacheFSObjects
	for k := 0; k < numDisks; k++ {
		i := (index + k) % numDisks
		if c.cfs[i] == nil {
			continue
		}
		if c.cfs[i].IsOnline() {
			if firstOnlineDisk == nil {
				firstOnlineDisk = c.cfs[i]
			}
			if c.cfs[i].Exists(ctx, bucket, object) {
				return c.cfs[i], nil
			}
		}
	}

	if firstOnlineDisk != nil {
		return firstOnlineDisk, nil
	}
	return nil, errDiskNotFound
}

// choose a cache deterministically based on hash of bucket,object. The hash index is treated as
// a hint. In the event that the cache drive at hash index is offline, treat the list of cache drives
// as a circular buffer and walk through them starting at hash index until an online drive is found.
func (c diskCache) getCacheFS(ctx context.Context, bucket, object string) (*cacheFSObjects, error) {
	index := c.hashIndex(bucket, object)
	numDisks := len(c.cfs)
	for k := 0; k < numDisks; k++ {
		i := (index + k) % numDisks
		if c.cfs[i] == nil {
			continue
		}
		if c.cfs[i].IsOnline() {
			return c.cfs[i], nil
		}
	}
	return nil, errDiskNotFound
}

// Compute a unique hash sum for bucket and object
func (c diskCache) hashIndex(bucket, object string) int {
	return crcHashMod(pathJoin(bucket, object), len(c.cfs))
}

// construct a metadata k-v map
func (c cacheObjects) getMetadata(objInfo ObjectInfo) map[string]string {
	metadata := make(map[string]string)
	metadata["etag"] = objInfo.ETag
	metadata["content-type"] = objInfo.ContentType
	metadata["content-encoding"] = objInfo.ContentEncoding

	for key, val := range objInfo.UserDefined {
		metadata[key] = val
	}
	return metadata
}

func (c cacheObjects) GetObjectNInfo(ctx context.Context, bucket, object string, rs *HTTPRangeSpec, h http.Header, lockType LockType, opts ObjectOptions) (gr *GetObjectReader, err error) {
	if c.isCacheExclude(bucket, object) {
		return c.GetObjectNInfoFn(ctx, bucket, object, rs, h, lockType, opts)
	}

	// fetch cacheFSObjects if object is currently cached or nearest available cache drive
	dcache, err := c.cache.getCachedFSLoc(ctx, bucket, object)
	if err != nil {
		return c.GetObjectNInfoFn(ctx, bucket, object, rs, h, lockType, opts)
	}

	cacheReader, cacheErr := dcache.GetObjectNInfo(ctx, bucket, object, rs, h, lockType, opts)

	objInfo, err := c.GetObjectInfoFn(ctx, bucket, object, opts)
	if backendDownError(err) && cacheErr == nil {
		return cacheReader, nil
	} else if err != nil {
		if _, ok := err.(ObjectNotFound); ok {
			if cacheErr == nil {
				cacheReader.Close()
				// Delete cached entry if backend object
				// was deleted.
				dcache.Delete(ctx, bucket, object)
			}
		}
		return nil, err
	}

	if !objInfo.IsCacheable() || filterFromCache(objInfo.UserDefined) {
		return c.GetObjectNInfoFn(ctx, bucket, object, rs, h, lockType, opts)
	}

	if cacheErr == nil {
		if cacheReader.ObjInfo.ETag == objInfo.ETag && !isStaleCache(objInfo) {
			// Object is not stale, so serve from cache
			return cacheReader, nil
		}
		cacheReader.Close()
		// Object is stale, so delete from cache
		dcache.Delete(ctx, bucket, object)
	}

	// Since we got here, we are serving the request from backend,
	// and also adding the object to the cache.

	if rs != nil {
		// We don't cache partial objects.
		return c.GetObjectNInfoFn(ctx, bucket, object, rs, h, lockType, opts)
	}
	if !dcache.diskAvailable(objInfo.Size) {
		return c.GetObjectNInfoFn(ctx, bucket, object, rs, h, lockType, opts)
	}

	bkReader, bkErr := c.GetObjectNInfoFn(ctx, bucket, object, rs, h, lockType, opts)
	if bkErr != nil {
		return nil, bkErr
	}

	// Initialize pipe.
	pipeReader, pipeWriter := io.Pipe()
	teeReader := io.TeeReader(bkReader, pipeWriter)
	hashReader, herr := hash.NewReader(pipeReader, bkReader.ObjInfo.Size, "", "", bkReader.ObjInfo.Size, globalCLIContext.StrictS3Compat)
	if herr != nil {
		bkReader.Close()
		return nil, herr
	}

	go func() {
		putErr := dcache.Put(ctx, bucket, object, NewPutObjReader(hashReader, nil, nil), ObjectOptions{UserDefined: c.getMetadata(bkReader.ObjInfo)})
		// close the write end of the pipe, so the error gets
		// propagated to getObjReader
		pipeWriter.CloseWithError(putErr)
	}()

	cleanupBackend := func() { bkReader.Close() }
	cleanupPipe := func() { pipeReader.Close() }
	return NewGetObjectReaderFromReader(teeReader, bkReader.ObjInfo, opts.CheckCopyPrecondFn, cleanupBackend, cleanupPipe)
}

// Uses cached-object to serve the request. If object is not cached it serves the request from the backend and also
// stores it in the cache for serving subsequent requests.
func (c cacheObjects) GetObject(ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string, opts ObjectOptions) (err error) {
	GetObjectFn := c.GetObjectFn
	GetObjectInfoFn := c.GetObjectInfoFn

	if c.isCacheExclude(bucket, object) {
		return GetObjectFn(ctx, bucket, object, startOffset, length, writer, etag, opts)
	}
	// fetch cacheFSObjects if object is currently cached or nearest available cache drive
	dcache, err := c.cache.getCachedFSLoc(ctx, bucket, object)
	if err != nil {
		return GetObjectFn(ctx, bucket, object, startOffset, length, writer, etag, opts)
	}
	// stat object on backend
	objInfo, err := GetObjectInfoFn(ctx, bucket, object, opts)
	backendDown := backendDownError(err)
	if err != nil && !backendDown {
		if _, ok := err.(ObjectNotFound); ok {
			// Delete the cached entry if backend object was deleted.
			dcache.Delete(ctx, bucket, object)
		}
		return err
	}

	if !backendDown && !objInfo.IsCacheable() {
		return GetObjectFn(ctx, bucket, object, startOffset, length, writer, etag, opts)
	}

	if !backendDown && filterFromCache(objInfo.UserDefined) {
		return GetObjectFn(ctx, bucket, object, startOffset, length, writer, etag, opts)
	}

	cachedObjInfo, err := dcache.GetObjectInfo(ctx, bucket, object, opts)
	if err == nil {
		if backendDown {
			// If the backend is down, serve the request from cache.
			return dcache.Get(ctx, bucket, object, startOffset, length, writer, etag, opts)
		}
		if cachedObjInfo.ETag == objInfo.ETag && !isStaleCache(objInfo) {
			return dcache.Get(ctx, bucket, object, startOffset, length, writer, etag, opts)
		}
		dcache.Delete(ctx, bucket, object)
	}
	if startOffset != 0 || (length > 0 && length != objInfo.Size) {
		// We don't cache partial objects.
		return GetObjectFn(ctx, bucket, object, startOffset, length, writer, etag, opts)
	}
	if !dcache.diskAvailable(objInfo.Size) {
		return GetObjectFn(ctx, bucket, object, startOffset, length, writer, etag, opts)
	}
	// Initialize pipe.
	pipeReader, pipeWriter := io.Pipe()
	hashReader, err := hash.NewReader(pipeReader, objInfo.Size, "", "", objInfo.Size, globalCLIContext.StrictS3Compat)
	if err != nil {
		return err
	}
	go func() {
		gerr := GetObjectFn(ctx, bucket, object, 0, objInfo.Size, io.MultiWriter(writer, pipeWriter), etag, opts)
		pipeWriter.CloseWithError(gerr) // Close writer explicitly signaling we wrote all data.
	}()

	opts.UserDefined = c.getMetadata(objInfo)
	err = dcache.Put(ctx, bucket, object, NewPutObjReader(hashReader, nil, nil), opts)
	if err != nil {
		return err
	}
	pipeReader.Close()
	return
}

// Returns ObjectInfo from cache if available.
func (c cacheObjects) GetObjectInfo(ctx context.Context, bucket, object string, opts ObjectOptions) (ObjectInfo, error) {
	getObjectInfoFn := c.GetObjectInfoFn
	if c.isCacheExclude(bucket, object) {
		return getObjectInfoFn(ctx, bucket, object, opts)
	}
	// fetch cacheFSObjects if object is currently cached or nearest available cache drive
	dcache, err := c.cache.getCachedFSLoc(ctx, bucket, object)
	if err != nil {
		return getObjectInfoFn(ctx, bucket, object, opts)
	}
	objInfo, err := getObjectInfoFn(ctx, bucket, object, opts)
	if err != nil {
		if _, ok := err.(ObjectNotFound); ok {
			// Delete the cached entry if backend object was deleted.
			dcache.Delete(ctx, bucket, object)
			return ObjectInfo{}, err
		}
		if !backendDownError(err) {
			return ObjectInfo{}, err
		}
		// when backend is down, serve from cache.
		cachedObjInfo, cerr := dcache.GetObjectInfo(ctx, bucket, object, opts)
		if cerr == nil {
			return cachedObjInfo, nil
		}
		return ObjectInfo{}, BackendDown{}
	}
	// when backend is up, do a sanity check on cached object
	cachedObjInfo, err := dcache.GetObjectInfo(ctx, bucket, object, opts)
	if err != nil {
		return objInfo, nil
	}
	if cachedObjInfo.ETag != objInfo.ETag {
		// Delete the cached entry if the backend object was replaced.
		dcache.Delete(ctx, bucket, object)
	}
	return objInfo, nil
}

// Returns function "listDir" of the type listDirFunc.
// isLeaf - is used by listDir function to check if an entry is a leaf or non-leaf entry.
// disks - list of fsObjects
func listDirCacheFactory(isLeaf func(string, string) bool, disks []*cacheFSObjects) ListDirFunc {
	listCacheDirs := func(bucket, prefixDir, prefixEntry string) (dirs []string) {
		var entries []string
		for _, disk := range disks {
			// ignore disk-caches that might be missing/offline
			if disk == nil {
				continue
			}

			fs := disk.FSObjects
			var err error
			entries, err = readDir(pathJoin(fs.fsPath, bucket, prefixDir))
			if err != nil {
				continue
			}

			for i := range entries {
				if isLeaf(bucket, entries[i]) {
					entries[i] = strings.TrimSuffix(entries[i], slashSeparator)
				}
			}

			// Filter entries that have the prefix prefixEntry.
			entries = filterMatchingPrefix(entries, prefixEntry)
			dirs = append(dirs, entries...)
		}
		return dirs
	}

	// listDir - lists all the entries at a given prefix and given entry in the prefix.
	listDir := func(bucket, prefixDir, prefixEntry string) (mergedEntries []string) {
		cacheEntries := listCacheDirs(bucket, prefixDir, prefixEntry)
		for _, entry := range cacheEntries {
			// Find elements in entries which are not in mergedEntries
			idx := sort.SearchStrings(mergedEntries, entry)
			// if entry is already present in mergedEntries don't add.
			if idx < len(mergedEntries) && mergedEntries[idx] == entry {
				continue
			}
			mergedEntries = append(mergedEntries, entry)
			sort.Strings(mergedEntries)
		}
		return mergedEntries
	}
	return listDir
}

// List all objects at prefix upto maxKeys, optionally delimited by '/' from the cache. Maintains the list pool
// state for future re-entrant list requests.
func (c cacheObjects) listCacheObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result ListObjectsInfo, err error) {
	var objInfos []ObjectInfo
	var eof bool
	var nextMarker string

	recursive := true
	if delimiter == slashSeparator {
		recursive = false
	}
	walkResultCh, endWalkCh := c.listPool.Release(listParams{bucket, recursive, marker, prefix})
	if walkResultCh == nil {
		endWalkCh = make(chan struct{})

		listDir := listDirCacheFactory(func(bucket, object string) bool {
			fs, err := c.cache.getCacheFS(ctx, bucket, object)
			if err != nil {
				return false
			}
			_, err = fs.getObjectInfo(ctx, bucket, object)
			return err == nil
		}, c.cache.cfs)
		walkResultCh = startTreeWalk(ctx, bucket, prefix, marker, recursive, listDir, endWalkCh)
	}

	for i := 0; i < maxKeys; {
		walkResult, ok := <-walkResultCh
		if !ok {
			// Closed channel.
			eof = true
			break
		}

		entry := walkResult.entry
		var objInfo ObjectInfo
		if hasSuffix(entry, slashSeparator) {
			// Object name needs to be full path.
			objInfo.Bucket = bucket
			objInfo.Name = entry
			objInfo.IsDir = true
		} else {
			// Set the Mode to a "regular" file.
			var err error
			fs, err := c.cache.getCacheFS(ctx, bucket, entry)
			if err != nil {
				// Ignore errDiskNotFound
				if err == errDiskNotFound {
					continue
				}
				return result, toObjectErr(err, bucket, prefix)
			}
			objInfo, err = fs.getObjectInfo(ctx, bucket, entry)
			if err != nil {
				// Ignore ObjectNotFound error
				if _, ok := err.(ObjectNotFound); ok {
					continue
				}
				return result, toObjectErr(err, bucket, prefix)
			}
		}
		nextMarker = objInfo.Name
		objInfos = append(objInfos, objInfo)
		i++
		if walkResult.end {
			eof = true
			break
		}
	}

	params := listParams{bucket, recursive, nextMarker, prefix}
	if !eof {
		c.listPool.Set(params, walkResultCh, endWalkCh)
	}

	result = ListObjectsInfo{IsTruncated: !eof}
	for _, objInfo := range objInfos {
		result.NextMarker = objInfo.Name
		if objInfo.IsDir && delimiter == slashSeparator {
			result.Prefixes = append(result.Prefixes, objInfo.Name)
			continue
		}
		result.Objects = append(result.Objects, objInfo)
	}
	return result, nil
}

// listCacheV2Objects lists all blobs in bucket filtered by prefix from the cache
func (c cacheObjects) listCacheV2Objects(ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result ListObjectsV2Info, err error) {
	loi, err := c.listCacheObjects(ctx, bucket, prefix, continuationToken, delimiter, maxKeys)
	if err != nil {
		return result, err
	}

	listObjectsV2Info := ListObjectsV2Info{
		IsTruncated:           loi.IsTruncated,
		ContinuationToken:     continuationToken,
		NextContinuationToken: loi.NextMarker,
		Objects:               loi.Objects,
		Prefixes:              loi.Prefixes,
	}
	return listObjectsV2Info, err
}

// List all objects at prefix upto maxKeys., optionally delimited by '/'. Maintains the list pool
// state for future re-entrant list requests. Retrieve from cache if backend is down
func (c cacheObjects) ListObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result ListObjectsInfo, err error) {

	listObjectsFn := c.ListObjectsFn

	result, err = listObjectsFn(ctx, bucket, prefix, marker, delimiter, maxKeys)
	if err != nil {
		if backendDownError(err) {
			return c.listCacheObjects(ctx, bucket, prefix, marker, delimiter, maxKeys)
		}
		return
	}
	return
}

// ListObjectsV2 lists all blobs in bucket filtered by prefix
func (c cacheObjects) ListObjectsV2(ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result ListObjectsV2Info, err error) {
	listObjectsV2Fn := c.ListObjectsV2Fn

	result, err = listObjectsV2Fn(ctx, bucket, prefix, continuationToken, delimiter, maxKeys, fetchOwner, startAfter)
	if err != nil {
		if backendDownError(err) {
			return c.listCacheV2Objects(ctx, bucket, prefix, continuationToken, delimiter, maxKeys, fetchOwner, startAfter)
		}
		return
	}
	return
}

// Lists all the buckets in the cache
func (c cacheObjects) listBuckets(ctx context.Context) (buckets []BucketInfo, err error) {
	m := make(map[string]string)
	for _, cache := range c.cache.cfs {
		// ignore disk-caches that might be missing/offline
		if cache == nil {
			continue
		}
		entries, err := cache.ListBuckets(ctx)

		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			_, ok := m[entry.Name]
			if !ok {
				m[entry.Name] = entry.Name
				buckets = append(buckets, entry)
			}
		}
	}
	// Sort bucket infos by bucket name.
	sort.Sort(byBucketName(buckets))
	return
}

// Returns list of buckets from cache or the backend. If the backend is down, buckets
// available on cache are served.
func (c cacheObjects) ListBuckets(ctx context.Context) (buckets []BucketInfo, err error) {
	listBucketsFn := c.ListBucketsFn
	buckets, err = listBucketsFn(ctx)
	if err != nil {
		if backendDownError(err) {
			return c.listBuckets(ctx)
		}
		return []BucketInfo{}, err
	}
	return
}

// Returns bucket info from cache if backend is down.
func (c cacheObjects) GetBucketInfo(ctx context.Context, bucket string) (bucketInfo BucketInfo, err error) {
	getBucketInfoFn := c.GetBucketInfoFn
	bucketInfo, err = getBucketInfoFn(ctx, bucket)
	if backendDownError(err) {
		for _, cache := range c.cache.cfs {
			// ignore disk-caches that might be missing/offline
			if cache == nil {
				continue
			}
			if bucketInfo, err = cache.GetBucketInfo(ctx, bucket); err == nil {
				return
			}
		}
	}
	return
}

// Delete Object deletes from cache as well if backend operation succeeds
func (c cacheObjects) DeleteObject(ctx context.Context, bucket, object string) (err error) {
	if err = c.DeleteObjectFn(ctx, bucket, object); err != nil {
		return
	}
	if c.isCacheExclude(bucket, object) {
		return
	}
	dcache, cerr := c.cache.getCachedFSLoc(ctx, bucket, object)
	if cerr == nil {
		_ = dcache.DeleteObject(ctx, bucket, object)
	}
	return
}

func (c cacheObjects) DeleteObjects(ctx context.Context, bucket string, objects []string) ([]error, error) {
	errs := make([]error, len(objects))
	for idx, object := range objects {
		errs[idx] = c.DeleteObject(ctx, bucket, object)
	}
	return errs, nil
}

// Returns true if object should be excluded from cache
func (c cacheObjects) isCacheExclude(bucket, object string) bool {
	for _, pattern := range c.exclude {
		matchStr := fmt.Sprintf("%s/%s", bucket, object)
		if ok := wildcard.MatchSimple(pattern, matchStr); ok {
			return true
		}
	}
	return false
}

// PutObject - caches the uploaded object for single Put operations
func (c cacheObjects) PutObject(ctx context.Context, bucket, object string, r *PutObjReader, opts ObjectOptions) (objInfo ObjectInfo, err error) {
	putObjectFn := c.PutObjectFn
	data := r.Reader
	dcache, err := c.cache.getCacheFS(ctx, bucket, object)
	if err != nil {
		// disk cache could not be located,execute backend call.
		return putObjectFn(ctx, bucket, object, r, opts)
	}
	size := r.Size()

	// fetch from backend if there is no space on cache drive
	if !dcache.diskAvailable(size) {
		return putObjectFn(ctx, bucket, object, r, opts)
	}
	// fetch from backend if cache exclude pattern or cache-control
	// directive set to exclude
	if c.isCacheExclude(bucket, object) || filterFromCache(opts.UserDefined) {
		dcache.Delete(ctx, bucket, object)
		return putObjectFn(ctx, bucket, object, r, opts)
	}
	objInfo = ObjectInfo{}
	// Initialize pipe to stream data to backend
	pipeReader, pipeWriter := io.Pipe()
	hashReader, err := hash.NewReader(pipeReader, size, data.MD5HexString(), data.SHA256HexString(), data.ActualSize(), globalCLIContext.StrictS3Compat)
	if err != nil {
		return ObjectInfo{}, err
	}
	// Initialize pipe to stream data to cache
	rPipe, wPipe := io.Pipe()
	cHashReader, err := hash.NewReader(rPipe, size, data.MD5HexString(), data.SHA256HexString(), data.ActualSize(), globalCLIContext.StrictS3Compat)
	if err != nil {
		return ObjectInfo{}, err
	}
	oinfoCh := make(chan ObjectInfo)
	errCh := make(chan error)
	go func() {
		oinfo, perr := putObjectFn(ctx, bucket, object, NewPutObjReader(hashReader, nil, nil), opts)
		if perr != nil {
			pipeWriter.CloseWithError(perr)
			wPipe.CloseWithError(perr)
			close(oinfoCh)
			errCh <- perr
			return
		}
		close(errCh)
		oinfoCh <- oinfo
	}()

	go func() {
		if err = dcache.Put(ctx, bucket, object, NewPutObjReader(cHashReader, nil, nil), opts); err != nil {
			wPipe.CloseWithError(err)
			return
		}
	}()

	mwriter := io.MultiWriter(pipeWriter, wPipe)
	_, err = io.Copy(mwriter, data)
	if err != nil {
		err = <-errCh
		return objInfo, err
	}
	pipeWriter.Close()
	wPipe.Close()
	objInfo = <-oinfoCh
	return objInfo, err
}

// NewMultipartUpload - Starts a new multipart upload operation to backend and cache.
func (c cacheObjects) NewMultipartUpload(ctx context.Context, bucket, object string, opts ObjectOptions) (uploadID string, err error) {
	newMultipartUploadFn := c.NewMultipartUploadFn

	if c.isCacheExclude(bucket, object) || filterFromCache(opts.UserDefined) {
		return newMultipartUploadFn(ctx, bucket, object, opts)
	}

	dcache, err := c.cache.getCacheFS(ctx, bucket, object)
	if err != nil {
		// disk cache could not be located,execute backend call.
		return newMultipartUploadFn(ctx, bucket, object, opts)
	}

	uploadID, err = newMultipartUploadFn(ctx, bucket, object, opts)
	if err != nil {
		return
	}
	// create new multipart upload in cache with same uploadID
	dcache.NewMultipartUpload(ctx, bucket, object, uploadID, opts)
	return uploadID, err
}

// PutObjectPart - uploads part to backend and cache simultaneously.
func (c cacheObjects) PutObjectPart(ctx context.Context, bucket, object, uploadID string, partID int, r *PutObjReader, opts ObjectOptions) (info PartInfo, err error) {
	data := r.Reader
	putObjectPartFn := c.PutObjectPartFn
	dcache, err := c.cache.getCacheFS(ctx, bucket, object)
	if err != nil {
		// disk cache could not be located,execute backend call.
		return putObjectPartFn(ctx, bucket, object, uploadID, partID, r, opts)
	}

	if c.isCacheExclude(bucket, object) {
		return putObjectPartFn(ctx, bucket, object, uploadID, partID, r, opts)
	}

	// make sure cache has at least size space available
	size := data.Size()
	if !dcache.diskAvailable(size) {
		select {
		case dcache.purgeChan <- struct{}{}:
		default:
		}
		return putObjectPartFn(ctx, bucket, object, uploadID, partID, r, opts)
	}

	info = PartInfo{}
	// Initialize pipe to stream data to backend
	pipeReader, pipeWriter := io.Pipe()
	hashReader, err := hash.NewReader(pipeReader, size, data.MD5HexString(), data.SHA256HexString(), data.ActualSize(), globalCLIContext.StrictS3Compat)
	if err != nil {
		return
	}
	// Initialize pipe to stream data to cache
	rPipe, wPipe := io.Pipe()
	cHashReader, err := hash.NewReader(rPipe, size, data.MD5HexString(), data.SHA256HexString(), data.ActualSize(), globalCLIContext.StrictS3Compat)
	if err != nil {
		return
	}
	pinfoCh := make(chan PartInfo)
	errorCh := make(chan error)
	go func() {
		info, err = putObjectPartFn(ctx, bucket, object, uploadID, partID, NewPutObjReader(hashReader, nil, nil), opts)
		if err != nil {
			close(pinfoCh)
			pipeWriter.CloseWithError(err)
			wPipe.CloseWithError(err)
			errorCh <- err
			return
		}
		close(errorCh)
		pinfoCh <- info
	}()
	go func() {
		if _, perr := dcache.PutObjectPart(ctx, bucket, object, uploadID, partID, NewPutObjReader(cHashReader, nil, nil), opts); perr != nil {
			wPipe.CloseWithError(perr)
			return
		}
	}()

	mwriter := io.MultiWriter(pipeWriter, wPipe)
	_, err = io.Copy(mwriter, data)
	if err != nil {
		err = <-errorCh
		return PartInfo{}, err
	}
	pipeWriter.Close()
	wPipe.Close()
	info = <-pinfoCh
	return info, err
}

// AbortMultipartUpload - aborts multipart upload on backend and cache.
func (c cacheObjects) AbortMultipartUpload(ctx context.Context, bucket, object, uploadID string) error {
	abortMultipartUploadFn := c.AbortMultipartUploadFn

	if c.isCacheExclude(bucket, object) {
		return abortMultipartUploadFn(ctx, bucket, object, uploadID)
	}

	dcache, err := c.cache.getCacheFS(ctx, bucket, object)
	if err != nil {
		// disk cache could not be located,execute backend call.
		return abortMultipartUploadFn(ctx, bucket, object, uploadID)
	}
	// execute backend operation
	err = abortMultipartUploadFn(ctx, bucket, object, uploadID)
	if err != nil {
		return err
	}
	// abort multipart upload on cache
	dcache.AbortMultipartUpload(ctx, bucket, object, uploadID)
	return nil
}

// CompleteMultipartUpload - completes multipart upload operation on backend and cache.
func (c cacheObjects) CompleteMultipartUpload(ctx context.Context, bucket, object, uploadID string, uploadedParts []CompletePart, opts ObjectOptions) (objInfo ObjectInfo, err error) {
	completeMultipartUploadFn := c.CompleteMultipartUploadFn

	if c.isCacheExclude(bucket, object) {
		return completeMultipartUploadFn(ctx, bucket, object, uploadID, uploadedParts, opts)
	}

	dcache, err := c.cache.getCacheFS(ctx, bucket, object)
	if err != nil {
		// disk cache could not be located,execute backend call.
		return completeMultipartUploadFn(ctx, bucket, object, uploadID, uploadedParts, opts)
	}
	// perform backend operation
	objInfo, err = completeMultipartUploadFn(ctx, bucket, object, uploadID, uploadedParts, opts)
	if err != nil {
		return
	}
	// create new multipart upload in cache with same uploadID
	dcache.CompleteMultipartUpload(ctx, bucket, object, uploadID, uploadedParts, opts)
	return
}

// StorageInfo - returns underlying storage statistics.
func (c cacheObjects) StorageInfo(ctx context.Context) (cInfo CacheStorageInfo) {
	var total, free uint64
	for _, cfs := range c.cache.cfs {
		if cfs == nil {
			continue
		}
		info, err := getDiskInfo(cfs.fsPath)
		logger.GetReqInfo(ctx).AppendTags("cachePath", cfs.fsPath)
		logger.LogIf(ctx, err)
		total += info.Total
		free += info.Free
	}
	return CacheStorageInfo{
		Total: total,
		Free:  free,
	}
}

// DeleteBucket - marks bucket to be deleted from cache if bucket is deleted from backend.
func (c cacheObjects) DeleteBucket(ctx context.Context, bucket string) (err error) {
	deleteBucketFn := c.DeleteBucketFn
	var toDel []*cacheFSObjects
	for _, cfs := range c.cache.cfs {
		// ignore disk-caches that might be missing/offline
		if cfs == nil {
			continue
		}
		if _, cerr := cfs.GetBucketInfo(ctx, bucket); cerr == nil {
			toDel = append(toDel, cfs)
		}
	}
	// perform backend operation
	err = deleteBucketFn(ctx, bucket)
	if err != nil {
		return
	}
	// move bucket metadata and content to cache's trash dir
	for _, d := range toDel {
		d.moveBucketToTrash(ctx, bucket)
	}
	return
}

// newCache initializes the cacheFSObjects for the "drives" specified in config.json
// or the global env overrides.
func newCache(config CacheConfig) (*diskCache, error) {
	var cfsObjects []*cacheFSObjects
	ctx := logger.SetReqInfo(context.Background(), &logger.ReqInfo{})
	formats, err := loadAndValidateCacheFormat(ctx, config.Drives)
	if err != nil {
		return nil, err
	}
	for i, dir := range config.Drives {
		// skip cacheFSObjects creation for cache drives missing a format.json
		if formats[i] == nil {
			cfsObjects = append(cfsObjects, nil)
			continue
		}
		if err := checkAtimeSupport(dir); err != nil {
			return nil, errors.New("Atime support required for disk caching")
		}
		cache, err := newCacheFSObjects(dir, config.Expiry, config.MaxUse)
		if err != nil {
			return nil, err
		}
		// Start the purging go-routine for entries that have expired
		go cache.purge()

		// Start trash purge routine for deleted buckets.
		go cache.purgeTrash()

		cfsObjects = append(cfsObjects, cache)
	}
	return &diskCache{cfs: cfsObjects}, nil
}

// Return error if Atime is disabled on the O/S
func checkAtimeSupport(dir string) (err error) {
	file, err := ioutil.TempFile(dir, "prefix")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	finfo1, err := os.Stat(file.Name())
	if err != nil {
		return
	}
	if _, err = io.Copy(ioutil.Discard, file); err != io.EOF {
		return
	}

	finfo2, err := os.Stat(file.Name())

	if atime.Get(finfo2).Equal(atime.Get(finfo1)) {
		return errors.New("Atime not supported")
	}
	return
}

// Returns cacheObjects for use by Server.
func newServerCacheObjects(config CacheConfig) (CacheObjectLayer, error) {
	// list of disk caches for cache "drives" specified in config.json or MINIO_CACHE_DRIVES env var.
	dcache, err := newCache(config)
	if err != nil {
		return nil, err
	}

	return &cacheObjects{
		cache:    dcache,
		exclude:  config.Exclude,
		listPool: NewTreeWalkPool(globalLookupTimeout),
		GetObjectFn: func(ctx context.Context, bucket, object string, startOffset int64, length int64, writer io.Writer, etag string, opts ObjectOptions) error {
			return newObjectLayerFn().GetObject(ctx, bucket, object, startOffset, length, writer, etag, opts)
		},
		GetObjectInfoFn: func(ctx context.Context, bucket, object string, opts ObjectOptions) (ObjectInfo, error) {
			return newObjectLayerFn().GetObjectInfo(ctx, bucket, object, opts)
		},
		GetObjectNInfoFn: func(ctx context.Context, bucket, object string, rs *HTTPRangeSpec, h http.Header, lockType LockType, opts ObjectOptions) (gr *GetObjectReader, err error) {
			return newObjectLayerFn().GetObjectNInfo(ctx, bucket, object, rs, h, lockType, opts)
		},
		PutObjectFn: func(ctx context.Context, bucket, object string, data *PutObjReader, opts ObjectOptions) (objInfo ObjectInfo, err error) {
			return newObjectLayerFn().PutObject(ctx, bucket, object, data, opts)
		},
		DeleteObjectFn: func(ctx context.Context, bucket, object string) error {
			return newObjectLayerFn().DeleteObject(ctx, bucket, object)
		},
		DeleteObjectsFn: func(ctx context.Context, bucket string, objects []string) ([]error, error) {
			errs := make([]error, len(objects))
			for idx, object := range objects {
				errs[idx] = newObjectLayerFn().DeleteObject(ctx, bucket, object)
			}
			return errs, nil
		},

		ListObjectsFn: func(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (result ListObjectsInfo, err error) {
			return newObjectLayerFn().ListObjects(ctx, bucket, prefix, marker, delimiter, maxKeys)
		},
		ListObjectsV2Fn: func(ctx context.Context, bucket, prefix, continuationToken, delimiter string, maxKeys int, fetchOwner bool, startAfter string) (result ListObjectsV2Info, err error) {
			return newObjectLayerFn().ListObjectsV2(ctx, bucket, prefix, continuationToken, delimiter, maxKeys, fetchOwner, startAfter)
		},
		ListBucketsFn: func(ctx context.Context) (buckets []BucketInfo, err error) {
			return newObjectLayerFn().ListBuckets(ctx)
		},
		GetBucketInfoFn: func(ctx context.Context, bucket string) (bucketInfo BucketInfo, err error) {
			return newObjectLayerFn().GetBucketInfo(ctx, bucket)
		},
		NewMultipartUploadFn: func(ctx context.Context, bucket, object string, opts ObjectOptions) (uploadID string, err error) {
			return newObjectLayerFn().NewMultipartUpload(ctx, bucket, object, opts)
		},
		PutObjectPartFn: func(ctx context.Context, bucket, object, uploadID string, partID int, data *PutObjReader, opts ObjectOptions) (info PartInfo, err error) {
			return newObjectLayerFn().PutObjectPart(ctx, bucket, object, uploadID, partID, data, opts)
		},
		AbortMultipartUploadFn: func(ctx context.Context, bucket, object, uploadID string) error {
			return newObjectLayerFn().AbortMultipartUpload(ctx, bucket, object, uploadID)
		},
		CompleteMultipartUploadFn: func(ctx context.Context, bucket, object, uploadID string, uploadedParts []CompletePart, opts ObjectOptions) (objInfo ObjectInfo, err error) {
			return newObjectLayerFn().CompleteMultipartUpload(ctx, bucket, object, uploadID, uploadedParts, opts)
		},
		DeleteBucketFn: func(ctx context.Context, bucket string) error {
			return newObjectLayerFn().DeleteBucket(ctx, bucket)
		},
	}, nil
}

type cacheControl struct {
	exclude  bool
	expiry   time.Time
	maxAge   int
	sMaxAge  int
	minFresh int
}

// cache exclude directives in cache-control header
var cacheExcludeDirectives = []string{
	"no-cache",
	"no-store",
	"must-revalidate",
}

// returns true if cache exclude directives are set.
func isCacheExcludeDirective(s string) bool {
	for _, directive := range cacheExcludeDirectives {
		if s == directive {
			return true
		}
	}
	return false
}

// returns struct with cache-control settings from user metadata.
func getCacheControlOpts(m map[string]string) (c cacheControl, err error) {
	var headerVal string
	for k, v := range m {
		if k == "cache-control" {
			headerVal = v
		}
		if k == "expires" {
			if e, err := http.ParseTime(v); err == nil {
				c.expiry = e
			}
		}
	}
	if headerVal == "" {
		return
	}
	headerVal = strings.ToLower(headerVal)
	headerVal = strings.TrimSpace(headerVal)

	vals := strings.Split(headerVal, ",")
	for _, val := range vals {
		val = strings.TrimSpace(val)
		p := strings.Split(val, "=")
		if isCacheExcludeDirective(p[0]) {
			c.exclude = true
			continue
		}

		if len(p) != 2 {
			continue
		}
		if p[0] == "max-age" ||
			p[0] == "s-maxage" ||
			p[0] == "min-fresh" {
			i, err := strconv.Atoi(p[1])
			if err != nil {
				return c, err
			}
			if p[0] == "max-age" {
				c.maxAge = i
			}
			if p[0] == "s-maxage" {
				c.sMaxAge = i
			}
			if p[0] == "min-fresh" {
				c.minFresh = i
			}
		}
	}
	return c, nil
}

// return true if metadata has a cache-control header
// directive to exclude object from cache.
func filterFromCache(m map[string]string) bool {
	c, err := getCacheControlOpts(m)
	if err != nil {
		return false
	}
	return c.exclude
}

// returns true if cache expiry conditions met in cache-control/expiry metadata.
func isStaleCache(objInfo ObjectInfo) bool {
	c, err := getCacheControlOpts(objInfo.UserDefined)
	if err != nil {
		return false
	}
	now := time.Now()
	if c.sMaxAge > 0 && c.sMaxAge > int(now.Sub(objInfo.ModTime).Seconds()) {
		return true
	}
	if c.maxAge > 0 && c.maxAge > int(now.Sub(objInfo.ModTime).Seconds()) {
		return true
	}
	if !c.expiry.Equal(time.Time{}) && c.expiry.Before(time.Now()) {
		return true
	}
	if c.minFresh > 0 && c.minFresh <= int(now.Sub(objInfo.ModTime).Seconds()) {
		return true
	}
	return false
}
