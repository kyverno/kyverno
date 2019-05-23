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
)

// Tests cleanup multipart uploads for filesystem backend.
func TestFSCleanupMultipartUploadsInRoutine(t *testing.T) {
	// Prepare for tests
	disk := filepath.Join(globalTestTmpDir, "minio-"+nextSuffix())
	defer os.RemoveAll(disk)

	obj := initFSObjects(disk, t)
	fs := obj.(*FSObjects)

	// Close the go-routine, we are going to
	// manually start it and test in this test case.
	GlobalServiceDoneCh <- struct{}{}

	bucketName := "bucket"
	objectName := "object"

	obj.MakeBucketWithLocation(context.Background(), bucketName, "")
	uploadID, err := obj.NewMultipartUpload(context.Background(), bucketName, objectName, ObjectOptions{})
	if err != nil {
		t.Fatal("Unexpected err: ", err)
	}

	go fs.cleanupStaleMultipartUploads(context.Background(), 20*time.Millisecond, 0, GlobalServiceDoneCh)

	// Wait for 40ms such that - we have given enough time for
	// cleanup routine to kick in.
	time.Sleep(40 * time.Millisecond)

	// Close the routine we do not need it anymore.
	GlobalServiceDoneCh <- struct{}{}

	// Check if upload id was already purged.
	if err = obj.AbortMultipartUpload(context.Background(), bucketName, objectName, uploadID); err != nil {
		if _, ok := err.(InvalidUploadID); !ok {
			t.Fatal("Unexpected err: ", err)
		}
	}
}

// TestNewMultipartUploadFaultyDisk - test NewMultipartUpload with faulty disks
func TestNewMultipartUploadFaultyDisk(t *testing.T) {
	// Prepare for tests
	disk := filepath.Join(globalTestTmpDir, "minio-"+nextSuffix())
	defer os.RemoveAll(disk)
	obj := initFSObjects(disk, t)

	fs := obj.(*FSObjects)
	bucketName := "bucket"
	objectName := "object"

	if err := obj.MakeBucketWithLocation(context.Background(), bucketName, ""); err != nil {
		t.Fatal("Cannot create bucket, err: ", err)
	}

	// Test with disk removed.
	os.RemoveAll(disk)
	if _, err := fs.NewMultipartUpload(context.Background(), bucketName, objectName, ObjectOptions{UserDefined: map[string]string{"X-Amz-Meta-xid": "3f"}}); err != nil {
		if !isSameType(err, BucketNotFound{}) {
			t.Fatal("Unexpected error ", err)
		}
	}
}

// TestPutObjectPartFaultyDisk - test PutObjectPart with faulty disks
func TestPutObjectPartFaultyDisk(t *testing.T) {
	// Prepare for tests
	disk := filepath.Join(globalTestTmpDir, "minio-"+nextSuffix())
	defer os.RemoveAll(disk)
	obj := initFSObjects(disk, t)

	fs := obj.(*FSObjects)
	bucketName := "bucket"
	objectName := "object"
	data := []byte("12345")
	dataLen := int64(len(data))

	if err := obj.MakeBucketWithLocation(context.Background(), bucketName, ""); err != nil {
		t.Fatal("Cannot create bucket, err: ", err)
	}

	uploadID, err := fs.NewMultipartUpload(context.Background(), bucketName, objectName, ObjectOptions{UserDefined: map[string]string{"X-Amz-Meta-xid": "3f"}})
	if err != nil {
		t.Fatal("Unexpected error ", err)
	}

	md5Hex := getMD5Hash(data)
	sha256sum := ""

	fs.fsPath = filepath.Join(globalTestTmpDir, "minio-"+nextSuffix())
	_, err = fs.PutObjectPart(context.Background(), bucketName, objectName, uploadID, 1, mustGetPutObjReader(t, bytes.NewReader(data), dataLen, md5Hex, sha256sum), ObjectOptions{})
	if !isSameType(err, BucketNotFound{}) {
		t.Fatal("Unexpected error ", err)
	}
}

// TestCompleteMultipartUploadFaultyDisk - test CompleteMultipartUpload with faulty disks
func TestCompleteMultipartUploadFaultyDisk(t *testing.T) {
	// Prepare for tests
	disk := filepath.Join(globalTestTmpDir, "minio-"+nextSuffix())
	defer os.RemoveAll(disk)
	obj := initFSObjects(disk, t)

	fs := obj.(*FSObjects)
	bucketName := "bucket"
	objectName := "object"
	data := []byte("12345")

	if err := obj.MakeBucketWithLocation(context.Background(), bucketName, ""); err != nil {
		t.Fatal("Cannot create bucket, err: ", err)
	}

	uploadID, err := fs.NewMultipartUpload(context.Background(), bucketName, objectName, ObjectOptions{UserDefined: map[string]string{"X-Amz-Meta-xid": "3f"}})
	if err != nil {
		t.Fatal("Unexpected error ", err)
	}

	md5Hex := getMD5Hash(data)

	parts := []CompletePart{{PartNumber: 1, ETag: md5Hex}}
	fs.fsPath = filepath.Join(globalTestTmpDir, "minio-"+nextSuffix())
	if _, err := fs.CompleteMultipartUpload(context.Background(), bucketName, objectName, uploadID, parts, ObjectOptions{}); err != nil {
		if !isSameType(err, BucketNotFound{}) {
			t.Fatal("Unexpected error ", err)
		}
	}
}

// TestCompleteMultipartUpload - test CompleteMultipartUpload
func TestCompleteMultipartUpload(t *testing.T) {
	// Prepare for tests
	disk := filepath.Join(globalTestTmpDir, "minio-"+nextSuffix())
	defer os.RemoveAll(disk)
	obj := initFSObjects(disk, t)

	fs := obj.(*FSObjects)
	bucketName := "bucket"
	objectName := "object"
	data := []byte("12345")

	if err := obj.MakeBucketWithLocation(context.Background(), bucketName, ""); err != nil {
		t.Fatal("Cannot create bucket, err: ", err)
	}

	uploadID, err := fs.NewMultipartUpload(context.Background(), bucketName, objectName, ObjectOptions{UserDefined: map[string]string{"X-Amz-Meta-xid": "3f"}})
	if err != nil {
		t.Fatal("Unexpected error ", err)
	}

	md5Hex := getMD5Hash(data)

	if _, err := fs.PutObjectPart(context.Background(), bucketName, objectName, uploadID, 1, mustGetPutObjReader(t, bytes.NewReader(data), 5, md5Hex, ""), ObjectOptions{}); err != nil {
		t.Fatal("Unexpected error ", err)
	}

	parts := []CompletePart{{PartNumber: 1, ETag: md5Hex}}

	if _, err := fs.CompleteMultipartUpload(context.Background(), bucketName, objectName, uploadID, parts, ObjectOptions{}); err != nil {
		t.Fatal("Unexpected error ", err)
	}
}

// TestCompleteMultipartUpload - test CompleteMultipartUpload
func TestAbortMultipartUpload(t *testing.T) {
	// Prepare for tests
	disk := filepath.Join(globalTestTmpDir, "minio-"+nextSuffix())
	defer os.RemoveAll(disk)
	obj := initFSObjects(disk, t)

	fs := obj.(*FSObjects)
	bucketName := "bucket"
	objectName := "object"
	data := []byte("12345")

	if err := obj.MakeBucketWithLocation(context.Background(), bucketName, ""); err != nil {
		t.Fatal("Cannot create bucket, err: ", err)
	}

	uploadID, err := fs.NewMultipartUpload(context.Background(), bucketName, objectName, ObjectOptions{UserDefined: map[string]string{"X-Amz-Meta-xid": "3f"}})
	if err != nil {
		t.Fatal("Unexpected error ", err)
	}

	md5Hex := getMD5Hash(data)

	if _, err := fs.PutObjectPart(context.Background(), bucketName, objectName, uploadID, 1, mustGetPutObjReader(t, bytes.NewReader(data), 5, md5Hex, ""), ObjectOptions{}); err != nil {
		t.Fatal("Unexpected error ", err)
	}
	time.Sleep(time.Second) // Without Sleep on windows, the fs.AbortMultipartUpload() fails with "The process cannot access the file because it is being used by another process."
	if err := fs.AbortMultipartUpload(context.Background(), bucketName, objectName, uploadID); err != nil {
		t.Fatal("Unexpected error ", err)
	}
}

// TestListMultipartUploadsFaultyDisk - test ListMultipartUploads with faulty disks
func TestListMultipartUploadsFaultyDisk(t *testing.T) {
	// Prepare for tests
	disk := filepath.Join(globalTestTmpDir, "minio-"+nextSuffix())
	defer os.RemoveAll(disk)

	obj := initFSObjects(disk, t)

	fs := obj.(*FSObjects)
	bucketName := "bucket"
	objectName := "object"

	if err := obj.MakeBucketWithLocation(context.Background(), bucketName, ""); err != nil {
		t.Fatal("Cannot create bucket, err: ", err)
	}

	_, err := fs.NewMultipartUpload(context.Background(), bucketName, objectName, ObjectOptions{UserDefined: map[string]string{"X-Amz-Meta-xid": "3f"}})
	if err != nil {
		t.Fatal("Unexpected error ", err)
	}

	fs.fsPath = filepath.Join(globalTestTmpDir, "minio-"+nextSuffix())
	if _, err := fs.ListMultipartUploads(context.Background(), bucketName, objectName, "", "", "", 1000); err != nil {
		if !isSameType(err, BucketNotFound{}) {
			t.Fatal("Unexpected error ", err)
		}
	}
}
