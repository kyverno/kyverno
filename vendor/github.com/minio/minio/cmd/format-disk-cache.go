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
	"os"
	"reflect"

	"github.com/minio/minio/cmd/logger"
)

const (
	// Represents Cache format json holding details on all other cache drives in use.
	formatCache = "cache"

	// formatCacheV1.Cache.Version
	formatCacheVersionV1 = "1"

	formatMetaVersion1 = "1"

	formatCacheV1DistributionAlgo = "CRCMOD"
)

// Represents the current cache structure with list of
// disks comprising the disk cache
// formatCacheV1 - structure holds format config version '1'.
type formatCacheV1 struct {
	formatMetaV1
	Cache struct {
		Version string `json:"version"` // Version of 'cache' format.
		This    string `json:"this"`    // This field carries assigned disk uuid.
		// Disks field carries the input disk order generated the first
		// time when fresh disks were supplied.
		Disks []string `json:"disks"`
		// Distribution algorithm represents the hashing algorithm
		// to pick the right set index for an object.
		DistributionAlgo string `json:"distributionAlgo"`
	} `json:"cache"` // Cache field holds cache format.
}

// Used to detect the version of "cache" format.
type formatCacheVersionDetect struct {
	Cache struct {
		Version string `json:"version"`
	} `json:"cache"`
}

// Return a slice of format, to be used to format uninitialized disks.
func newFormatCacheV1(drives []string) []*formatCacheV1 {
	diskCount := len(drives)
	var disks = make([]string, diskCount)

	var formats = make([]*formatCacheV1, diskCount)

	for i := 0; i < diskCount; i++ {
		format := &formatCacheV1{}
		format.Version = formatMetaVersion1
		format.Format = formatCache
		format.Cache.Version = formatCacheVersionV1
		format.Cache.DistributionAlgo = formatCacheV1DistributionAlgo
		format.Cache.This = mustGetUUID()
		formats[i] = format
		disks[i] = formats[i].Cache.This
	}
	for i := 0; i < diskCount; i++ {
		format := formats[i]
		format.Cache.Disks = disks
	}
	return formats
}

// Creates a new cache format.json if unformatted.
func createFormatCache(fsFormatPath string, format *formatCacheV1) error {
	// open file using READ & WRITE permission
	var file, err = os.OpenFile(fsFormatPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	// Close the locked file upon return.
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return err
	}
	if fi.Size() != 0 {
		// format.json already got created because of another minio process's createFormatCache()
		return nil
	}
	return jsonSave(file, format)
}

// This function creates a cache format file on disk and returns a slice
// of format cache config
func initFormatCache(ctx context.Context, drives []string) (formats []*formatCacheV1, err error) {
	nformats := newFormatCacheV1(drives)
	for _, drive := range drives {
		_, err = os.Stat(drive)
		if err == nil {
			continue
		}
		if !os.IsNotExist(err) {
			logger.GetReqInfo(ctx).AppendTags("drive", drive)
			logger.LogIf(ctx, err)
			return nil, err
		}
		if err = os.Mkdir(drive, 0777); err != nil {
			logger.GetReqInfo(ctx).AppendTags("drive", drive)
			logger.LogIf(ctx, err)
			return nil, err
		}
	}
	for i, drive := range drives {
		if err = os.Mkdir(pathJoin(drive, minioMetaBucket), 0777); err != nil {
			if !os.IsExist(err) {
				logger.GetReqInfo(ctx).AppendTags("drive", drive)
				logger.LogIf(ctx, err)
				return nil, err
			}
		}
		cacheFormatPath := pathJoin(drive, minioMetaBucket, formatConfigFile)
		// Fresh disk - create format.json for this cfs
		if err = createFormatCache(cacheFormatPath, nformats[i]); err != nil {
			logger.GetReqInfo(ctx).AppendTags("drive", drive)
			logger.LogIf(ctx, err)
			return nil, err
		}
	}
	return nformats, nil
}

func loadFormatCache(ctx context.Context, drives []string) ([]*formatCacheV1, error) {
	formats := make([]*formatCacheV1, len(drives))
	for i, drive := range drives {
		cacheFormatPath := pathJoin(drive, minioMetaBucket, formatConfigFile)
		f, err := os.Open(cacheFormatPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			logger.LogIf(ctx, err)
			return nil, err
		}
		defer f.Close()
		format, err := formatMetaCacheV1(f)
		if err != nil {
			continue
		}
		formats[i] = format
	}
	return formats, nil
}

// unmarshalls the cache format.json into formatCacheV1
func formatMetaCacheV1(r io.ReadSeeker) (*formatCacheV1, error) {
	format := &formatCacheV1{}
	if err := jsonLoad(r, format); err != nil {
		return nil, err
	}
	return format, nil
}

func checkFormatCacheValue(format *formatCacheV1) error {
	// Validate format version and format type.
	if format.Version != formatMetaVersion1 {
		return fmt.Errorf("Unsupported version of cache format [%s] found", format.Version)
	}
	if format.Format != formatCache {
		return fmt.Errorf("Unsupported cache format [%s] found", format.Format)
	}
	if format.Cache.Version != formatCacheVersionV1 {
		return fmt.Errorf("Unsupported Cache backend format found [%s]", format.Cache.Version)
	}
	return nil
}

func checkFormatCacheValues(formats []*formatCacheV1) (int, error) {
	for i, formatCache := range formats {
		if formatCache == nil {
			continue
		}
		if err := checkFormatCacheValue(formatCache); err != nil {
			return i, err
		}
		if len(formats) != len(formatCache.Cache.Disks) {
			return i, fmt.Errorf("Expected number of cache drives %d , got  %d",
				len(formatCache.Cache.Disks), len(formats))
		}
	}
	return -1, nil
}

// checkCacheDisksConsistency - checks if "This" disk uuid on each disk is consistent with all "Disks" slices
// across disks.
func checkCacheDiskConsistency(formats []*formatCacheV1) error {
	var disks = make([]string, len(formats))
	// Collect currently available disk uuids.
	for index, format := range formats {
		if format == nil {
			disks[index] = ""
			continue
		}
		disks[index] = format.Cache.This
	}
	for i, format := range formats {
		if format == nil {
			continue
		}
		j := findCacheDiskIndex(disks[i], format.Cache.Disks)
		if j == -1 {
			return fmt.Errorf("UUID on positions %d:%d do not match with , expected %s", i, j, disks[i])
		}
		if i != j {
			return fmt.Errorf("UUID on positions %d:%d do not match with , expected %s got %s", i, j, disks[i], format.Cache.Disks[j])
		}
	}
	return nil
}

// checkCacheDisksSliceConsistency - validate cache Disks order if they are consistent.
func checkCacheDisksSliceConsistency(formats []*formatCacheV1) error {
	var sentinelDisks []string
	// Extract first valid Disks slice.
	for _, format := range formats {
		if format == nil {
			continue
		}
		sentinelDisks = format.Cache.Disks
		break
	}
	for _, format := range formats {
		if format == nil {
			continue
		}
		currentDisks := format.Cache.Disks
		if !reflect.DeepEqual(sentinelDisks, currentDisks) {
			return errors.New("inconsistent cache drives found")
		}
	}
	return nil
}

// findCacheDiskIndex returns position of cache disk in JBOD.
func findCacheDiskIndex(disk string, disks []string) int {
	for index, uuid := range disks {
		if uuid == disk {
			return index
		}
	}
	return -1
}

// validate whether cache drives order has changed
func validateCacheFormats(ctx context.Context, formats []*formatCacheV1) error {
	count := 0
	for _, format := range formats {
		if format == nil {
			count++
		}
	}
	if count == len(formats) {
		return errors.New("Cache format files missing on all drives")
	}
	if _, err := checkFormatCacheValues(formats); err != nil {
		logger.LogIf(ctx, err)
		return err
	}
	if err := checkCacheDisksSliceConsistency(formats); err != nil {
		logger.LogIf(ctx, err)
		return err
	}
	err := checkCacheDiskConsistency(formats)
	logger.LogIf(ctx, err)
	return err
}

// return true if all of the list of cache drives are
// fresh disks
func cacheDrivesUnformatted(drives []string) bool {
	count := 0
	for _, drive := range drives {
		cacheFormatPath := pathJoin(drive, minioMetaBucket, formatConfigFile)
		if _, err := os.Stat(cacheFormatPath); os.IsNotExist(err) {
			count++
		}
	}
	return count == len(drives)
}

// create format.json for each cache drive if fresh disk or load format from disk
// Then validate the format for all drives in the cache to ensure order
// of cache drives has not changed.
func loadAndValidateCacheFormat(ctx context.Context, drives []string) (formats []*formatCacheV1, err error) {
	if cacheDrivesUnformatted(drives) {
		formats, err = initFormatCache(ctx, drives)
	} else {
		formats, err = loadFormatCache(ctx, drives)
	}
	if err != nil {
		return nil, err
	}
	if err = validateCacheFormats(ctx, formats); err != nil {
		return nil, err
	}
	return formats, nil
}
