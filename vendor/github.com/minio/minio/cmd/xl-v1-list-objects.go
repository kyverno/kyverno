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
	"sort"
)

// Returns function "listDir" of the type listDirFunc.
// disks - used for doing disk.ListDir()
func listDirFactory(ctx context.Context, disks ...StorageAPI) ListDirFunc {
	// Returns sorted merged entries from all the disks.
	listDir := func(bucket, prefixDir, prefixEntry string) (mergedEntries []string) {
		for _, disk := range disks {
			if disk == nil {
				continue
			}
			var entries []string
			var newEntries []string
			var err error
			entries, err = disk.ListDir(bucket, prefixDir, -1, xlMetaJSONFile)
			if err != nil {
				continue
			}

			// Find elements in entries which are not in mergedEntries
			for _, entry := range entries {
				idx := sort.SearchStrings(mergedEntries, entry)
				// if entry is already present in mergedEntries don't add.
				if idx < len(mergedEntries) && mergedEntries[idx] == entry {
					continue
				}
				newEntries = append(newEntries, entry)
			}

			if len(newEntries) > 0 {
				// Merge the entries and sort it.
				mergedEntries = append(mergedEntries, newEntries...)
				sort.Strings(mergedEntries)
			}
		}
		return filterMatchingPrefix(mergedEntries, prefixEntry)
	}
	return listDir
}

// listObjects - wrapper function implemented over file tree walk.
func (xl xlObjects) listObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (loi ListObjectsInfo, e error) {
	// Default is recursive, if delimiter is set then list non recursive.
	recursive := true
	if delimiter == SlashSeparator {
		recursive = false
	}

	walkResultCh, endWalkCh := xl.listPool.Release(listParams{bucket, recursive, marker, prefix, false})
	if walkResultCh == nil {
		endWalkCh = make(chan struct{})
		listDir := listDirFactory(ctx, xl.getLoadBalancedDisks()...)
		walkResultCh = startTreeWalk(ctx, bucket, prefix, marker, recursive, listDir, endWalkCh)
	}

	var objInfos []ObjectInfo
	var eof bool
	var nextMarker string
	for i := 0; i < maxKeys; {

		walkResult, ok := <-walkResultCh
		if !ok {
			// Closed channel.
			eof = true
			break
		}
		entry := walkResult.entry
		var objInfo ObjectInfo
		if HasSuffix(entry, SlashSeparator) {
			// Object name needs to be full path.
			objInfo.Bucket = bucket
			objInfo.Name = entry
			objInfo.IsDir = true
		} else {
			// Set the Mode to a "regular" file.
			var err error
			objInfo, err = xl.getObjectInfo(ctx, bucket, entry)
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
		}
		nextMarker = objInfo.Name
		objInfos = append(objInfos, objInfo)
		i++
		if walkResult.end {
			eof = true
			break
		}
	}

	params := listParams{bucket, recursive, nextMarker, prefix, false}
	if !eof {
		xl.listPool.Set(params, walkResultCh, endWalkCh)
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

	return result, nil
}

// ListObjects - list all objects at prefix, delimited by '/'.
func (xl xlObjects) ListObjects(ctx context.Context, bucket, prefix, marker, delimiter string, maxKeys int) (loi ListObjectsInfo, e error) {
	if err := checkListObjsArgs(ctx, bucket, prefix, marker, delimiter, xl); err != nil {
		return loi, err
	}

	// With max keys of zero we have reached eof, return right here.
	if maxKeys == 0 {
		return loi, nil
	}

	// Marker is set validate pre-condition.
	if marker != "" {
		// Marker not common with prefix is not implemented.Send an empty response
		if !HasPrefix(marker, prefix) {
			return ListObjectsInfo{}, e
		}
	}

	// For delimiter and prefix as '/' we do not list anything at all
	// since according to s3 spec we stop at the 'delimiter' along
	// with the prefix. On a flat namespace with 'prefix' as '/'
	// we don't have any entries, since all the keys are of form 'keyName/...'
	if delimiter == SlashSeparator && prefix == SlashSeparator {
		return loi, nil
	}

	// Over flowing count - reset to maxObjectList.
	if maxKeys < 0 || maxKeys > maxObjectList {
		maxKeys = maxObjectList
	}

	// Initiate a list operation, if successful filter and return quickly.
	listObjInfo, err := xl.listObjects(ctx, bucket, prefix, marker, delimiter, maxKeys)
	if err == nil {
		// We got the entries successfully return.
		return listObjInfo, nil
	}

	// Return error at the end.
	return loi, toObjectErr(err, bucket, prefix)
}
