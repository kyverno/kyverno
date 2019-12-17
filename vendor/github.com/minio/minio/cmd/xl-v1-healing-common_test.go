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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/minio/minio/pkg/madmin"
)

// validates functionality provided to find most common
// time occurrence from a list of time.
func TestCommonTime(t *testing.T) {
	// List of test cases for common modTime.
	testCases := []struct {
		times []time.Time
		time  time.Time
	}{
		{
			// 1. Tests common times when slice has varying time elements.
			[]time.Time{
				time.Unix(0, 1).UTC(),
				time.Unix(0, 2).UTC(),
				time.Unix(0, 3).UTC(),
				time.Unix(0, 3).UTC(),
				time.Unix(0, 2).UTC(),
				time.Unix(0, 3).UTC(),
				time.Unix(0, 1).UTC(),
			}, time.Unix(0, 3).UTC(),
		},
		{
			// 2. Tests common time obtained when all elements are equal.
			[]time.Time{
				time.Unix(0, 3).UTC(),
				time.Unix(0, 3).UTC(),
				time.Unix(0, 3).UTC(),
				time.Unix(0, 3).UTC(),
				time.Unix(0, 3).UTC(),
				time.Unix(0, 3).UTC(),
				time.Unix(0, 3).UTC(),
			}, time.Unix(0, 3).UTC(),
		},
		{
			// 3. Tests common time obtained when elements have a mixture
			// of sentinel values.
			[]time.Time{
				time.Unix(0, 3).UTC(),
				time.Unix(0, 3).UTC(),
				time.Unix(0, 2).UTC(),
				time.Unix(0, 1).UTC(),
				time.Unix(0, 3).UTC(),
				time.Unix(0, 4).UTC(),
				time.Unix(0, 3).UTC(),
				timeSentinel,
				timeSentinel,
				timeSentinel,
			}, time.Unix(0, 3).UTC(),
		},
	}

	// Tests all the testcases, and validates them against expected
	// common modtime. Tests fail if modtime does not match.
	for i, testCase := range testCases {
		// Obtain a common mod time from modTimes slice.
		ctime, _ := commonTime(testCase.times)
		if testCase.time != ctime {
			t.Fatalf("Test case %d, expect to pass but failed. Wanted modTime: %s, got modTime: %s\n", i+1, testCase.time, ctime)
		}
	}
}

// TestListOnlineDisks - checks if listOnlineDisks and outDatedDisks
// are consistent with each other.
func TestListOnlineDisks(t *testing.T) {
	obj, disks, err := prepareXL16()
	if err != nil {
		t.Fatalf("Prepare XL backend failed - %v", err)
	}
	defer removeRoots(disks)

	type tamperKind int
	const (
		noTamper    tamperKind = iota
		deletePart  tamperKind = iota
		corruptPart tamperKind = iota
	)
	threeNanoSecs := time.Unix(0, 3).UTC()
	fourNanoSecs := time.Unix(0, 4).UTC()
	modTimesThreeNone := []time.Time{
		threeNanoSecs, threeNanoSecs, threeNanoSecs, threeNanoSecs,
		threeNanoSecs, threeNanoSecs, threeNanoSecs,
		timeSentinel, timeSentinel, timeSentinel, timeSentinel,
		timeSentinel, timeSentinel, timeSentinel, timeSentinel,
		timeSentinel,
	}
	modTimesThreeFour := []time.Time{
		threeNanoSecs, threeNanoSecs, threeNanoSecs, threeNanoSecs,
		threeNanoSecs, threeNanoSecs, threeNanoSecs, threeNanoSecs,
		fourNanoSecs, fourNanoSecs, fourNanoSecs, fourNanoSecs,
		fourNanoSecs, fourNanoSecs, fourNanoSecs, fourNanoSecs,
	}
	testCases := []struct {
		modTimes       []time.Time
		expectedTime   time.Time
		errs           []error
		_tamperBackend tamperKind
	}{
		{
			modTimes:     modTimesThreeFour,
			expectedTime: fourNanoSecs,
			errs: []error{
				nil, nil, nil, nil, nil, nil, nil, nil, nil,
				nil, nil, nil, nil, nil, nil, nil,
			},
			_tamperBackend: noTamper,
		},
		{
			modTimes:     modTimesThreeNone,
			expectedTime: threeNanoSecs,
			errs: []error{
				// Disks that have a valid xl.json.
				nil, nil, nil, nil, nil, nil, nil,
				// Majority of disks don't have xl.json.
				errFileNotFound, errFileNotFound,
				errFileNotFound, errFileNotFound,
				errFileNotFound, errDiskAccessDenied,
				errDiskNotFound, errFileNotFound,
				errFileNotFound,
			},
			_tamperBackend: deletePart,
		},
		{
			modTimes:     modTimesThreeNone,
			expectedTime: threeNanoSecs,
			errs: []error{
				// Disks that have a valid xl.json.
				nil, nil, nil, nil, nil, nil, nil,
				// Majority of disks don't have xl.json.
				errFileNotFound, errFileNotFound,
				errFileNotFound, errFileNotFound,
				errFileNotFound, errDiskAccessDenied,
				errDiskNotFound, errFileNotFound,
				errFileNotFound,
			},
			_tamperBackend: corruptPart,
		},
	}

	bucket := "bucket"
	object := "object"
	data := bytes.Repeat([]byte("a"), 1024)
	z := obj.(*xlZones)
	xlDisks := z.zones[0].sets[0].getDisks()
	for i, test := range testCases {
		// Prepare bucket/object backend for the tests below.

		// Cleanup from previous test.
		obj.DeleteObject(context.Background(), bucket, object)
		obj.DeleteBucket(context.Background(), bucket)

		err = obj.MakeBucketWithLocation(context.Background(), "bucket", "")
		if err != nil {
			t.Fatalf("Failed to make a bucket %v", err)
		}

		_, err = obj.PutObject(context.Background(), bucket, object, mustGetPutObjReader(t, bytes.NewReader(data), int64(len(data)), "", ""), ObjectOptions{})
		if err != nil {
			t.Fatalf("Failed to putObject %v", err)
		}

		tamperedIndex := -1
		switch test._tamperBackend {
		case deletePart:
			for index, err := range test.errs {
				if err != nil {
					continue
				}
				// Remove a part from a disk
				// which has a valid xl.json,
				// and check if that disk
				// appears in outDatedDisks.
				tamperedIndex = index
				dErr := xlDisks[index].DeleteFile(bucket, filepath.Join(object, "part.1"))
				if dErr != nil {
					t.Fatalf("Test %d: Failed to delete %s - %v", i+1,
						filepath.Join(object, "part.1"), dErr)
				}
				break
			}
		case corruptPart:
			for index, err := range test.errs {
				if err != nil {
					continue
				}
				// Corrupt a part from a disk
				// which has a valid xl.json,
				// and check if that disk
				// appears in outDatedDisks.
				tamperedIndex = index
				filePath := pathJoin(xlDisks[index].String(), bucket, object, "part.1")
				f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_SYNC, 0)
				if err != nil {
					t.Fatalf("Failed to open %s: %s\n", filePath, err)
				}
				f.Write([]byte("oops")) // Will cause bitrot error
				f.Close()
				break
			}

		}

		partsMetadata, errs := readAllXLMetadata(context.Background(), xlDisks, bucket, object)
		for i := range partsMetadata {
			if errs[i] != nil {
				t.Fatalf("Test %d: expected error to be nil: %s", i+1, errs[i].Error())
			}
			partsMetadata[i].Stat.ModTime = test.modTimes[i]
		}

		onlineDisks, modTime := listOnlineDisks(xlDisks, partsMetadata, test.errs)
		if !modTime.Equal(test.expectedTime) {
			t.Fatalf("Test %d: Expected modTime to be equal to %v but was found to be %v",
				i+1, test.expectedTime, modTime)
		}

		availableDisks, newErrs := disksWithAllParts(context.Background(), onlineDisks, partsMetadata, test.errs, bucket, object, madmin.HealDeepScan)
		test.errs = newErrs

		if test._tamperBackend != noTamper {
			if tamperedIndex != -1 && availableDisks[tamperedIndex] != nil {
				t.Fatalf("Test %d: disk (%v) with part.1 missing is not a disk with available data",
					i+1, xlDisks[tamperedIndex])
			}
		}

	}
}

func TestDisksWithAllParts(t *testing.T) {
	ctx := context.Background()
	obj, disks, err := prepareXL16()
	if err != nil {
		t.Fatalf("Prepare XL backend failed - %v", err)
	}
	defer removeRoots(disks)

	bucket := "bucket"
	object := "object"
	// make data with more than one part
	partCount := 3
	data := bytes.Repeat([]byte("a"), 6*1024*1024*partCount)
	z := obj.(*xlZones)
	xl := z.zones[0].sets[0]
	xlDisks := xl.getDisks()
	err = obj.MakeBucketWithLocation(ctx, "bucket", "")
	if err != nil {
		t.Fatalf("Failed to make a bucket %v", err)
	}

	_, err = obj.PutObject(ctx, bucket, object, mustGetPutObjReader(t, bytes.NewReader(data), int64(len(data)), "", ""), ObjectOptions{})
	if err != nil {
		t.Fatalf("Failed to putObject %v", err)
	}

	_, errs := readAllXLMetadata(ctx, xlDisks, bucket, object)
	readQuorum := len(xlDisks) / 2
	if reducedErr := reduceReadQuorumErrs(ctx, errs, objectOpIgnoredErrs, readQuorum); reducedErr != nil {
		t.Fatalf("Failed to read xl meta data %v", reducedErr)
	}

	// Test that all disks are returned without any failures with
	// unmodified meta data
	partsMetadata, errs := readAllXLMetadata(ctx, xlDisks, bucket, object)
	if err != nil {
		t.Fatalf("Failed to read xl meta data %v", err)
	}

	filteredDisks, errs := disksWithAllParts(ctx, xlDisks, partsMetadata, errs, bucket, object, madmin.HealDeepScan)

	if len(filteredDisks) != len(xlDisks) {
		t.Errorf("Unexpected number of disks: %d", len(filteredDisks))
	}

	for diskIndex, disk := range filteredDisks {
		if errs[diskIndex] != nil {
			t.Errorf("Unexpected error %s", errs[diskIndex])
		}

		if disk == nil {
			t.Errorf("Disk erroneously filtered, diskIndex: %d", diskIndex)
		}
	}

	diskFailures := make(map[int]string)
	// key = disk index, value = part name with hash mismatch
	diskFailures[0] = "part.1"
	diskFailures[3] = "part.1"
	diskFailures[15] = "part.1"

	for diskIndex, partName := range diskFailures {
		for _, info := range partsMetadata[diskIndex].Erasure.Checksums {
			if info.Name == partName {
				filePath := pathJoin(xlDisks[diskIndex].String(), bucket, object, partName)
				f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_SYNC, 0)
				if err != nil {
					t.Fatalf("Failed to open %s: %s\n", filePath, err)
				}
				f.Write([]byte("oops")) // Will cause bitrot error
				f.Close()
			}
		}
	}

	errs = make([]error, len(xlDisks))
	filteredDisks, errs = disksWithAllParts(ctx, xlDisks, partsMetadata, errs, bucket, object, madmin.HealDeepScan)

	if len(filteredDisks) != len(xlDisks) {
		t.Errorf("Unexpected number of disks: %d", len(filteredDisks))
	}

	for diskIndex, disk := range filteredDisks {
		if _, ok := diskFailures[diskIndex]; ok {
			if disk != nil {
				t.Errorf("Disk not filtered as expected, disk: %d", diskIndex)
			}
			if errs[diskIndex] == nil {
				t.Errorf("Expected error not received, diskIndex: %d", diskIndex)
			}
		} else {
			if disk == nil {
				t.Errorf("Disk erroneously filtered, diskIndex: %d", diskIndex)
			}
			if errs[diskIndex] != nil {
				t.Errorf("Unexpected error, %s, diskIndex: %d", errs[diskIndex], diskIndex)
			}

		}
	}
}
