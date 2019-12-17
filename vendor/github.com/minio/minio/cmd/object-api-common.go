/*
 * MinIO Cloud Storage, (C) 2016-2019 MinIO, Inc.
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
	"path"
	"sync"

	"strings"

	humanize "github.com/dustin/go-humanize"
	"github.com/minio/minio/cmd/logger"
)

const (
	// Block size used for all internal operations version 1.
	blockSizeV1 = 10 * humanize.MiByte

	// Staging buffer read size for all internal operations version 1.
	readSizeV1 = 1 * humanize.MiByte

	// Buckets meta prefix.
	bucketMetaPrefix = "buckets"

	// ETag (hex encoded md5sum) of empty string.
	emptyETag = "d41d8cd98f00b204e9800998ecf8427e"
)

// Global object layer mutex, used for safely updating object layer.
var globalObjLayerMutex *sync.RWMutex

// Global object layer, only accessed by globalObjectAPI.
var globalObjectAPI ObjectLayer

//Global cacheObjects, only accessed by newCacheObjectsFn().
var globalCacheObjectAPI CacheObjectLayer

func init() {
	// Initialize this once per server initialization.
	globalObjLayerMutex = &sync.RWMutex{}
}

// Checks if the object is a directory, this logic uses
// if size == 0 and object ends with SlashSeparator then
// returns true.
func isObjectDir(object string, size int64) bool {
	return HasSuffix(object, SlashSeparator) && size == 0
}

// Converts just bucket, object metadata into ObjectInfo datatype.
func dirObjectInfo(bucket, object string, size int64, metadata map[string]string) ObjectInfo {
	// This is a special case with size as '0' and object ends with
	// a slash separator, we treat it like a valid operation and
	// return success.
	etag := metadata["etag"]
	delete(metadata, "etag")
	if etag == "" {
		etag = emptyETag
	}

	return ObjectInfo{
		Bucket:      bucket,
		Name:        object,
		ModTime:     UTCNow(),
		ContentType: "application/octet-stream",
		IsDir:       true,
		Size:        size,
		ETag:        etag,
		UserDefined: metadata,
	}
}

func deleteBucketMetadata(ctx context.Context, bucket string, objAPI ObjectLayer) {
	// Delete bucket access policy, if present - ignore any errors.
	removePolicyConfig(ctx, objAPI, bucket)

	// Delete notification config, if present - ignore any errors.
	removeNotificationConfig(ctx, objAPI, bucket)

	// Delete listener config, if present - ignore any errors.
	removeListenerConfig(ctx, objAPI, bucket)
}

// Depending on the disk type network or local, initialize storage API.
func newStorageAPI(endpoint Endpoint) (storage StorageAPI, err error) {
	if endpoint.IsLocal {
		storage, err := newPosix(endpoint.Path)
		if err != nil {
			return nil, err
		}
		return &posixDiskIDCheck{storage: storage}, nil
	}

	return newStorageRESTClient(endpoint), nil
}

// Cleanup a directory recursively.
func cleanupDir(ctx context.Context, storage StorageAPI, volume, dirPath string) error {
	var delFunc func(string) error
	// Function to delete entries recursively.
	delFunc = func(entryPath string) error {
		if !HasSuffix(entryPath, SlashSeparator) {
			// Delete the file entry.
			err := storage.DeleteFile(volume, entryPath)
			logger.LogIf(ctx, err)
			return err
		}

		// If it's a directory, list and call delFunc() for each entry.
		entries, err := storage.ListDir(volume, entryPath, -1, "")
		// If entryPath prefix never existed, safe to ignore.
		if err == errFileNotFound {
			return nil
		} else if err != nil { // For any other errors fail.
			logger.LogIf(ctx, err)
			return err
		} // else on success..

		// Entry path is empty, just delete it.
		if len(entries) == 0 {
			err = storage.DeleteFile(volume, entryPath)
			logger.LogIf(ctx, err)
			return err
		}

		// Recurse and delete all other entries.
		for _, entry := range entries {
			if err = delFunc(pathJoin(entryPath, entry)); err != nil {
				return err
			}
		}
		return nil
	}
	err := delFunc(retainSlash(pathJoin(dirPath)))
	return err
}

// Cleanup objects in bulk and recursively: each object will have a list of sub-files to delete in the backend
func cleanupObjectsBulk(storage StorageAPI, volume string, objsPaths []string, errs []error) ([]error, error) {
	// The list of files in disk to delete
	var filesToDelete []string
	// Map files to delete to the passed objsPaths
	var filesToDeleteObjsIndexes []int

	// Traverse and return the list of sub entries
	var traverse func(string) ([]string, error)
	traverse = func(entryPath string) ([]string, error) {
		var output = make([]string, 0)
		if !HasSuffix(entryPath, SlashSeparator) {
			output = append(output, entryPath)
			return output, nil
		}
		entries, err := storage.ListDir(volume, entryPath, -1, "")
		if err != nil {
			if err == errFileNotFound {
				return nil, nil
			}
			return nil, err
		}

		for _, entry := range entries {
			subEntries, err := traverse(pathJoin(entryPath, entry))
			if err != nil {
				return nil, err
			}
			output = append(output, subEntries...)
		}
		return output, nil
	}

	// Find and collect the list of files to remove associated
	// to the passed objects paths
	for idx, objPath := range objsPaths {
		if errs[idx] != nil {
			continue
		}
		output, err := traverse(retainSlash(pathJoin(objPath)))
		if err != nil {
			errs[idx] = err
			continue
		} else {
			errs[idx] = nil
		}
		filesToDelete = append(filesToDelete, output...)
		for i := 0; i < len(output); i++ {
			filesToDeleteObjsIndexes = append(filesToDeleteObjsIndexes, idx)
		}
	}

	// Reverse the list so remove can succeed
	reverseStringSlice(filesToDelete)

	dErrs, err := storage.DeleteFileBulk(volume, filesToDelete)
	if err != nil {
		return nil, err
	}

	// Map files deletion errors to the correspondent objects
	for i := range dErrs {
		if dErrs[i] != nil {
			if errs[filesToDeleteObjsIndexes[i]] != nil {
				errs[filesToDeleteObjsIndexes[i]] = dErrs[i]
			}
		}
	}

	return errs, nil
}

// Removes notification.xml for a given bucket, only used during DeleteBucket.
func removeNotificationConfig(ctx context.Context, objAPI ObjectLayer, bucket string) error {
	// Verify bucket is valid.
	if !IsValidBucketName(bucket) {
		return BucketNameInvalid{Bucket: bucket}
	}

	ncPath := path.Join(bucketConfigPrefix, bucket, bucketNotificationConfig)
	return objAPI.DeleteObject(ctx, minioMetaBucket, ncPath)
}

// Remove listener configuration from storage layer. Used when a bucket is deleted.
func removeListenerConfig(ctx context.Context, objAPI ObjectLayer, bucket string) error {
	// make the path
	lcPath := path.Join(bucketConfigPrefix, bucket, bucketListenerConfig)
	return objAPI.DeleteObject(ctx, minioMetaBucket, lcPath)
}

func listObjectsNonSlash(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int, tpool *TreeWalkPool, listDir ListDirFunc, getObjInfo func(context.Context, string, string) (ObjectInfo, error), getObjectInfoDirs ...func(context.Context, string, string) (ObjectInfo, error)) (loi ListObjectsInfo, err error) {
	endWalkCh := make(chan struct{})
	defer close(endWalkCh)
	recursive := true
	walkResultCh := startTreeWalk(ctx, bucket, prefix, "", recursive, listDir, endWalkCh)

	var objInfos []ObjectInfo
	var eof bool
	var prevPrefix string

	for {
		if len(objInfos) == maxKeys {
			break
		}
		result, ok := <-walkResultCh
		if !ok {
			eof = true
			break
		}

		var objInfo ObjectInfo
		var err error

		index := strings.Index(strings.TrimPrefix(result.entry, prefix), delimiter)
		if index == -1 {
			objInfo, err = getObjInfo(ctx, bucket, result.entry)
			if err != nil {
				// Ignore errFileNotFound as the object might have got
				// deleted in the interim period of listing and getObjectInfo(),
				// ignore quorum error as it might be an entry from an outdated disk.
				if IsErrIgnored(err, []error{
					errFileNotFound,
					errXLReadQuorum,
				}...) {
					continue
				}
				return loi, toObjectErr(err, bucket, prefix)
			}
		} else {
			index = len(prefix) + index + len(delimiter)
			currPrefix := result.entry[:index]
			if currPrefix == prevPrefix {
				continue
			}
			prevPrefix = currPrefix

			objInfo = ObjectInfo{
				Bucket: bucket,
				Name:   currPrefix,
				IsDir:  true,
			}
		}

		if objInfo.Name <= marker {
			continue
		}

		objInfos = append(objInfos, objInfo)
		if result.end {
			eof = true
			break
		}
	}

	result := ListObjectsInfo{}
	for _, objInfo := range objInfos {
		if objInfo.IsDir {
			result.Prefixes = append(result.Prefixes, objInfo.Name)
			continue
		}
		result.Objects = append(result.Objects, objInfo)
	}

	if !eof {
		result.IsTruncated = true
		if len(objInfos) > 0 {
			result.NextMarker = objInfos[len(objInfos)-1].Name
		}
	}

	return result, nil
}

func listObjects(ctx context.Context, obj ObjectLayer, bucket, prefix, marker, delimiter string, maxKeys int, tpool *TreeWalkPool, listDir ListDirFunc, getObjInfo func(context.Context, string, string) (ObjectInfo, error), getObjectInfoDirs ...func(context.Context, string, string) (ObjectInfo, error)) (loi ListObjectsInfo, err error) {
	if delimiter != SlashSeparator && delimiter != "" {
		return listObjectsNonSlash(ctx, bucket, prefix, marker, delimiter, maxKeys, tpool, listDir, getObjInfo, getObjectInfoDirs...)
	}

	if err := checkListObjsArgs(ctx, bucket, prefix, marker, delimiter, obj); err != nil {
		return loi, err
	}

	// Marker is set validate pre-condition.
	if marker != "" {
		// Marker not common with prefix is not implemented. Send an empty response
		if !HasPrefix(marker, prefix) {
			return loi, nil
		}
	}

	// With max keys of zero we have reached eof, return right here.
	if maxKeys == 0 {
		return loi, nil
	}

	// For delimiter and prefix as '/' we do not list anything at all
	// since according to s3 spec we stop at the 'delimiter'
	// along // with the prefix. On a flat namespace with 'prefix'
	// as '/' we don't have any entries, since all the keys are
	// of form 'keyName/...'
	if delimiter == SlashSeparator && prefix == SlashSeparator {
		return loi, nil
	}

	// Over flowing count - reset to maxObjectList.
	if maxKeys < 0 || maxKeys > maxObjectList {
		maxKeys = maxObjectList
	}

	// Default is recursive, if delimiter is set then list non recursive.
	recursive := true
	if delimiter == SlashSeparator {
		recursive = false
	}

	walkResultCh, endWalkCh := tpool.Release(listParams{bucket, recursive, marker, prefix, false})
	if walkResultCh == nil {
		endWalkCh = make(chan struct{})
		walkResultCh = startTreeWalk(ctx, bucket, prefix, marker, recursive, listDir, endWalkCh)
	}

	var objInfos []ObjectInfo
	var eof bool
	var nextMarker string

	// List until maxKeys requested.
	for i := 0; i < maxKeys; {
		walkResult, ok := <-walkResultCh
		if !ok {
			// Closed channel.
			eof = true
			break
		}

		var objInfo ObjectInfo
		var err error
		if HasSuffix(walkResult.entry, SlashSeparator) {
			for _, getObjectInfoDir := range getObjectInfoDirs {
				objInfo, err = getObjectInfoDir(ctx, bucket, walkResult.entry)
				if err == nil {
					break
				}
				if err == errFileNotFound {
					err = nil
					objInfo = ObjectInfo{
						Bucket: bucket,
						Name:   walkResult.entry,
						IsDir:  true,
					}
				}
			}
		} else {
			objInfo, err = getObjInfo(ctx, bucket, walkResult.entry)
		}
		if err != nil {
			// Ignore errFileNotFound as the object might have got
			// deleted in the interim period of listing and getObjectInfo(),
			// ignore quorum error as it might be an entry from an outdated disk.
			if IsErrIgnored(err, []error{
				errFileNotFound,
				errXLReadQuorum,
			}...) {
				continue
			}
			return loi, toObjectErr(err, bucket, prefix)
		}
		nextMarker = objInfo.Name
		objInfos = append(objInfos, objInfo)
		if walkResult.end {
			eof = true
			break
		}
		i++
	}

	// Save list routine for the next marker if we haven't reached EOF.
	params := listParams{bucket, recursive, nextMarker, prefix, false}
	if !eof {
		tpool.Set(params, walkResultCh, endWalkCh)
	}

	result := ListObjectsInfo{}
	for _, objInfo := range objInfos {
		if objInfo.IsDir && delimiter == SlashSeparator {
			result.Prefixes = append(result.Prefixes, objInfo.Name)
			continue
		}
		result.Objects = append(result.Objects, objInfo)
	}

	if !eof {
		result.IsTruncated = true
		if len(objInfos) > 0 {
			result.NextMarker = objInfos[len(objInfos)-1].Name
		}
	}

	// Success.
	return result, nil
}

// Fetch the histogram interval corresponding
// to the passed object size.
func objSizeToHistoInterval(usize uint64) string {
	size := int64(usize)

	var interval objectHistogramInterval
	for _, interval = range ObjectsHistogramIntervals {
		var cond1, cond2 bool
		if size >= interval.start || interval.start == -1 {
			cond1 = true
		}
		if size <= interval.end || interval.end == -1 {
			cond2 = true
		}
		if cond1 && cond2 {
			return interval.name
		}
	}

	// This would be the last element of histogram intervals
	return interval.name
}
