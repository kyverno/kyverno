// +build linux

/*
 * MinIO Cloud Storage, (C) 2017 MinIO, Inc.
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

package mountinfo

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests cross device mount verification function, for both failure
// and success cases.
func TestCrossDeviceMountPaths(t *testing.T) {
	successCase :=
		`/dev/0 /path/to/0/1 type0 flags 0 0
		/dev/1    /path/to/1   type1	flags 1 1
		/dev/2 /path/to/1/2 type2 flags,1,2=3 2 2
                /dev/3 /path/to/1.1 type3 falgs,1,2=3 3 3
		`
	dir, err := ioutil.TempDir("", "TestReadProcmountInfos")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	mountsPath := filepath.Join(dir, "mounts")
	if err = ioutil.WriteFile(mountsPath, []byte(successCase), 0666); err != nil {
		t.Fatal(err)
	}
	// Failure case where we detected successfully cross device mounts.
	{
		var absPaths = []string{"/path/to/1"}
		if err = checkCrossDevice(absPaths, mountsPath); err == nil {
			t.Fatal("Expected to fail, but found success")
		}

		mp := []mountInfo{
			{"/dev/2", "/path/to/1/2", "type2", []string{"flags"}, "2", "2"},
		}
		msg := fmt.Sprintf("Cross-device mounts detected on path (/path/to/1) at following locations %s. Export path should not have any sub-mounts, refusing to start.", mp)
		if err.Error() != msg {
			t.Fatalf("Expected msg %s, got %s", msg, err)
		}
	}
	// Failure case when input path is not absolute.
	{
		var absPaths = []string{"."}
		if err = checkCrossDevice(absPaths, mountsPath); err == nil {
			t.Fatal("Expected to fail for non absolute paths")
		}
		expectedErrMsg := fmt.Sprintf("Invalid argument, path (%s) is expected to be absolute", ".")
		if err.Error() != expectedErrMsg {
			t.Fatalf("Expected %s, got %s", expectedErrMsg, err)
		}
	}
	// Success case, where path doesn't have any mounts.
	{
		var absPaths = []string{"/path/to/x"}
		if err = checkCrossDevice(absPaths, mountsPath); err != nil {
			t.Fatalf("Expected success, failed instead (%s)", err)
		}
	}
}

// Tests cross device mount verification function, for both failure
// and success cases.
func TestCrossDeviceMount(t *testing.T) {
	successCase :=
		`/dev/0 /path/to/0/1 type0 flags 0 0
		/dev/1    /path/to/1   type1	flags 1 1
		/dev/2 /path/to/1/2 type2 flags,1,2=3 2 2
                /dev/3 /path/to/1.1 type3 falgs,1,2=3 3 3
		`
	dir, err := ioutil.TempDir("", "TestReadProcmountInfos")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	mountsPath := filepath.Join(dir, "mounts")
	if err = ioutil.WriteFile(mountsPath, []byte(successCase), 0666); err != nil {
		t.Fatal(err)
	}
	mounts, err := readProcMounts(mountsPath)
	if err != nil {
		t.Fatal(err)
	}
	// Failure case where we detected successfully cross device mounts.
	{
		if err = mounts.checkCrossMounts("/path/to/1"); err == nil {
			t.Fatal("Expected to fail, but found success")
		}

		mp := []mountInfo{
			{"/dev/2", "/path/to/1/2", "type2", []string{"flags"}, "2", "2"},
		}
		msg := fmt.Sprintf("Cross-device mounts detected on path (/path/to/1) at following locations %s. Export path should not have any sub-mounts, refusing to start.", mp)
		if err.Error() != msg {
			t.Fatalf("Expected msg %s, got %s", msg, err)
		}
	}
	// Failure case when input path is not absolute.
	{
		if err = mounts.checkCrossMounts("."); err == nil {
			t.Fatal("Expected to fail for non absolute paths")
		}
		expectedErrMsg := fmt.Sprintf("Invalid argument, path (%s) is expected to be absolute", ".")
		if err.Error() != expectedErrMsg {
			t.Fatalf("Expected %s, got %s", expectedErrMsg, err)
		}
	}
	// Success case, where path doesn't have any mounts.
	{
		if err = mounts.checkCrossMounts("/path/to/x"); err != nil {
			t.Fatalf("Expected success, failed instead (%s)", err)
		}
	}
}

// Tests read proc mounts file.
func TestReadProcmountInfos(t *testing.T) {
	successCase :=
		`/dev/0 /path/to/0 type0 flags 0 0
		/dev/1    /path/to/1   type1	flags 1 1
		/dev/2 /path/to/2 type2 flags,1,2=3 2 2
		`
	dir, err := ioutil.TempDir("", "TestReadProcmountInfos")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	mountsPath := filepath.Join(dir, "mounts")
	if err = ioutil.WriteFile(mountsPath, []byte(successCase), 0666); err != nil {
		t.Fatal(err)
	}
	// Verifies if reading each line worked properly.
	{
		var mounts mountInfos
		mounts, err = readProcMounts(mountsPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(mounts) != 3 {
			t.Fatalf("expected 3 mounts, got %d", len(mounts))
		}
		mp := mountInfo{"/dev/0", "/path/to/0", "type0", []string{"flags"}, "0", "0"}
		if !mountPointsEqual(mounts[0], mp) {
			t.Errorf("got unexpected MountPoint[0]: %#v", mounts[0])
		}
		mp = mountInfo{"/dev/1", "/path/to/1", "type1", []string{"flags"}, "1", "1"}
		if !mountPointsEqual(mounts[1], mp) {
			t.Errorf("got unexpected mountInfo[1]: %#v", mounts[1])
		}
		mp = mountInfo{"/dev/2", "/path/to/2", "type2", []string{"flags", "1", "2=3"}, "2", "2"}
		if !mountPointsEqual(mounts[2], mp) {
			t.Errorf("got unexpected mountInfo[2]: %#v", mounts[2])
		}
	}
	// Failure case mounts path doesn't exist, if not fail.
	{
		if _, err = readProcMounts(filepath.Join(dir, "non-existent")); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

// Tests read proc mounts reader.
func TestReadProcMountFrom(t *testing.T) {
	successCase :=
		`/dev/0 /path/to/0 type0 flags 0 0
		/dev/1    /path/to/1   type1	flags 1 1
		/dev/2 /path/to/2 type2 flags,1,2=3 2 2
		`
	// Success case, verifies if parsing works properly.
	{
		mounts, err := parseMountFrom(strings.NewReader(successCase))
		if err != nil {
			t.Errorf("expected success")
		}
		if len(mounts) != 3 {
			t.Fatalf("expected 3 mounts, got %d", len(mounts))
		}
		mp := mountInfo{"/dev/0", "/path/to/0", "type0", []string{"flags"}, "0", "0"}
		if !mountPointsEqual(mounts[0], mp) {
			t.Errorf("got unexpected mountInfo[0]: %#v", mounts[0])
		}
		mp = mountInfo{"/dev/1", "/path/to/1", "type1", []string{"flags"}, "1", "1"}
		if !mountPointsEqual(mounts[1], mp) {
			t.Errorf("got unexpected mountInfo[1]: %#v", mounts[1])
		}
		mp = mountInfo{"/dev/2", "/path/to/2", "type2", []string{"flags", "1", "2=3"}, "2", "2"}
		if !mountPointsEqual(mounts[2], mp) {
			t.Errorf("got unexpected mountInfo[2]: %#v", mounts[2])
		}
	}
	// Error cases where parsing fails with invalid Freq and Pass params.
	{
		errorCases := []string{
			"/dev/0 /path/to/mount\n",
			"/dev/1 /path/to/mount type flags a 0\n",
			"/dev/2 /path/to/mount type flags 0 b\n",
		}
		for _, ec := range errorCases {
			_, rerr := parseMountFrom(strings.NewReader(ec))
			if rerr == nil {
				t.Errorf("expected error")
			}
		}
	}
}

// Helpers for tests.

// Check if two `mountInfo` are equal.
func mountPointsEqual(a, b mountInfo) bool {
	if a.Device != b.Device || a.Path != b.Path || a.FSType != b.FSType || !slicesEqual(a.Options, b.Options) || a.Pass != b.Pass || a.Freq != b.Freq {
		return false
	}
	return true
}

// Checks if two string slices are equal.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
