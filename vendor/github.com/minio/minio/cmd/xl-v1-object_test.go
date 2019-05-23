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
	"bytes"
	"context"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/minio/minio/pkg/madmin"
)

func TestRepeatPutObjectPart(t *testing.T) {
	var objLayer ObjectLayer
	var disks []string
	var err error
	var opts ObjectOptions

	objLayer, disks, err = prepareXL16()
	if err != nil {
		t.Fatal(err)
	}

	// cleaning up of temporary test directories
	defer removeRoots(disks)

	err = objLayer.MakeBucketWithLocation(context.Background(), "bucket1", "")
	if err != nil {
		t.Fatal(err)
	}

	uploadID, err := objLayer.NewMultipartUpload(context.Background(), "bucket1", "mpartObj1", opts)
	if err != nil {
		t.Fatal(err)
	}
	fiveMBBytes := bytes.Repeat([]byte("a"), 5*humanize.MiByte)
	md5Hex := getMD5Hash(fiveMBBytes)
	_, err = objLayer.PutObjectPart(context.Background(), "bucket1", "mpartObj1", uploadID, 1, mustGetPutObjReader(t, bytes.NewReader(fiveMBBytes), 5*humanize.MiByte, md5Hex, ""), opts)
	if err != nil {
		t.Fatal(err)
	}
	// PutObjectPart should succeed even if part already exists. ref: https://github.com/minio/minio/issues/1930
	_, err = objLayer.PutObjectPart(context.Background(), "bucket1", "mpartObj1", uploadID, 1, mustGetPutObjReader(t, bytes.NewReader(fiveMBBytes), 5*humanize.MiByte, md5Hex, ""), opts)
	if err != nil {
		t.Fatal(err)
	}
}

func TestXLDeleteObjectBasic(t *testing.T) {
	testCases := []struct {
		bucket      string
		object      string
		expectedErr error
	}{
		{".test", "dir/obj", BucketNameInvalid{Bucket: ".test"}},
		{"----", "dir/obj", BucketNameInvalid{Bucket: "----"}},
		{"bucket", "", ObjectNameInvalid{Bucket: "bucket", Object: ""}},
		{"bucket", "doesnotexist", ObjectNotFound{Bucket: "bucket", Object: "doesnotexist"}},
		{"bucket", "dir/doesnotexist", ObjectNotFound{Bucket: "bucket", Object: "dir/doesnotexist"}},
		{"bucket", "dir", ObjectNotFound{Bucket: "bucket", Object: "dir"}},
		{"bucket", "dir/", ObjectNotFound{Bucket: "bucket", Object: "dir/"}},
		{"bucket", "dir/obj", nil},
	}

	// Create an instance of xl backend
	xl, fsDirs, err := prepareXL16()
	if err != nil {
		t.Fatal(err)
	}

	err = xl.MakeBucketWithLocation(context.Background(), "bucket", "")
	if err != nil {
		t.Fatal(err)
	}

	// Create object "dir/obj" under bucket "bucket" for Test 7 to pass
	_, err = xl.PutObject(context.Background(), "bucket", "dir/obj", mustGetPutObjReader(t, bytes.NewReader([]byte("abcd")), int64(len("abcd")), "", ""), ObjectOptions{})
	if err != nil {
		t.Fatalf("XL Object upload failed: <ERROR> %s", err)
	}
	for i, test := range testCases {
		actualErr := xl.DeleteObject(context.Background(), test.bucket, test.object)
		if test.expectedErr != nil && actualErr != test.expectedErr {
			t.Errorf("Test %d: Expected to fail with %s, but failed with %s", i+1, test.expectedErr, actualErr)
		}
		if test.expectedErr == nil && actualErr != nil {
			t.Errorf("Test %d: Expected to pass, but failed with %s", i+1, actualErr)
		}
	}
	// Cleanup backend directories
	removeRoots(fsDirs)
}

func TestXLDeleteObjectsXLSet(t *testing.T) {

	var objs []*xlObjects
	for i := 0; i < 32; i++ {
		obj, fsDirs, err := prepareXL(16)
		if err != nil {
			t.Fatal("Unable to initialize 'XL' object layer.", err)
		}
		// Remove all dirs.
		for _, dir := range fsDirs {
			defer os.RemoveAll(dir)
		}
		objs = append(objs, obj.(*xlObjects))
	}

	xlSets := &xlSets{sets: objs, distributionAlgo: "CRCMOD"}

	type testCaseType struct {
		bucket string
		object string
	}

	bucketName := "bucket"
	testCases := []testCaseType{
		{bucketName, "dir/obj1"},
		{bucketName, "dir/obj2"},
		{bucketName, "obj3"},
		{bucketName, "obj_4"},
	}

	err := xlSets.MakeBucketWithLocation(context.Background(), bucketName, "")
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range testCases {
		_, err = xlSets.PutObject(context.Background(), testCase.bucket, testCase.object,
			mustGetPutObjReader(t, bytes.NewReader([]byte("abcd")), int64(len("abcd")), "", ""), ObjectOptions{})
		if err != nil {
			t.Fatalf("XL Object upload failed: <ERROR> %s", err)
		}
	}

	toObjectNames := func(testCases []testCaseType) []string {
		names := make([]string, len(testCases))
		for i := range testCases {
			names[i] = testCases[i].object
		}
		return names
	}

	objectNames := toObjectNames(testCases)
	delErrs, err := xlSets.DeleteObjects(context.Background(), bucketName, objectNames)
	if err != nil {
		t.Errorf("Failed to call DeleteObjects with the error: `%v`", err)
	}

	for i := range delErrs {
		if delErrs[i] != nil {
			t.Errorf("Failed to remove object `%v` with the error: `%v`", objectNames[i], delErrs[i])
		}
	}

	for _, test := range testCases {
		_, statErr := xlSets.GetObjectInfo(context.Background(), test.bucket, test.object, ObjectOptions{})
		switch statErr.(type) {
		case ObjectNotFound:
		default:
			t.Fatalf("Object %s is not removed", test.bucket+"/"+test.object)
		}
	}
}

func TestXLDeleteObjectDiskNotFound(t *testing.T) {
	// Reset global storage class flags
	resetGlobalStorageEnvs()
	// Create an instance of xl backend.
	obj, fsDirs, err := prepareXL16()
	if err != nil {
		t.Fatal(err)
	}

	xl := obj.(*xlObjects)

	// Create "bucket"
	err = obj.MakeBucketWithLocation(context.Background(), "bucket", "")
	if err != nil {
		t.Fatal(err)
	}

	bucket := "bucket"
	object := "object"
	opts := ObjectOptions{}
	// Create object "obj" under bucket "bucket".
	_, err = obj.PutObject(context.Background(), bucket, object, mustGetPutObjReader(t, bytes.NewReader([]byte("abcd")), int64(len("abcd")), "", ""), opts)
	if err != nil {
		t.Fatal(err)
	}
	// for a 16 disk setup, quorum is 9. To simulate disks not found yet
	// quorum is available, we remove disks leaving quorum disks behind.
	for i := range xl.storageDisks[:7] {
		xl.storageDisks[i] = newNaughtyDisk(xl.storageDisks[i], nil, errFaultyDisk)
	}
	err = obj.DeleteObject(context.Background(), bucket, object)
	if err != nil {
		t.Fatal(err)
	}

	// Create "obj" under "bucket".
	_, err = obj.PutObject(context.Background(), bucket, object, mustGetPutObjReader(t, bytes.NewReader([]byte("abcd")), int64(len("abcd")), "", ""), opts)
	if err != nil {
		t.Fatal(err)
	}

	// Remove one more disk to 'lose' quorum, by setting it to nil.
	xl.storageDisks[7] = nil
	xl.storageDisks[8] = nil
	err = obj.DeleteObject(context.Background(), bucket, object)
	// since majority of disks are not available, metaquorum is not achieved and hence errXLReadQuorum error
	if err != toObjectErr(errXLReadQuorum, bucket, object) {
		t.Errorf("Expected deleteObject to fail with %v, but failed with %v", toObjectErr(errXLReadQuorum, bucket, object), err)
	}
	// Cleanup backend directories
	removeRoots(fsDirs)
}

func TestGetObjectNoQuorum(t *testing.T) {
	// Create an instance of xl backend.
	obj, fsDirs, err := prepareXL16()
	if err != nil {
		t.Fatal(err)
	}

	xl := obj.(*xlObjects)

	// Create "bucket"
	err = obj.MakeBucketWithLocation(context.Background(), "bucket", "")
	if err != nil {
		t.Fatal(err)
	}

	bucket := "bucket"
	object := "object"
	opts := ObjectOptions{}
	// Create "object" under "bucket".
	_, err = obj.PutObject(context.Background(), bucket, object, mustGetPutObjReader(t, bytes.NewReader([]byte("abcd")), int64(len("abcd")), "", ""), opts)
	if err != nil {
		t.Fatal(err)
	}

	// Make 9 disks offline, which leaves less than quorum number of disks
	// in a 16 disk XL setup. The original disks are 'replaced' with
	// naughtyDisks that fail after 'f' successful StorageAPI method
	// invocations, where f - [0,2)
	for f := 0; f < 2; f++ {
		diskErrors := make(map[int]error)
		for i := 0; i <= f; i++ {
			diskErrors[i] = nil
		}
		for i := range xl.storageDisks[:9] {
			switch diskType := xl.storageDisks[i].(type) {
			case *naughtyDisk:
				xl.storageDisks[i] = newNaughtyDisk(diskType.disk, diskErrors, errFaultyDisk)
			default:
				xl.storageDisks[i] = newNaughtyDisk(xl.storageDisks[i], diskErrors, errFaultyDisk)
			}
		}
		// Fetch object from store.
		err = xl.GetObject(context.Background(), bucket, object, 0, int64(len("abcd")), ioutil.Discard, "", opts)
		if err != toObjectErr(errXLReadQuorum, bucket, object) {
			t.Errorf("Expected putObject to fail with %v, but failed with %v", toObjectErr(errXLWriteQuorum, bucket, object), err)
		}
	}
	// Cleanup backend directories.
	removeRoots(fsDirs)
}

func TestPutObjectNoQuorum(t *testing.T) {
	// Create an instance of xl backend.
	obj, fsDirs, err := prepareXL16()
	if err != nil {
		t.Fatal(err)
	}

	xl := obj.(*xlObjects)

	// Create "bucket"
	err = obj.MakeBucketWithLocation(context.Background(), "bucket", "")
	if err != nil {
		t.Fatal(err)
	}

	bucket := "bucket"
	object := "object"
	opts := ObjectOptions{}
	// Create "object" under "bucket".
	_, err = obj.PutObject(context.Background(), bucket, object, mustGetPutObjReader(t, bytes.NewReader([]byte("abcd")), int64(len("abcd")), "", ""), opts)
	if err != nil {
		t.Fatal(err)
	}

	// Make 9 disks offline, which leaves less than quorum number of disks
	// in a 16 disk XL setup. The original disks are 'replaced' with
	// naughtyDisks that fail after 'f' successful StorageAPI method
	// invocations, where f - [0,3)
	for f := 0; f < 3; f++ {
		diskErrors := make(map[int]error)
		for i := 0; i <= f; i++ {
			diskErrors[i] = nil
		}
		for i := range xl.storageDisks[:9] {
			switch diskType := xl.storageDisks[i].(type) {
			case *naughtyDisk:
				xl.storageDisks[i] = newNaughtyDisk(diskType.disk, diskErrors, errFaultyDisk)
			default:
				xl.storageDisks[i] = newNaughtyDisk(xl.storageDisks[i], diskErrors, errFaultyDisk)
			}
		}
		// Upload new content to same object "object"
		_, err = obj.PutObject(context.Background(), bucket, object, mustGetPutObjReader(t, bytes.NewReader([]byte("abcd")), int64(len("abcd")), "", ""), opts)
		if err != toObjectErr(errXLWriteQuorum, bucket, object) {
			t.Errorf("Expected putObject to fail with %v, but failed with %v", toObjectErr(errXLWriteQuorum, bucket, object), err)
		}
	}
	// Cleanup backend directories.
	removeRoots(fsDirs)
}

// Tests both object and bucket healing.
func TestHealing(t *testing.T) {
	obj, fsDirs, err := prepareXL16()
	if err != nil {
		t.Fatal(err)
	}
	defer removeRoots(fsDirs)
	xl := obj.(*xlObjects)

	// Create "bucket"
	err = obj.MakeBucketWithLocation(context.Background(), "bucket", "")
	if err != nil {
		t.Fatal(err)
	}

	bucket := "bucket"
	object := "object"

	data := make([]byte, 1*humanize.MiByte)
	length := int64(len(data))
	_, err = rand.Read(data)
	if err != nil {
		t.Fatal(err)
	}

	_, err = obj.PutObject(context.Background(), bucket, object, mustGetPutObjReader(t, bytes.NewReader(data), length, "", ""), ObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}

	disk := xl.storageDisks[0]
	xlMetaPreHeal, err := readXLMeta(context.Background(), disk, bucket, object)
	if err != nil {
		t.Fatal(err)
	}

	// Remove the object - to simulate the case where the disk was down when the object
	// was created.
	err = os.RemoveAll(path.Join(fsDirs[0], bucket, object))
	if err != nil {
		t.Fatal(err)
	}

	_, err = xl.HealObject(context.Background(), bucket, object, false, false, madmin.HealNormalScan)
	if err != nil {
		t.Fatal(err)
	}

	xlMetaPostHeal, err := readXLMeta(context.Background(), disk, bucket, object)
	if err != nil {
		t.Fatal(err)
	}

	// After heal the meta file should be as expected.
	if !reflect.DeepEqual(xlMetaPreHeal, xlMetaPostHeal) {
		t.Fatal("HealObject failed")
	}

	err = os.RemoveAll(path.Join(fsDirs[0], bucket, object, "xl.json"))
	if err != nil {
		t.Fatal(err)
	}

	// Write xl.json with different modtime to simulate the case where a disk had
	// gone down when an object was replaced by a new object.
	xlMetaOutDated := xlMetaPreHeal
	xlMetaOutDated.Stat.ModTime = time.Now()
	err = writeXLMetadata(context.Background(), disk, bucket, object, xlMetaOutDated)
	if err != nil {
		t.Fatal(err)
	}

	_, err = xl.HealObject(context.Background(), bucket, object, false, false, madmin.HealDeepScan)
	if err != nil {
		t.Fatal(err)
	}

	xlMetaPostHeal, err = readXLMeta(context.Background(), disk, bucket, object)
	if err != nil {
		t.Fatal(err)
	}

	// After heal the meta file should be as expected.
	if !reflect.DeepEqual(xlMetaPreHeal, xlMetaPostHeal) {
		t.Fatal("HealObject failed")
	}

	// Remove the bucket - to simulate the case where bucket was
	// created when the disk was down.
	err = os.RemoveAll(path.Join(fsDirs[0], bucket))
	if err != nil {
		t.Fatal(err)
	}
	// This would create the bucket.
	_, err = xl.HealBucket(context.Background(), bucket, false, false)
	if err != nil {
		t.Fatal(err)
	}
	// Stat the bucket to make sure that it was created.
	_, err = xl.storageDisks[0].StatVol(bucket)
	if err != nil {
		t.Fatal(err)
	}
}
