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
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	slashpath "path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"bytes"

	humanize "github.com/dustin/go-humanize"
	"github.com/klauspost/readahead"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/disk"
	xioutil "github.com/minio/minio/pkg/ioutil"
	"github.com/minio/minio/pkg/mountinfo"
	"github.com/ncw/directio"
)

const (
	diskMinFreeSpace  = 900 * humanize.MiByte // Min 900MiB free space.
	diskMinTotalSpace = diskMinFreeSpace      // Min 900MiB total space.
	maxAllowedIOError = 5
	readBlockSize     = humanize.KiByte * 32 // Default read block size 32KiB.
)

// isValidVolname verifies a volname name in accordance with object
// layer requirements.
func isValidVolname(volname string) bool {
	if len(volname) < 3 {
		return false
	}

	if runtime.GOOS == "windows" {
		// Volname shouldn't have reserved characters in Windows.
		return !strings.ContainsAny(volname, `\:*?\"<>|`)
	}

	return true
}

// posix - implements StorageAPI interface.
type posix struct {
	// Disk usage metrics
	totalUsed  uint64 // ref: https://golang.org/pkg/sync/atomic/#pkg-note-BUG
	ioErrCount int32  // ref: https://golang.org/pkg/sync/atomic/#pkg-note-BUG

	diskPath  string
	pool      sync.Pool
	connected bool

	diskMount bool // indicates if the path is an actual mount.

	diskFileInfo os.FileInfo
	// Disk usage metrics
	stopUsageCh chan struct{}
}

// checkPathLength - returns error if given path name length more than 255
func checkPathLength(pathName string) error {
	// Apple OS X path length is limited to 1016
	if runtime.GOOS == "darwin" && len(pathName) > 1016 {
		return errFileNameTooLong
	}

	if runtime.GOOS == "windows" {
		// Convert any '\' to '/'.
		pathName = filepath.ToSlash(pathName)
	}

	// Check each path segment length is > 255
	for len(pathName) > 0 && pathName != "." && pathName != "/" {
		dir, file := slashpath.Dir(pathName), slashpath.Base(pathName)

		if len(file) > 255 {
			return errFileNameTooLong
		}

		pathName = dir
	} // Success.
	return nil
}

func getValidPath(path string) (string, error) {
	if path == "" {
		return path, errInvalidArgument
	}

	var err error
	// Disallow relative paths, figure out absolute paths.
	path, err = filepath.Abs(path)
	if err != nil {
		return path, err
	}

	fi, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return path, err
	}
	if os.IsNotExist(err) {
		// Disk not found create it.
		if err = os.MkdirAll(path, 0777); err != nil {
			return path, err
		}
	}
	if fi != nil && !fi.IsDir() {
		return path, syscall.ENOTDIR
	}

	di, err := getDiskInfo(path)
	if err != nil {
		return path, err
	}
	if err = checkDiskMinTotal(di); err != nil {
		return path, err
	}

	// check if backend is writable.
	file, err := os.Create(pathJoin(path, ".writable-check.tmp"))
	if err != nil {
		return path, err
	}
	defer os.Remove(pathJoin(path, ".writable-check.tmp"))
	file.Close()

	return path, nil
}

// isDirEmpty - returns whether given directory is empty or not.
func isDirEmpty(dirname string) bool {
	f, err := os.Open((dirname))
	if err != nil {
		if !os.IsNotExist(err) {
			logger.LogIf(context.Background(), err)
		}

		return false
	}
	defer f.Close()
	// List one entry.
	_, err = f.Readdirnames(1)
	if err != io.EOF {
		if !os.IsNotExist(err) {
			logger.LogIf(context.Background(), err)
		}

		return false
	}
	// Returns true if we have reached EOF, directory is indeed empty.
	return true
}

// Initialize a new storage disk.
func newPosix(path string) (*posix, error) {
	var err error
	if path, err = getValidPath(path); err != nil {
		return nil, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	p := &posix{
		connected: true,
		diskPath:  path,
		pool: sync.Pool{
			New: func() interface{} {
				b := directio.AlignedBlock(readBlockSize)
				return &b
			},
		},
		stopUsageCh:  make(chan struct{}),
		diskFileInfo: fi,
		diskMount:    mountinfo.IsLikelyMountPoint(path),
	}

	if !p.diskMount {
		go p.diskUsage(GlobalServiceDoneCh)
	}

	// Success.
	return p, nil
}

// getDiskInfo returns given disk information.
func getDiskInfo(diskPath string) (di disk.Info, err error) {
	if err = checkPathLength(diskPath); err == nil {
		di, err = disk.GetInfo(diskPath)
	}

	if os.IsNotExist(err) {
		err = errDiskNotFound
	}

	return di, err
}

// List of operating systems where we ignore disk space
// verification.
var ignoreDiskFreeOS = []string{
	globalWindowsOSName,
	globalNetBSDOSName,
}

// check if disk total has minimum required size.
func checkDiskMinTotal(di disk.Info) (err error) {
	// Remove 5% from total space for cumulative disk space
	// used for journalling, inodes etc.
	totalDiskSpace := float64(di.Total) * 0.95
	if int64(totalDiskSpace) <= diskMinTotalSpace {
		return errMinDiskSize
	}
	return nil
}

// check if disk free has minimum required size.
func checkDiskMinFree(di disk.Info) error {
	// Remove 5% from free space for cumulative disk space used for journalling, inodes etc.
	availableDiskSpace := float64(di.Free) * 0.95
	if int64(availableDiskSpace) <= diskMinFreeSpace {
		return errDiskFull
	}

	// Success.
	return nil
}

// checkDiskFree verifies if disk path has sufficient minimum free disk space and files.
func checkDiskFree(diskPath string, neededSpace int64) (err error) {
	// We don't validate disk space or inode utilization on windows.
	// Each windows call to 'GetVolumeInformationW' takes around
	// 3-5seconds. And StatDISK is not supported by Go for solaris
	// and netbsd.
	if contains(ignoreDiskFreeOS, runtime.GOOS) {
		return nil
	}

	var di disk.Info
	di, err = getDiskInfo((diskPath))
	if err != nil {
		return err
	}

	if err = checkDiskMinFree(di); err != nil {
		return err
	}

	// Check if we have enough space to store data
	if neededSpace > int64(float64(di.Free)*0.95) {
		return errDiskFull
	}

	return nil
}

// Implements stringer compatible interface.
func (s *posix) String() string {
	return s.diskPath
}

func (s *posix) LastError() error {
	return nil
}

func (s *posix) Close() error {
	close(s.stopUsageCh)
	s.connected = false
	return nil
}

func (s *posix) IsOnline() bool {
	return s.connected
}

// DiskInfo is an extended type which returns current
// disk usage per path.
type DiskInfo struct {
	Total    uint64
	Free     uint64
	Used     uint64
	RootDisk bool
}

// DiskInfo provides current information about disk space usage,
// total free inodes and underlying filesystem.
func (s *posix) DiskInfo() (info DiskInfo, err error) {
	di, err := getDiskInfo(s.diskPath)
	if err != nil {
		return info, err
	}
	used := di.Total - di.Free
	if !s.diskMount {
		used = atomic.LoadUint64(&s.totalUsed)
	}

	rootDisk, err := disk.IsRootDisk(s.diskPath)
	if err != nil {
		return info, err
	}
	return DiskInfo{
		Total:    di.Total,
		Free:     di.Free,
		Used:     used,
		RootDisk: rootDisk,
	}, nil
}

// getVolDir - will convert incoming volume names to
// corresponding valid volume names on the backend in a platform
// compatible way for all operating systems. If volume is not found
// an error is generated.
func (s *posix) getVolDir(volume string) (string, error) {
	if volume == "" || volume == "." || volume == ".." {
		return "", errVolumeNotFound
	}
	volumeDir := pathJoin(s.diskPath, volume)
	return volumeDir, nil
}

// checkDiskFound - validates if disk is available,
// returns errDiskNotFound if not found.
func (s *posix) checkDiskFound() (err error) {
	if !s.IsOnline() {
		return errDiskNotFound
	}
	fi, err := os.Stat(s.diskPath)
	if err != nil {
		switch {
		case os.IsNotExist(err):
			return errDiskNotFound
		case isSysErrTooLong(err):
			return errFileNameTooLong
		case isSysErrIO(err):
			return errFaultyDisk
		default:
			return err
		}
	}
	if !os.SameFile(s.diskFileInfo, fi) {
		s.connected = false
		return errDiskNotFound
	}
	return nil
}

// diskUsage returns du information for the posix path, in a continuous routine.
func (s *posix) diskUsage(doneCh chan struct{}) {
	ticker := time.NewTicker(globalUsageCheckInterval)
	defer ticker.Stop()

	usageFn := func(ctx context.Context, entry string) error {
		if globalHTTPServer != nil {
			// Wait at max 1 minute for an inprogress request
			// before proceeding to count the usage.
			waitCount := 60
			// Any requests in progress, delay the usage.
			for globalHTTPServer.GetRequestCount() > 0 && waitCount > 0 {
				waitCount--
				time.Sleep(1 * time.Second)
			}
		}

		select {
		case <-doneCh:
			return errWalkAbort
		case <-s.stopUsageCh:
			return errWalkAbort
		default:
			fi, err := os.Stat(entry)
			if err != nil {
				err = osErrToFSFileErr(err)
				return err
			}
			atomic.AddUint64(&s.totalUsed, uint64(fi.Size()))
			return nil
		}
	}

	// Return this routine upon errWalkAbort, continue for any other error on purpose
	// so that we can start the routine freshly in another 12 hours.
	if err := getDiskUsage(context.Background(), s.diskPath, usageFn); err == errWalkAbort {
		return
	}

	for {
		select {
		case <-s.stopUsageCh:
			return
		case <-doneCh:
			return
		case <-time.After(globalUsageCheckInterval):
			var usage uint64
			usageFn = func(ctx context.Context, entry string) error {
				if globalHTTPServer != nil {
					// Wait at max 1 minute for an inprogress request
					// before proceeding to count the usage.
					waitCount := 60
					// Any requests in progress, delay the usage.
					for globalHTTPServer.GetRequestCount() > 0 && waitCount > 0 {
						waitCount--
						time.Sleep(1 * time.Second)
					}
				}

				select {
				case <-s.stopUsageCh:
					return errWalkAbort
				default:
					fi, err := os.Stat(entry)
					if err != nil {
						err = osErrToFSFileErr(err)
						return err
					}
					usage = usage + uint64(fi.Size())
					return nil
				}
			}

			if err := getDiskUsage(context.Background(), s.diskPath, usageFn); err != nil {
				continue
			}

			atomic.StoreUint64(&s.totalUsed, usage)
		}
	}
}

// Make a volume entry.
func (s *posix) MakeVol(volume string) (err error) {
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return errFaultyDisk
	}

	if err = s.checkDiskFound(); err != nil {
		return err
	}

	if !isValidVolname(volume) {
		return errInvalidArgument
	}

	volumeDir, err := s.getVolDir(volume)
	if err != nil {
		return err
	}

	if _, err := os.Stat(volumeDir); err != nil {
		// Volume does not exist we proceed to create.
		if os.IsNotExist(err) {
			// Make a volume entry, with mode 0777 mkdir honors system umask.
			err = os.MkdirAll(volumeDir, 0777)
		}
		if os.IsPermission(err) {
			return errDiskAccessDenied
		} else if isSysErrIO(err) {
			return errFaultyDisk
		}
		return err
	}

	// Stat succeeds we return errVolumeExists.
	return errVolumeExists
}

// ListVols - list volumes.
func (s *posix) ListVols() (volsInfo []VolInfo, err error) {
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return nil, errFaultyDisk
	}

	if err = s.checkDiskFound(); err != nil {
		return nil, err
	}

	volsInfo, err = listVols(s.diskPath)
	if err != nil {
		if isSysErrIO(err) {
			return nil, errFaultyDisk
		}
		return nil, err
	}
	for i, vol := range volsInfo {
		volInfo := VolInfo{
			Name:    vol.Name,
			Created: vol.Created,
		}
		volsInfo[i] = volInfo
	}
	return volsInfo, nil
}

// List all the volumes from diskPath.
func listVols(dirPath string) ([]VolInfo, error) {
	if err := checkPathLength(dirPath); err != nil {
		return nil, err
	}
	entries, err := readDir(dirPath)
	if err != nil {
		return nil, errDiskNotFound
	}
	var volsInfo []VolInfo
	for _, entry := range entries {
		if !hasSuffix(entry, slashSeparator) || !isValidVolname(slashpath.Clean(entry)) {
			// Skip if entry is neither a directory not a valid volume name.
			continue
		}
		var fi os.FileInfo
		fi, err = os.Stat(pathJoin(dirPath, entry))
		if err != nil {
			// If the file does not exist, skip the entry.
			if os.IsNotExist(err) {
				continue
			} else if isSysErrIO(err) {
				return nil, errFaultyDisk
			}
			return nil, err
		}
		volsInfo = append(volsInfo, VolInfo{
			Name: fi.Name(),
			// As os.Stat() doesn't carry other than ModTime(), use
			// ModTime() as CreatedTime.
			Created: fi.ModTime(),
		})
	}
	return volsInfo, nil
}

// StatVol - get volume info.
func (s *posix) StatVol(volume string) (volInfo VolInfo, err error) {
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return VolInfo{}, errFaultyDisk
	}

	if err = s.checkDiskFound(); err != nil {
		return VolInfo{}, err
	}

	// Verify if volume is valid and it exists.
	volumeDir, err := s.getVolDir(volume)
	if err != nil {
		return VolInfo{}, err
	}
	// Stat a volume entry.
	var st os.FileInfo
	st, err = os.Stat(volumeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return VolInfo{}, errVolumeNotFound
		} else if isSysErrIO(err) {
			return VolInfo{}, errFaultyDisk
		}
		return VolInfo{}, err
	}
	// As os.Stat() doesn't carry other than ModTime(), use ModTime()
	// as CreatedTime.
	createdTime := st.ModTime()
	return VolInfo{
		Name:    volume,
		Created: createdTime,
	}, nil
}

// DeleteVol - delete a volume.
func (s *posix) DeleteVol(volume string) (err error) {
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return errFaultyDisk
	}

	if err = s.checkDiskFound(); err != nil {
		return err
	}

	// Verify if volume is valid and it exists.
	volumeDir, err := s.getVolDir(volume)
	if err != nil {
		return err
	}
	err = os.Remove((volumeDir))
	if err != nil {
		switch {
		case os.IsNotExist(err):
			return errVolumeNotFound
		case isSysErrNotEmpty(err):
			return errVolumeNotEmpty
		case os.IsPermission(err):
			return errDiskAccessDenied
		case isSysErrIO(err):
			return errFaultyDisk
		default:
			return err
		}
	}
	return nil
}

// Walk - is a sorted walker which returns file entries in lexically
// sorted order, additionally along with metadata about each of those entries.
func (s *posix) Walk(volume, dirPath, marker string, recursive bool, leafFile string,
	readMetadataFn readMetadataFunc, endWalkCh chan struct{}) (ch chan FileInfo, err error) {

	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return nil, errFaultyDisk
	}

	if err = s.checkDiskFound(); err != nil {
		return nil, err
	}

	// Verify if volume is valid and it exists.
	volumeDir, err := s.getVolDir(volume)
	if err != nil {
		return nil, err
	}

	// Stat a volume entry.
	_, err = os.Stat(volumeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errVolumeNotFound
		} else if isSysErrIO(err) {
			return nil, errFaultyDisk
		}
		return nil, err
	}

	ch = make(chan FileInfo)
	go func() {
		defer close(ch)
		listDir := func(volume, dirPath, dirEntry string) (entries []string) {
			entries, err := s.ListDir(volume, dirPath, -1, leafFile)
			if err != nil {
				return
			}
			sort.Strings(entries)
			return filterMatchingPrefix(entries, dirEntry)
		}

		walkResultCh := startTreeWalk(context.Background(), volume, dirPath, marker, recursive, listDir, endWalkCh)
		for {
			walkResult, ok := <-walkResultCh
			if !ok {
				return
			}
			var fi FileInfo
			if hasSuffix(walkResult.entry, slashSeparator) {
				fi = FileInfo{
					Volume: volume,
					Name:   walkResult.entry,
					Mode:   os.ModeDir,
				}
			} else {
				buf, err := s.ReadAll(volume, pathJoin(walkResult.entry, leafFile))
				if err != nil {
					continue
				}
				fi = readMetadataFn(buf, volume, walkResult.entry)
			}
			select {
			case ch <- fi:
			case <-endWalkCh:
				return
			}
		}
	}()

	return ch, nil
}

// ListDir - return all the entries at the given directory path.
// If an entry is a directory it will be returned with a trailing "/".
func (s *posix) ListDir(volume, dirPath string, count int, leafFile string) (entries []string, err error) {
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return nil, errFaultyDisk
	}

	if err = s.checkDiskFound(); err != nil {
		return nil, err
	}

	// Verify if volume is valid and it exists.
	volumeDir, err := s.getVolDir(volume)
	if err != nil {
		return nil, err
	}
	// Stat a volume entry.
	_, err = os.Stat((volumeDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errVolumeNotFound
		} else if isSysErrIO(err) {
			return nil, errFaultyDisk
		}
		return nil, err
	}

	dirPath = pathJoin(volumeDir, dirPath)
	if count > 0 {
		entries, err = readDirN(dirPath, count)
	} else {
		entries, err = readDir(dirPath)
	}

	// If leaf file is specified, filter out the entries.
	if leafFile != "" {
		for i, entry := range entries {
			if _, serr := os.Stat(pathJoin(dirPath, entry, leafFile)); serr == nil {
				entries[i] = strings.TrimSuffix(entry, slashSeparator)
			}
		}
	}

	return entries, err
}

// ReadAll reads from r until an error or EOF and returns the data it read.
// A successful call returns err == nil, not err == EOF. Because ReadAll is
// defined to read from src until EOF, it does not treat an EOF from Read
// as an error to be reported.
// This API is meant to be used on files which have small memory footprint, do
// not use this on large files as it would cause server to crash.
func (s *posix) ReadAll(volume, path string) (buf []byte, err error) {
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return nil, errFaultyDisk
	}

	if err = s.checkDiskFound(); err != nil {
		return nil, err
	}

	volumeDir, err := s.getVolDir(volume)
	if err != nil {
		return nil, err
	}
	// Stat a volume entry.
	_, err = os.Stat((volumeDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errVolumeNotFound
		} else if isSysErrIO(err) {
			return nil, errFaultyDisk
		} else if isSysErrTooManyFiles(err) {
			return nil, errTooManyOpenFiles
		}
		return nil, err
	}

	// Validate file path length, before reading.
	filePath := pathJoin(volumeDir, path)
	if err = checkPathLength((filePath)); err != nil {
		return nil, err
	}

	// Open the file for reading.
	buf, err = ioutil.ReadFile((filePath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errFileNotFound
		} else if os.IsPermission(err) {
			return nil, errFileAccessDenied
		} else if pathErr, ok := err.(*os.PathError); ok {
			switch pathErr.Err {
			case syscall.ENOTDIR, syscall.EISDIR:
				return nil, errFileNotFound
			default:
				if isSysErrHandleInvalid(pathErr.Err) {
					// This case is special and needs to be handled for windows.
					return nil, errFileNotFound
				}
			}
			return nil, pathErr
		} else if isSysErrIO(err) {
			return nil, errFaultyDisk
		}
		return nil, err
	}
	return buf, nil
}

// ReadFile reads exactly len(buf) bytes into buf. It returns the
// number of bytes copied. The error is EOF only if no bytes were
// read. On return, n == len(buf) if and only if err == nil. n == 0
// for io.EOF.
//
// If an EOF happens after reading some but not all the bytes,
// ReadFile returns ErrUnexpectedEOF.
//
// If the BitrotVerifier is not nil or not verified ReadFile
// tries to verify whether the disk has bitrot.
//
// Additionally ReadFile also starts reading from an offset. ReadFile
// semantics are same as io.ReadFull.
func (s *posix) ReadFile(volume, path string, offset int64, buffer []byte, verifier *BitrotVerifier) (int64, error) {
	var n int
	var err error
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if offset < 0 {
		return 0, errInvalidArgument
	}

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return 0, errFaultyDisk
	}

	if err = s.checkDiskFound(); err != nil {
		return 0, err
	}

	volumeDir, err := s.getVolDir(volume)
	if err != nil {
		return 0, err
	}
	// Stat a volume entry.
	_, err = os.Stat((volumeDir))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, errVolumeNotFound
		} else if isSysErrIO(err) {
			return 0, errFaultyDisk
		}
		return 0, err
	}

	// Validate effective path length before reading.
	filePath := pathJoin(volumeDir, path)
	if err = checkPathLength((filePath)); err != nil {
		return 0, err
	}

	// Open the file for reading.
	file, err := os.Open((filePath))
	if err != nil {
		switch {
		case os.IsNotExist(err):
			return 0, errFileNotFound
		case os.IsPermission(err):
			return 0, errFileAccessDenied
		case isSysErrNotDir(err):
			return 0, errFileAccessDenied
		case isSysErrIO(err):
			return 0, errFaultyDisk
		case isSysErrTooManyFiles(err):
			return 0, errTooManyOpenFiles
		default:
			return 0, err
		}
	}

	// Close the file descriptor.
	defer file.Close()

	st, err := file.Stat()
	if err != nil {
		return 0, err
	}

	// Verify it is a regular file, otherwise subsequent Seek is
	// undefined.
	if !st.Mode().IsRegular() {
		return 0, errIsNotRegular
	}

	if verifier == nil {
		n, err = file.ReadAt(buffer, offset)
		return int64(n), err
	}

	bufp := s.pool.Get().(*[]byte)
	defer s.pool.Put(bufp)

	h := verifier.algorithm.New()
	if _, err = io.CopyBuffer(h, io.LimitReader(file, offset), *bufp); err != nil {
		return 0, err
	}

	if n, err = io.ReadFull(file, buffer); err != nil {
		return int64(n), err
	}

	if _, err = h.Write(buffer); err != nil {
		return 0, err
	}

	if _, err = io.CopyBuffer(h, file, *bufp); err != nil {
		return 0, err
	}

	if !bytes.Equal(h.Sum(nil), verifier.sum) {
		return 0, hashMismatchError{hex.EncodeToString(verifier.sum), hex.EncodeToString(h.Sum(nil))}
	}

	return int64(len(buffer)), nil
}

func (s *posix) openFile(volume, path string, mode int) (f *os.File, err error) {
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return nil, errFaultyDisk
	}

	if err = s.checkDiskFound(); err != nil {
		return nil, err
	}

	volumeDir, err := s.getVolDir(volume)
	if err != nil {
		return nil, err
	}
	// Stat a volume entry.
	_, err = os.Stat((volumeDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errVolumeNotFound
		} else if isSysErrIO(err) {
			return nil, errFaultyDisk
		}
		return nil, err
	}

	filePath := pathJoin(volumeDir, path)
	if err = checkPathLength((filePath)); err != nil {
		return nil, err
	}

	// Verify if the file already exists and is not of regular type.
	var st os.FileInfo
	if st, err = os.Stat(filePath); err == nil {
		if !st.Mode().IsRegular() {
			return nil, errIsNotRegular
		}
	} else {
		// Create top level directories if they don't exist.
		// with mode 0777 mkdir honors system umask.
		if err = mkdirAll(slashpath.Dir(filePath), 0777); err != nil {
			return nil, err
		}
	}

	w, err := os.OpenFile(filePath, mode, 0666)
	if err != nil {
		// File path cannot be verified since one of the parents is a file.
		switch {
		case isSysErrNotDir(err):
			return nil, errFileAccessDenied
		case os.IsPermission(err):
			return nil, errFileAccessDenied
		case isSysErrIO(err):
			return nil, errFaultyDisk
		case isSysErrTooManyFiles(err):
			return nil, errTooManyOpenFiles
		default:
			return nil, err
		}
	}

	return w, nil
}

// ReadFileStream - Returns the read stream of the file.
func (s *posix) ReadFileStream(volume, path string, offset, length int64) (io.ReadCloser, error) {
	var err error
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if offset < 0 {
		return nil, errInvalidArgument
	}

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return nil, errFaultyDisk
	}

	if err = s.checkDiskFound(); err != nil {
		return nil, err
	}

	volumeDir, err := s.getVolDir(volume)
	if err != nil {
		return nil, err
	}
	// Stat a volume entry.
	_, err = os.Stat((volumeDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errVolumeNotFound
		} else if isSysErrIO(err) {
			return nil, errFaultyDisk
		}
		return nil, err
	}

	// Validate effective path length before reading.
	filePath := pathJoin(volumeDir, path)
	if err = checkPathLength((filePath)); err != nil {
		return nil, err
	}

	// Open the file for reading.
	file, err := os.Open((filePath))
	if err != nil {
		switch {
		case os.IsNotExist(err):
			return nil, errFileNotFound
		case os.IsPermission(err):
			return nil, errFileAccessDenied
		case isSysErrNotDir(err):
			return nil, errFileAccessDenied
		case isSysErrIO(err):
			return nil, errFaultyDisk
		case isSysErrTooManyFiles(err):
			return nil, errTooManyOpenFiles
		default:
			return nil, err
		}
	}

	st, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// Verify it is a regular file, otherwise subsequent Seek is
	// undefined.
	if !st.Mode().IsRegular() {
		return nil, errIsNotRegular
	}

	if _, err = file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}

	r := struct {
		io.Reader
		io.Closer
	}{Reader: io.LimitReader(file, length), Closer: file}

	return readahead.NewReadCloser(r), nil
}

// CreateFile - creates the file.
func (s *posix) CreateFile(volume, path string, fileSize int64, r io.Reader) (err error) {
	if fileSize < -1 {
		return errInvalidArgument
	}
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return errFaultyDisk
	}

	// Validate if disk is indeed free.
	if err = checkDiskFree(s.diskPath, fileSize); err != nil {
		if isSysErrIO(err) {
			return errFaultyDisk
		}
		return err
	}

	if err = s.checkDiskFound(); err != nil {
		return err
	}

	volumeDir, err := s.getVolDir(volume)
	if err != nil {
		return err
	}
	// Stat a volume entry.
	_, err = os.Stat((volumeDir))
	if err != nil {
		if os.IsNotExist(err) {
			return errVolumeNotFound
		} else if isSysErrIO(err) {
			return errFaultyDisk
		}
		return err
	}

	filePath := pathJoin(volumeDir, path)
	if err = checkPathLength((filePath)); err != nil {
		return err
	}

	// Create top level directories if they don't exist.
	// with mode 0777 mkdir honors system umask.
	if err = mkdirAll(slashpath.Dir(filePath), 0777); err != nil {
		return err
	}

	w, err := disk.OpenFileDirectIO(filePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0666)
	if err != nil {
		switch {
		case os.IsPermission(err):
			return errFileAccessDenied
		case os.IsExist(err):
			return errFileAccessDenied
		case isSysErrIO(err):
			return errFaultyDisk
		default:
			return err
		}
	}

	var e error
	if fileSize > 0 {
		// Allocate needed disk space to append data
		e = Fallocate(int(w.Fd()), 0, fileSize)
	}

	// Ignore errors when Fallocate is not supported in the current system
	if e != nil && !isSysErrNoSys(e) && !isSysErrOpNotSupported(e) {
		switch {
		case isSysErrNoSpace(e):
			err = errDiskFull
		case isSysErrIO(e):
			err = errFaultyDisk
		default:
			// For errors: EBADF, EINTR, EINVAL, ENODEV, EPERM, ESPIPE  and ETXTBSY
			// Appending was failed anyway, returns unexpected error
			err = errUnexpected
		}
		return err
	}

	defer w.Close()

	bufp := s.pool.Get().(*[]byte)
	defer s.pool.Put(bufp)

	written, err := xioutil.CopyAligned(w, r, *bufp)
	if err != nil {
		return err
	}

	if written < fileSize {
		return errLessData
	} else if written > fileSize {
		return errMoreData
	}

	return nil
}

func (s *posix) WriteAll(volume, path string, reader io.Reader) (err error) {
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return errFaultyDisk
	}

	// Create file if not found. Note that it is created with os.O_EXCL flag as the file
	// always is supposed to be created in the tmp directory with a unique file name.
	w, err := s.openFile(volume, path, os.O_CREATE|os.O_SYNC|os.O_WRONLY|os.O_EXCL)
	if err != nil {
		return err
	}

	defer w.Close()

	bufp := s.pool.Get().(*[]byte)
	defer s.pool.Put(bufp)

	_, err = io.CopyBuffer(w, reader, *bufp)
	return err
}

// AppendFile - append a byte array at path, if file doesn't exist at
// path this call explicitly creates it.
func (s *posix) AppendFile(volume, path string, buf []byte) (err error) {
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return errFaultyDisk
	}

	var w *os.File
	// Create file if not found. Not doing O_DIRECT here to avoid the code that does buffer aligned writes.
	// AppendFile() is only used by healing code to heal objects written in old format.
	w, err = s.openFile(volume, path, os.O_CREATE|os.O_SYNC|os.O_APPEND|os.O_WRONLY)
	if err != nil {
		return err
	}

	if _, err = w.Write(buf); err != nil {
		return err
	}

	return w.Close()
}

// StatFile - get file info.
func (s *posix) StatFile(volume, path string) (file FileInfo, err error) {
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return FileInfo{}, errFaultyDisk
	}

	if err = s.checkDiskFound(); err != nil {
		return FileInfo{}, err
	}

	volumeDir, err := s.getVolDir(volume)
	if err != nil {
		return FileInfo{}, err
	}
	// Stat a volume entry.
	_, err = os.Stat((volumeDir))
	if err != nil {
		if os.IsNotExist(err) {
			return FileInfo{}, errVolumeNotFound
		}
		return FileInfo{}, err
	}

	filePath := slashpath.Join(volumeDir, path)
	if err = checkPathLength((filePath)); err != nil {
		return FileInfo{}, err
	}
	st, err := os.Stat((filePath))
	if err != nil {
		switch {
		case os.IsNotExist(err):
			// File is really not found.
			return FileInfo{}, errFileNotFound
		case isSysErrIO(err):
			return FileInfo{}, errFaultyDisk
		case isSysErrNotDir(err):
			// File path cannot be verified since one of the parents is a file.
			return FileInfo{}, errFileNotFound
		default:
			// Return all errors here.
			return FileInfo{}, err
		}
	}
	// If its a directory its not a regular file.
	if st.Mode().IsDir() {
		return FileInfo{}, errFileNotFound
	}
	return FileInfo{
		Volume:  volume,
		Name:    path,
		ModTime: st.ModTime(),
		Size:    st.Size(),
		Mode:    st.Mode(),
	}, nil
}

// deleteFile deletes a file path if its empty. If it's successfully deleted,
// it will recursively move up the tree, deleting empty parent directories
// until it finds one with files in it. Returns nil for a non-empty directory.
func deleteFile(basePath, deletePath string) error {
	if basePath == deletePath {
		return nil
	}

	// Attempt to remove path.
	if err := os.Remove((deletePath)); err != nil {
		switch {
		case isSysErrNotEmpty(err):
			// Ignore errors if the directory is not empty. The server relies on
			// this functionality, and sometimes uses recursion that should not
			// error on parent directories.
			return nil
		case os.IsNotExist(err):
			return errFileNotFound
		case os.IsPermission(err):
			return errFileAccessDenied
		case isSysErrIO(err):
			return errFaultyDisk
		default:
			return err
		}
	}

	// Trailing slash is removed when found to ensure
	// slashpath.Dir() to work as intended.
	deletePath = strings.TrimSuffix(deletePath, slashSeparator)
	deletePath = slashpath.Dir(deletePath)

	// Delete parent directory. Errors for parent directories shouldn't trickle down.
	deleteFile(basePath, deletePath)

	return nil
}

// DeleteFile - delete a file at path.
func (s *posix) DeleteFile(volume, path string) (err error) {
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return errFaultyDisk
	}

	if err = s.checkDiskFound(); err != nil {
		return err
	}

	volumeDir, err := s.getVolDir(volume)
	if err != nil {
		return err
	}
	// Stat a volume entry.
	_, err = os.Stat((volumeDir))
	if err != nil {
		if os.IsNotExist(err) {
			return errVolumeNotFound
		} else if isSysErrIO(err) {
			return errFaultyDisk
		}
		return err
	}

	// Following code is needed so that we retain "/" suffix if any in
	// path argument.
	filePath := pathJoin(volumeDir, path)
	if err = checkPathLength((filePath)); err != nil {
		return err
	}

	// Delete file and delete parent directory as well if its empty.
	return deleteFile(volumeDir, filePath)
}

func (s *posix) DeleteFileBulk(volume string, paths []string) (errs []error, err error) {
	errs = make([]error, len(paths))
	for idx, path := range paths {
		errs[idx] = s.DeleteFile(volume, path)
	}
	return
}

// RenameFile - rename source path to destination path atomically.
func (s *posix) RenameFile(srcVolume, srcPath, dstVolume, dstPath string) (err error) {
	defer func() {
		if err == errFaultyDisk {
			atomic.AddInt32(&s.ioErrCount, 1)
		}
	}()

	if atomic.LoadInt32(&s.ioErrCount) > maxAllowedIOError {
		return errFaultyDisk
	}

	if err = s.checkDiskFound(); err != nil {
		return err
	}

	srcVolumeDir, err := s.getVolDir(srcVolume)
	if err != nil {
		return err
	}
	dstVolumeDir, err := s.getVolDir(dstVolume)
	if err != nil {
		return err
	}
	// Stat a volume entry.
	_, err = os.Stat(srcVolumeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return errVolumeNotFound
		} else if isSysErrIO(err) {
			return errFaultyDisk
		}
		return err
	}
	_, err = os.Stat(dstVolumeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return errVolumeNotFound
		} else if isSysErrIO(err) {
			return errFaultyDisk
		}
	}

	srcIsDir := hasSuffix(srcPath, slashSeparator)
	dstIsDir := hasSuffix(dstPath, slashSeparator)
	// Either src and dst have to be directories or files, else return error.
	if !(srcIsDir && dstIsDir || !srcIsDir && !dstIsDir) {
		return errFileAccessDenied
	}
	srcFilePath := slashpath.Join(srcVolumeDir, srcPath)
	if err = checkPathLength(srcFilePath); err != nil {
		return err
	}
	dstFilePath := slashpath.Join(dstVolumeDir, dstPath)
	if err = checkPathLength(dstFilePath); err != nil {
		return err
	}
	if srcIsDir {
		// If source is a directory, we expect the destination to be non-existent but we
		// we still need to allow overwriting an empty directory since it represents
		// an object empty directory.
		_, err = os.Stat(dstFilePath)
		if isSysErrIO(err) {
			return errFaultyDisk
		}
		if err == nil && !isDirEmpty(dstFilePath) {
			return errFileAccessDenied
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		// Empty destination remove it before rename.
		if isDirEmpty(dstFilePath) {
			if err = os.Remove(dstFilePath); err != nil {
				if isSysErrNotEmpty(err) {
					return errFileAccessDenied
				}
				return err
			}
		}
	}

	if err = renameAll(srcFilePath, dstFilePath); err != nil {
		if isSysErrIO(err) {
			return errFaultyDisk
		}
		return err
	}

	// Remove parent dir of the source file if empty
	if parentDir := slashpath.Dir(srcFilePath); isDirEmpty(parentDir) {
		deleteFile(srcVolumeDir, parentDir)
	}

	return nil
}
