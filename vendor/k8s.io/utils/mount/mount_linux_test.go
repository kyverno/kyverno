// +build linux

/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mount

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

func TestReadProcMountsFrom(t *testing.T) {
	successCase :=
		`/dev/0 /path/to/0 type0 flags 0 0
/dev/1    /path/to/1   type1	flags 1 1
/dev/2 /path/to/2 type2 flags,1,2=3 2 2
`
	// NOTE: readProcMountsFrom has been updated to using fnv.New32a()
	mounts, err := parseProcMounts([]byte(successCase))
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}
	if len(mounts) != 3 {
		t.Fatalf("expected 3 mounts, got %d", len(mounts))
	}
	mp := MountPoint{"/dev/0", "/path/to/0", "type0", []string{"flags"}, 0, 0}
	if !mountPointsEqual(&mounts[0], &mp) {
		t.Errorf("got unexpected MountPoint[0]: %#v", mounts[0])
	}
	mp = MountPoint{"/dev/1", "/path/to/1", "type1", []string{"flags"}, 1, 1}
	if !mountPointsEqual(&mounts[1], &mp) {
		t.Errorf("got unexpected MountPoint[1]: %#v", mounts[1])
	}
	mp = MountPoint{"/dev/2", "/path/to/2", "type2", []string{"flags", "1", "2=3"}, 2, 2}
	if !mountPointsEqual(&mounts[2], &mp) {
		t.Errorf("got unexpected MountPoint[2]: %#v", mounts[2])
	}

	errorCases := []string{
		"/dev/0 /path/to/mount\n",
		"/dev/1 /path/to/mount type flags a 0\n",
		"/dev/2 /path/to/mount type flags 0 b\n",
	}
	for _, ec := range errorCases {
		_, err := parseProcMounts([]byte(ec))
		if err == nil {
			t.Errorf("expected error")
		}
	}
}

func mountPointsEqual(a, b *MountPoint) bool {
	if a.Device != b.Device || a.Path != b.Path || a.Type != b.Type || !reflect.DeepEqual(a.Opts, b.Opts) || a.Pass != b.Pass || a.Freq != b.Freq {
		return false
	}
	return true
}

func TestGetMountRefs(t *testing.T) {
	fm := NewFakeMounter(
		[]MountPoint{
			{Device: "/dev/sdb", Path: "/var/lib/kubelet/plugins/kubernetes.io/gce-pd/mounts/gce-pd"},
			{Device: "/dev/sdb", Path: "/var/lib/kubelet/pods/some-pod/volumes/kubernetes.io~gce-pd/gce-pd-in-pod"},
			{Device: "/dev/sdc", Path: "/var/lib/kubelet/plugins/kubernetes.io/gce-pd/mounts/gce-pd2"},
			{Device: "/dev/sdc", Path: "/var/lib/kubelet/pods/some-pod/volumes/kubernetes.io~gce-pd/gce-pd2-in-pod1"},
			{Device: "/dev/sdc", Path: "/var/lib/kubelet/pods/some-pod/volumes/kubernetes.io~gce-pd/gce-pd2-in-pod2"},
		})

	tests := []struct {
		mountPath    string
		expectedRefs []string
	}{
		{
			"/var/lib/kubelet/pods/some-pod/volumes/kubernetes.io~gce-pd/gce-pd-in-pod",
			[]string{
				"/var/lib/kubelet/plugins/kubernetes.io/gce-pd/mounts/gce-pd",
			},
		},
		{
			"/var/lib/kubelet/pods/some-pod/volumes/kubernetes.io~gce-pd/gce-pd2-in-pod1",
			[]string{
				"/var/lib/kubelet/pods/some-pod/volumes/kubernetes.io~gce-pd/gce-pd2-in-pod2",
				"/var/lib/kubelet/plugins/kubernetes.io/gce-pd/mounts/gce-pd2",
			},
		},
		{
			"/var/fake/directory/that/doesnt/exist",
			[]string{},
		},
	}

	for i, test := range tests {
		if refs, err := fm.GetMountRefs(test.mountPath); err != nil || !setEquivalent(test.expectedRefs, refs) {
			t.Errorf("%d. getMountRefs(%q) = %v, %v; expected %v, nil", i, test.mountPath, refs, err, test.expectedRefs)
		}
	}
}

func setEquivalent(set1, set2 []string) bool {
	map1 := make(map[string]bool)
	map2 := make(map[string]bool)
	for _, s := range set1 {
		map1[s] = true
	}
	for _, s := range set2 {
		map2[s] = true
	}

	for s := range map1 {
		if !map2[s] {
			return false
		}
	}
	for s := range map2 {
		if !map1[s] {
			return false
		}
	}
	return true
}

func TestGetDeviceNameFromMount(t *testing.T) {
	fm := NewFakeMounter(
		[]MountPoint{
			{Device: "/dev/disk/by-path/prefix-lun-1",
				Path: "/mnt/111"},
			{Device: "/dev/disk/by-path/prefix-lun-1",
				Path: "/mnt/222"},
		})

	tests := []struct {
		mountPath      string
		expectedDevice string
		expectedRefs   int
	}{
		{
			"/mnt/222",
			"/dev/disk/by-path/prefix-lun-1",
			2,
		},
	}

	for i, test := range tests {
		if device, refs, err := GetDeviceNameFromMount(fm, test.mountPath); err != nil || test.expectedRefs != refs || test.expectedDevice != device {
			t.Errorf("%d. GetDeviceNameFromMount(%s) = (%s, %d), %v; expected (%s,%d), nil", i, test.mountPath, device, refs, err, test.expectedDevice, test.expectedRefs)
		}
	}
}

func TestGetMountRefsByDev(t *testing.T) {
	fm := NewFakeMounter(
		[]MountPoint{
			{Device: "/dev/sdb", Path: "/var/lib/kubelet/plugins/kubernetes.io/gce-pd/mounts/gce-pd"},
			{Device: "/dev/sdb", Path: "/var/lib/kubelet/pods/some-pod/volumes/kubernetes.io~gce-pd/gce-pd-in-pod"},
			{Device: "/dev/sdc", Path: "/var/lib/kubelet/plugins/kubernetes.io/gce-pd/mounts/gce-pd2"},
			{Device: "/dev/sdc", Path: "/var/lib/kubelet/pods/some-pod/volumes/kubernetes.io~gce-pd/gce-pd2-in-pod1"},
			{Device: "/dev/sdc", Path: "/var/lib/kubelet/pods/some-pod/volumes/kubernetes.io~gce-pd/gce-pd2-in-pod2"},
		})

	tests := []struct {
		mountPath    string
		expectedRefs []string
	}{
		{
			"/var/lib/kubelet/plugins/kubernetes.io/gce-pd/mounts/gce-pd",
			[]string{
				"/var/lib/kubelet/pods/some-pod/volumes/kubernetes.io~gce-pd/gce-pd-in-pod",
			},
		},
		{
			"/var/lib/kubelet/plugins/kubernetes.io/gce-pd/mounts/gce-pd2",
			[]string{
				"/var/lib/kubelet/pods/some-pod/volumes/kubernetes.io~gce-pd/gce-pd2-in-pod1",
				"/var/lib/kubelet/pods/some-pod/volumes/kubernetes.io~gce-pd/gce-pd2-in-pod2",
			},
		},
	}

	for i, test := range tests {

		if refs, err := getMountRefsByDev(fm, test.mountPath); err != nil || !setEquivalent(test.expectedRefs, refs) {
			t.Errorf("%d. getMountRefsByDev(%q) = %v, %v; expected %v, nil", i, test.mountPath, refs, err, test.expectedRefs)
		}
	}
}

func TestPathWithinBase(t *testing.T) {
	tests := []struct {
		name     string
		fullPath string
		basePath string
		expected bool
	}{
		{
			name:     "good subpath",
			fullPath: "/a/b/c",
			basePath: "/a",
			expected: true,
		},
		{
			name:     "good subpath 2",
			fullPath: "/a/b/c",
			basePath: "/a/b",
			expected: true,
		},
		{
			name:     "good subpath end slash",
			fullPath: "/a/b/c/",
			basePath: "/a/b",
			expected: true,
		},
		{
			name:     "good subpath backticks",
			fullPath: "/a/b/../c",
			basePath: "/a",
			expected: true,
		},
		{
			name:     "good subpath equal",
			fullPath: "/a/b/c",
			basePath: "/a/b/c",
			expected: true,
		},
		{
			name:     "good subpath equal 2",
			fullPath: "/a/b/c/",
			basePath: "/a/b/c",
			expected: true,
		},
		{
			name:     "good subpath root",
			fullPath: "/a",
			basePath: "/",
			expected: true,
		},
		{
			name:     "bad subpath parent",
			fullPath: "/a/b/c",
			basePath: "/a/b/c/d",
			expected: false,
		},
		{
			name:     "bad subpath outside",
			fullPath: "/b/c",
			basePath: "/a/b/c",
			expected: false,
		},
		{
			name:     "bad subpath prefix",
			fullPath: "/a/b/cd",
			basePath: "/a/b/c",
			expected: false,
		},
		{
			name:     "bad subpath backticks",
			fullPath: "/a/../b",
			basePath: "/a",
			expected: false,
		},
		{
			name:     "configmap subpath",
			fullPath: "/var/lib/kubelet/pods/uuid/volumes/kubernetes.io~configmap/config/..timestamp/file.txt",
			basePath: "/var/lib/kubelet/pods/uuid/volumes/kubernetes.io~configmap/config",
			expected: true,
		},
	}
	for _, test := range tests {
		if PathWithinBase(test.fullPath, test.basePath) != test.expected {
			t.Errorf("test %q failed: expected %v", test.name, test.expected)
		}

	}
}

func TestSearchMountPoints(t *testing.T) {
	base := `
19 25 0:18 / /sys rw,nosuid,nodev,noexec,relatime shared:7 - sysfs sysfs rw
20 25 0:4 / /proc rw,nosuid,nodev,noexec,relatime shared:12 - proc proc rw
21 25 0:6 / /dev rw,nosuid,relatime shared:2 - devtmpfs udev rw,size=4058156k,nr_inodes=1014539,mode=755
22 21 0:14 / /dev/pts rw,nosuid,noexec,relatime shared:3 - devpts devpts rw,gid=5,mode=620,ptmxmode=000
23 25 0:19 / /run rw,nosuid,noexec,relatime shared:5 - tmpfs tmpfs rw,size=815692k,mode=755
25 0 252:0 / / rw,relatime shared:1 - ext4 /dev/mapper/ubuntu--vg-root rw,errors=remount-ro,data=ordered
26 19 0:12 / /sys/kernel/security rw,nosuid,nodev,noexec,relatime shared:8 - securityfs securityfs rw
27 21 0:21 / /dev/shm rw,nosuid,nodev shared:4 - tmpfs tmpfs rw
28 23 0:22 / /run/lock rw,nosuid,nodev,noexec,relatime shared:6 - tmpfs tmpfs rw,size=5120k
29 19 0:23 / /sys/fs/cgroup ro,nosuid,nodev,noexec shared:9 - tmpfs tmpfs ro,mode=755
30 29 0:24 / /sys/fs/cgroup/systemd rw,nosuid,nodev,noexec,relatime shared:10 - cgroup cgroup rw,xattr,release_agent=/lib/systemd/systemd-cgroups-agent,name=systemd
31 19 0:25 / /sys/fs/pstore rw,nosuid,nodev,noexec,relatime shared:11 - pstore pstore rw
32 29 0:26 / /sys/fs/cgroup/devices rw,nosuid,nodev,noexec,relatime shared:13 - cgroup cgroup rw,devices
33 29 0:27 / /sys/fs/cgroup/freezer rw,nosuid,nodev,noexec,relatime shared:14 - cgroup cgroup rw,freezer
34 29 0:28 / /sys/fs/cgroup/pids rw,nosuid,nodev,noexec,relatime shared:15 - cgroup cgroup rw,pids
35 29 0:29 / /sys/fs/cgroup/blkio rw,nosuid,nodev,noexec,relatime shared:16 - cgroup cgroup rw,blkio
36 29 0:30 / /sys/fs/cgroup/memory rw,nosuid,nodev,noexec,relatime shared:17 - cgroup cgroup rw,memory
37 29 0:31 / /sys/fs/cgroup/perf_event rw,nosuid,nodev,noexec,relatime shared:18 - cgroup cgroup rw,perf_event
38 29 0:32 / /sys/fs/cgroup/hugetlb rw,nosuid,nodev,noexec,relatime shared:19 - cgroup cgroup rw,hugetlb
39 29 0:33 / /sys/fs/cgroup/cpu,cpuacct rw,nosuid,nodev,noexec,relatime shared:20 - cgroup cgroup rw,cpu,cpuacct
40 29 0:34 / /sys/fs/cgroup/cpuset rw,nosuid,nodev,noexec,relatime shared:21 - cgroup cgroup rw,cpuset
41 29 0:35 / /sys/fs/cgroup/net_cls,net_prio rw,nosuid,nodev,noexec,relatime shared:22 - cgroup cgroup rw,net_cls,net_prio
58 25 7:1 / /mnt/disks/blkvol1 rw,relatime shared:38 - ext4 /dev/loop1 rw,data=ordere
`

	testcases := []struct {
		name         string
		source       string
		mountInfos   string
		expectedRefs []string
		expectedErr  error
	}{
		{
			"dir",
			"/mnt/disks/vol1",
			base,
			nil,
			nil,
		},
		{
			"dir-used",
			"/mnt/disks/vol1",
			base + `
56 25 252:0 /mnt/disks/vol1 /var/lib/kubelet/pods/1890aef5-5a60-11e8-962f-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-test rw,relatime shared:1 - ext4 /dev/mapper/ubuntu--vg-root rw,errors=remount-ro,data=ordered
57 25 0:45 / /mnt/disks/vol rw,relatime shared:36 - tmpfs tmpfs rw
`,
			[]string{"/var/lib/kubelet/pods/1890aef5-5a60-11e8-962f-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-test"},
			nil,
		},
		{
			"tmpfs-vol",
			"/mnt/disks/vol1",
			base + `120 25 0:76 / /mnt/disks/vol1 rw,relatime shared:41 - tmpfs vol1 rw,size=10000k
`,
			nil,
			nil,
		},
		{
			"tmpfs-vol-used-by-two-pods",
			"/mnt/disks/vol1",
			base + `120 25 0:76 / /mnt/disks/vol1 rw,relatime shared:41 - tmpfs vol1 rw,size=10000k
196 25 0:76 / /var/lib/kubelet/pods/ade3ac21-5a5b-11e8-8559-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-8f263585 rw,relatime shared:41 - tmpfs vol1 rw,size=10000k
228 25 0:76 / /var/lib/kubelet/pods/ac60532d-5a5b-11e8-8559-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-8f263585 rw,relatime shared:41 - tmpfs vol1 rw,size=10000k
`,
			[]string{
				"/var/lib/kubelet/pods/ade3ac21-5a5b-11e8-8559-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-8f263585",
				"/var/lib/kubelet/pods/ac60532d-5a5b-11e8-8559-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-8f263585",
			},
			nil,
		},
		{
			"tmpfs-subdir-used-indirectly-via-bindmount-dir-by-one-pod",
			"/mnt/vol1/foo",
			base + `177 25 0:46 / /mnt/data rw,relatime shared:37 - tmpfs data rw
190 25 0:46 /vol1 /mnt/vol1 rw,relatime shared:37 - tmpfs data rw
191 25 0:46 /vol2 /mnt/vol2 rw,relatime shared:37 - tmpfs data rw
62 25 0:46 /vol1/foo /var/lib/kubelet/pods/e25f2f01-5b06-11e8-8694-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-test rw,relatime shared:37 - tmpfs data rw
`,
			[]string{"/var/lib/kubelet/pods/e25f2f01-5b06-11e8-8694-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-test"},
			nil,
		},
		{
			"dir-bindmounted",
			"/mnt/disks/vol2",
			base + `342 25 252:0 /mnt/disks/vol2 /mnt/disks/vol2 rw,relatime shared:1 - ext4 /dev/mapper/ubuntu--vg-root rw,errors=remount-ro,data=ordered
`,
			nil,
			nil,
		},
		{
			"dir-bindmounted-used-by-one-pod",
			"/mnt/disks/vol2",
			base + `342 25 252:0 /mnt/disks/vol2 /mnt/disks/vol2 rw,relatime shared:1 - ext4 /dev/mapper/ubuntu--vg-root rw,errors=remount-ro,data=ordered
77 25 252:0 /mnt/disks/vol2 /var/lib/kubelet/pods/f30dc360-5a5d-11e8-962f-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-1fb30a1c rw,relatime shared:1 - ext4 /dev/mapper/ubuntu--vg-root rw,errors=remount-ro,data=ordered
`,
			[]string{"/var/lib/kubelet/pods/f30dc360-5a5d-11e8-962f-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-1fb30a1c"},
			nil,
		},
		{
			"blockfs",
			"/mnt/disks/blkvol1",
			base + `58 25 7:1 / /mnt/disks/blkvol1 rw,relatime shared:38 - ext4 /dev/loop1 rw,data=ordered
`,
			nil,
			nil,
		},
		{
			"blockfs-used-by-one-pod",
			"/mnt/disks/blkvol1",
			base + `58 25 7:1 / /mnt/disks/blkvol1 rw,relatime shared:38 - ext4 /dev/loop1 rw,data=ordered
62 25 7:1 / /var/lib/kubelet/pods/f19fe4e2-5a63-11e8-962f-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-test rw,relatime shared:38 - ext4 /dev/loop1 rw,data=ordered
`,
			[]string{"/var/lib/kubelet/pods/f19fe4e2-5a63-11e8-962f-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-test"},
			nil,
		},
		{
			"blockfs-used-by-two-pods",
			"/mnt/disks/blkvol1",
			base + `58 25 7:1 / /mnt/disks/blkvol1 rw,relatime shared:38 - ext4 /dev/loop1 rw,data=ordered
62 25 7:1 / /var/lib/kubelet/pods/f19fe4e2-5a63-11e8-962f-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-test rw,relatime shared:38 - ext4 /dev/loop1 rw,data=ordered
95 25 7:1 / /var/lib/kubelet/pods/4854a48b-5a64-11e8-962f-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-test rw,relatime shared:38 - ext4 /dev/loop1 rw,data=ordered
`,
			[]string{"/var/lib/kubelet/pods/f19fe4e2-5a63-11e8-962f-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-test",
				"/var/lib/kubelet/pods/4854a48b-5a64-11e8-962f-000c29bb0377/volumes/kubernetes.io~local-volume/local-pv-test"},
			nil,
		},
	}
	tmpFile, err := ioutil.TempFile("", "test-get-filetype")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()
	for _, v := range testcases {
		tmpFile.Truncate(0)
		tmpFile.Seek(0, 0)
		tmpFile.WriteString(v.mountInfos)
		tmpFile.Sync()
		refs, err := SearchMountPoints(v.source, tmpFile.Name())
		if !reflect.DeepEqual(refs, v.expectedRefs) {
			t.Errorf("test %q: expected Refs: %#v, got %#v", v.name, v.expectedRefs, refs)
		}
		if !reflect.DeepEqual(err, v.expectedErr) {
			t.Errorf("test %q: expected err: %v, got %v", v.name, v.expectedErr, err)
		}
	}
}
