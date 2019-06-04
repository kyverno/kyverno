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
	"io"
	"sync"
)

// naughtyDisk wraps a POSIX disk and returns programmed errors
// specified by the developer. The purpose is to simulate errors
// that are hard to simulate in practice like DiskNotFound.
// Programmed errors are stored in errors field.
type naughtyDisk struct {
	// The real disk
	disk StorageAPI
	// Programmed errors: API call number => error to return
	errors map[int]error
	// The error to return when no error value is programmed
	defaultErr error
	// The current API call number
	callNR int
	// Data protection
	mu sync.Mutex
}

func newNaughtyDisk(d StorageAPI, errs map[int]error, defaultErr error) *naughtyDisk {
	return &naughtyDisk{disk: d, errors: errs, defaultErr: defaultErr}
}

func (d *naughtyDisk) String() string {
	return d.disk.String()
}

func (d *naughtyDisk) IsOnline() bool {
	if err := d.calcError(); err != nil {
		return err == errDiskNotFound
	}
	return d.disk.IsOnline()
}

func (d *naughtyDisk) LastError() (err error) {
	return nil
}

func (d *naughtyDisk) Close() (err error) {
	if err = d.calcError(); err != nil {
		return err
	}
	return d.disk.Close()
}

func (d *naughtyDisk) calcError() (err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.callNR++
	if err, ok := d.errors[d.callNR]; ok {
		return err
	}
	if d.defaultErr != nil {
		return d.defaultErr
	}
	return nil
}

func (d *naughtyDisk) DiskInfo() (info DiskInfo, err error) {
	if err := d.calcError(); err != nil {
		return info, err
	}
	return d.disk.DiskInfo()
}

func (d *naughtyDisk) MakeVol(volume string) (err error) {
	if err := d.calcError(); err != nil {
		return err
	}
	return d.disk.MakeVol(volume)
}

func (d *naughtyDisk) ListVols() (vols []VolInfo, err error) {
	if err := d.calcError(); err != nil {
		return nil, err
	}
	return d.disk.ListVols()
}

func (d *naughtyDisk) StatVol(volume string) (volInfo VolInfo, err error) {
	if err := d.calcError(); err != nil {
		return VolInfo{}, err
	}
	return d.disk.StatVol(volume)
}
func (d *naughtyDisk) DeleteVol(volume string) (err error) {
	if err := d.calcError(); err != nil {
		return err
	}
	return d.disk.DeleteVol(volume)
}

func (d *naughtyDisk) Walk(volume, path, marker string, recursive bool, leafFile string, readMetadataFn readMetadataFunc, endWalkCh chan struct{}) (chan FileInfo, error) {
	if err := d.calcError(); err != nil {
		return nil, err
	}
	return d.disk.Walk(volume, path, marker, recursive, leafFile, readMetadataFn, endWalkCh)
}

func (d *naughtyDisk) ListDir(volume, path string, count int, leafFile string) (entries []string, err error) {
	if err := d.calcError(); err != nil {
		return []string{}, err
	}
	return d.disk.ListDir(volume, path, count, leafFile)
}

func (d *naughtyDisk) ReadFile(volume string, path string, offset int64, buf []byte, verifier *BitrotVerifier) (n int64, err error) {
	if err := d.calcError(); err != nil {
		return 0, err
	}
	return d.disk.ReadFile(volume, path, offset, buf, verifier)
}

func (d *naughtyDisk) ReadFileStream(volume, path string, offset, length int64) (io.ReadCloser, error) {
	if err := d.calcError(); err != nil {
		return nil, err
	}
	return d.disk.ReadFileStream(volume, path, offset, length)
}

func (d *naughtyDisk) CreateFile(volume, path string, size int64, reader io.Reader) error {
	if err := d.calcError(); err != nil {
		return err
	}
	return d.disk.CreateFile(volume, path, size, reader)
}

func (d *naughtyDisk) AppendFile(volume, path string, buf []byte) error {
	if err := d.calcError(); err != nil {
		return err
	}
	return d.disk.AppendFile(volume, path, buf)
}

func (d *naughtyDisk) RenameFile(srcVolume, srcPath, dstVolume, dstPath string) error {
	if err := d.calcError(); err != nil {
		return err
	}
	return d.disk.RenameFile(srcVolume, srcPath, dstVolume, dstPath)
}

func (d *naughtyDisk) StatFile(volume string, path string) (file FileInfo, err error) {
	if err := d.calcError(); err != nil {
		return FileInfo{}, err
	}
	return d.disk.StatFile(volume, path)
}

func (d *naughtyDisk) DeleteFile(volume string, path string) (err error) {
	if err := d.calcError(); err != nil {
		return err
	}
	return d.disk.DeleteFile(volume, path)
}

func (d *naughtyDisk) DeleteFileBulk(volume string, paths []string) ([]error, error) {
	errs := make([]error, len(paths))
	for idx, path := range paths {
		errs[idx] = d.disk.DeleteFile(volume, path)
	}
	return errs, nil
}

func (d *naughtyDisk) WriteAll(volume string, path string, reader io.Reader) (err error) {
	if err := d.calcError(); err != nil {
		return err
	}
	return d.disk.WriteAll(volume, path, reader)
}

func (d *naughtyDisk) ReadAll(volume string, path string) (buf []byte, err error) {
	if err := d.calcError(); err != nil {
		return nil, err
	}
	return d.disk.ReadAll(volume, path)
}
