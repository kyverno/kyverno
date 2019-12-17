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
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/minio/minio/cmd/config"
	xhttp "github.com/minio/minio/cmd/http"
	"github.com/minio/minio/cmd/logger"
)

var errDiskStale = errors.New("disk stale")

// To abstract a disk over network.
type storageRESTServer struct {
	storage *posix
}

func (s *storageRESTServer) writeErrorResponse(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(err.Error()))
	w.(http.Flusher).Flush()
}

// DefaultSkewTime - skew time is 15 minutes between minio peers.
const DefaultSkewTime = 15 * time.Minute

// Authenticates storage client's requests and validates for skewed time.
func storageServerRequestValidate(r *http.Request) error {
	_, owner, err := webRequestAuthenticate(r)
	if err != nil {
		return err
	}
	if !owner { // Disable access for non-admin users.
		return errAuthentication
	}

	requestTimeStr := r.Header.Get("X-Minio-Time")
	requestTime, err := time.Parse(time.RFC3339, requestTimeStr)
	if err != nil {
		return err
	}
	utcNow := UTCNow()
	delta := requestTime.Sub(utcNow)
	if delta < 0 {
		delta = delta * -1
	}
	if delta > DefaultSkewTime {
		return fmt.Errorf("client time %v is too apart with server time %v", requestTime, utcNow)
	}
	return nil
}

// IsValid - To authenticate and verify the time difference.
func (s *storageRESTServer) IsValid(w http.ResponseWriter, r *http.Request) bool {
	if err := storageServerRequestValidate(r); err != nil {
		s.writeErrorResponse(w, err)
		return false
	}
	diskID := r.URL.Query().Get(storageRESTDiskID)
	if diskID == "" {
		// Request sent empty disk-id, we allow the request
		// as the peer might be coming up and trying to read format.json
		// or create format.json
		return true
	}
	storedDiskID, err := s.storage.getDiskID()
	if err == nil && diskID == storedDiskID {
		// If format.json is available and request sent the right disk-id, we allow the request
		return true
	}
	s.writeErrorResponse(w, errDiskStale)
	return false
}

// DiskInfoHandler - returns disk info.
func (s *storageRESTServer) DiskInfoHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	info, err := s.storage.DiskInfo()
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	defer w.(http.Flusher).Flush()
	gob.NewEncoder(w).Encode(info)
}

func (s *storageRESTServer) CrawlAndGetDataUsageHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}

	usageInfo, err := s.storage.CrawlAndGetDataUsage(GlobalServiceDoneCh)
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}

	w.Header().Set(xhttp.ContentType, "text/event-stream")
	doneCh := sendWhiteSpaceToHTTPResponse(w)
	usageInfo, err = s.storage.CrawlAndGetDataUsage(GlobalServiceDoneCh)
	<-doneCh

	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}

	gob.NewEncoder(w).Encode(usageInfo)
	w.(http.Flusher).Flush()
}

// MakeVolHandler - make a volume.
func (s *storageRESTServer) MakeVolHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	err := s.storage.MakeVol(volume)
	if err != nil {
		s.writeErrorResponse(w, err)
	}
}

// ListVolsHandler - list volumes.
func (s *storageRESTServer) ListVolsHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	infos, err := s.storage.ListVols()
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	gob.NewEncoder(w).Encode(&infos)
	w.(http.Flusher).Flush()
}

// StatVolHandler - stat a volume.
func (s *storageRESTServer) StatVolHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	info, err := s.storage.StatVol(volume)
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	gob.NewEncoder(w).Encode(info)
	w.(http.Flusher).Flush()
}

// DeleteVolumeHandler - delete a volume.
func (s *storageRESTServer) DeleteVolHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	err := s.storage.DeleteVol(volume)
	if err != nil {
		s.writeErrorResponse(w, err)
	}
}

// AppendFileHandler - append data from the request to the file specified.
func (s *storageRESTServer) AppendFileHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	filePath := vars[storageRESTFilePath]

	buf := make([]byte, r.ContentLength)
	_, err := io.ReadFull(r.Body, buf)
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	err = s.storage.AppendFile(volume, filePath, buf)
	if err != nil {
		s.writeErrorResponse(w, err)
	}
}

// CreateFileHandler - fallocate() space for a file and copy the contents from the request.
func (s *storageRESTServer) CreateFileHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	filePath := vars[storageRESTFilePath]

	fileSizeStr := vars[storageRESTLength]
	fileSize, err := strconv.Atoi(fileSizeStr)
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	err = s.storage.CreateFile(volume, filePath, int64(fileSize), r.Body)
	if err != nil {
		s.writeErrorResponse(w, err)
	}
}

// WriteAllHandler - write to file all content.
func (s *storageRESTServer) WriteAllHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	filePath := vars[storageRESTFilePath]

	if r.ContentLength < 0 {
		s.writeErrorResponse(w, errInvalidArgument)
		return
	}

	err := s.storage.WriteAll(volume, filePath, io.LimitReader(r.Body, r.ContentLength))
	if err != nil {
		s.writeErrorResponse(w, err)
	}
}

// StatFileHandler - stat a file.
func (s *storageRESTServer) StatFileHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	filePath := vars[storageRESTFilePath]

	info, err := s.storage.StatFile(volume, filePath)
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	gob.NewEncoder(w).Encode(info)
	w.(http.Flusher).Flush()
}

// ReadAllHandler - read all the contents of a file.
func (s *storageRESTServer) ReadAllHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	filePath := vars[storageRESTFilePath]

	buf, err := s.storage.ReadAll(volume, filePath)
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	w.Header().Set(xhttp.ContentLength, strconv.Itoa(len(buf)))
	w.Write(buf)
	w.(http.Flusher).Flush()
}

// ReadFileHandler - read section of a file.
func (s *storageRESTServer) ReadFileHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	filePath := vars[storageRESTFilePath]
	offset, err := strconv.Atoi(vars[storageRESTOffset])
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	length, err := strconv.Atoi(vars[storageRESTLength])
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	if offset < 0 || length < 0 {
		s.writeErrorResponse(w, errInvalidArgument)
		return
	}
	var verifier *BitrotVerifier
	if vars[storageRESTBitrotAlgo] != "" {
		hashStr := vars[storageRESTBitrotHash]
		var hash []byte
		hash, err = hex.DecodeString(hashStr)
		if err != nil {
			s.writeErrorResponse(w, err)
			return
		}
		verifier = NewBitrotVerifier(BitrotAlgorithmFromString(vars[storageRESTBitrotAlgo]), hash)
	}
	buf := make([]byte, length)
	_, err = s.storage.ReadFile(volume, filePath, int64(offset), buf, verifier)
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	w.Header().Set(xhttp.ContentLength, strconv.Itoa(len(buf)))
	w.Write(buf)
	w.(http.Flusher).Flush()
}

// ReadFileHandler - read section of a file.
func (s *storageRESTServer) ReadFileStreamHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	filePath := vars[storageRESTFilePath]
	offset, err := strconv.Atoi(vars[storageRESTOffset])
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	length, err := strconv.Atoi(vars[storageRESTLength])
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}

	rc, err := s.storage.ReadFileStream(volume, filePath, int64(offset), int64(length))
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	defer rc.Close()

	w.Header().Set(xhttp.ContentLength, strconv.Itoa(length))

	io.Copy(w, rc)
	w.(http.Flusher).Flush()

}

// readMetadata func provides the function types for reading leaf metadata.
type readMetadataFunc func(buf []byte, volume, entry string) FileInfo

func readMetadata(buf []byte, volume, entry string) FileInfo {
	m, err := xlMetaV1UnmarshalJSON(context.Background(), buf)
	if err != nil {
		return FileInfo{}
	}
	return FileInfo{
		Volume:   volume,
		Name:     entry,
		ModTime:  m.Stat.ModTime,
		Size:     m.Stat.Size,
		Metadata: m.Meta,
		Parts:    m.Parts,
		Quorum:   m.Erasure.DataBlocks,
	}
}

// WalkHandler - remote caller to start walking at a requested directory path.
func (s *storageRESTServer) WalkHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	dirPath := vars[storageRESTDirPath]
	markerPath := vars[storageRESTMarkerPath]
	recursive, err := strconv.ParseBool(vars[storageRESTRecursive])
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	leafFile := vars[storageRESTLeafFile]

	endWalkCh := make(chan struct{})
	defer close(endWalkCh)

	fch, err := s.storage.Walk(volume, dirPath, markerPath, recursive, leafFile, readMetadata, endWalkCh)
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}

	w.Header().Set(xhttp.ContentType, "text/event-stream")
	encoder := gob.NewEncoder(w)
	for fi := range fch {
		encoder.Encode(&fi)
	}
	w.(http.Flusher).Flush()
}

// ListDirHandler - list a directory.
func (s *storageRESTServer) ListDirHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	dirPath := vars[storageRESTDirPath]
	leafFile := vars[storageRESTLeafFile]
	count, err := strconv.Atoi(vars[storageRESTCount])
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	entries, err := s.storage.ListDir(volume, dirPath, count, leafFile)
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	gob.NewEncoder(w).Encode(&entries)
	w.(http.Flusher).Flush()
}

// DeleteFileHandler - delete a file.
func (s *storageRESTServer) DeleteFileHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	filePath := vars[storageRESTFilePath]

	err := s.storage.DeleteFile(volume, filePath)
	if err != nil {
		s.writeErrorResponse(w, err)
	}
}

// DeleteFileBulkErrsResp - collection of deleteFile errors
// for bulk deletes
type DeleteFileBulkErrsResp struct {
	Errs []error
}

// DeleteFileError - error captured per delete operation
type DeleteFileError string

func (d DeleteFileError) Error() string {
	return string(d)
}

// DeleteFileBulkHandler - delete a file.
func (s *storageRESTServer) DeleteFileBulkHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := r.URL.Query()
	volume := vars.Get(storageRESTVolume)
	filePaths := vars[storageRESTFilePath]

	errs, err := s.storage.DeleteFileBulk(volume, filePaths)
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}

	derrsResp := &DeleteFileBulkErrsResp{Errs: make([]error, len(errs))}
	for idx, err := range errs {
		if err != nil {
			derrsResp.Errs[idx] = DeleteFileError(err.Error())
		}
	}

	gob.NewEncoder(w).Encode(derrsResp)
	w.(http.Flusher).Flush()
}

// RenameFileHandler - rename a file.
func (s *storageRESTServer) RenameFileHandler(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	srcVolume := vars[storageRESTSrcVolume]
	srcFilePath := vars[storageRESTSrcPath]
	dstVolume := vars[storageRESTDstVolume]
	dstFilePath := vars[storageRESTDstPath]
	err := s.storage.RenameFile(srcVolume, srcFilePath, dstVolume, dstFilePath)
	if err != nil {
		s.writeErrorResponse(w, err)
	}
}

// Send whitespace to the client to avoid timeouts with long storage
// operations, such as bitrot verification or data usage crawling.
func sendWhiteSpaceToHTTPResponse(w http.ResponseWriter) <-chan struct{} {
	doneCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second * 10)
		for {
			select {
			case <-ticker.C:
				w.Write([]byte(" "))
				w.(http.Flusher).Flush()
			case doneCh <- struct{}{}:
				ticker.Stop()
				return
			}
		}

	}()
	return doneCh
}

// VerifyFileResp - VerifyFile()'s response.
type VerifyFileResp struct {
	Err error
}

// VerifyFile - Verify the file for bitrot errors.
func (s *storageRESTServer) VerifyFile(w http.ResponseWriter, r *http.Request) {
	if !s.IsValid(w, r) {
		return
	}
	vars := mux.Vars(r)
	volume := vars[storageRESTVolume]
	filePath := vars[storageRESTFilePath]
	size, err := strconv.ParseInt(vars[storageRESTLength], 10, 0)
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	shardSize, err := strconv.Atoi(vars[storageRESTShardSize])
	if err != nil {
		s.writeErrorResponse(w, err)
		return
	}
	hashStr := vars[storageRESTBitrotHash]
	var hash []byte
	if hashStr != "" {
		hash, err = hex.DecodeString(hashStr)
		if err != nil {
			s.writeErrorResponse(w, err)
			return
		}
	}
	algoStr := vars[storageRESTBitrotAlgo]
	if algoStr == "" {
		s.writeErrorResponse(w, errInvalidArgument)
		return
	}
	w.Header().Set(xhttp.ContentType, "text/event-stream")
	encoder := gob.NewEncoder(w)
	doneCh := sendWhiteSpaceToHTTPResponse(w)
	err = s.storage.VerifyFile(volume, filePath, size, BitrotAlgorithmFromString(algoStr), hash, int64(shardSize))
	<-doneCh
	vresp := &VerifyFileResp{}
	if err != nil {
		vresp.Err = VerifyFileError(err.Error())
	}
	encoder.Encode(vresp)
	w.(http.Flusher).Flush()
}

// registerStorageRPCRouter - register storage rpc router.
func registerStorageRESTHandlers(router *mux.Router, endpointZones EndpointZones) {
	for _, ep := range endpointZones {
		for _, endpoint := range ep.Endpoints {
			if !endpoint.IsLocal {
				continue
			}
			storage, err := newPosix(endpoint.Path)
			if err != nil {
				logger.Fatal(config.ErrUnableToWriteInBackend(err),
					"Unable to initialize posix backend")
			}

			server := &storageRESTServer{storage: storage}

			subrouter := router.PathPrefix(path.Join(storageRESTPrefix, endpoint.Path)).Subrouter()

			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodDiskInfo).HandlerFunc(httpTraceHdrs(server.DiskInfoHandler))
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodCrawlAndGetDataUsage).HandlerFunc(httpTraceHdrs(server.CrawlAndGetDataUsageHandler))
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodMakeVol).HandlerFunc(httpTraceHdrs(server.MakeVolHandler)).Queries(restQueries(storageRESTVolume)...)
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodStatVol).HandlerFunc(httpTraceHdrs(server.StatVolHandler)).Queries(restQueries(storageRESTVolume)...)
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodDeleteVol).HandlerFunc(httpTraceHdrs(server.DeleteVolHandler)).Queries(restQueries(storageRESTVolume)...)
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodListVols).HandlerFunc(httpTraceHdrs(server.ListVolsHandler))

			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodAppendFile).HandlerFunc(httpTraceHdrs(server.AppendFileHandler)).
				Queries(restQueries(storageRESTVolume, storageRESTFilePath)...)
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodWriteAll).HandlerFunc(httpTraceHdrs(server.WriteAllHandler)).
				Queries(restQueries(storageRESTVolume, storageRESTFilePath)...)
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodCreateFile).HandlerFunc(httpTraceHdrs(server.CreateFileHandler)).
				Queries(restQueries(storageRESTVolume, storageRESTFilePath, storageRESTLength)...)

			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodStatFile).HandlerFunc(httpTraceHdrs(server.StatFileHandler)).
				Queries(restQueries(storageRESTVolume, storageRESTFilePath)...)
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodReadAll).HandlerFunc(httpTraceHdrs(server.ReadAllHandler)).
				Queries(restQueries(storageRESTVolume, storageRESTFilePath)...)
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodReadFile).HandlerFunc(httpTraceHdrs(server.ReadFileHandler)).
				Queries(restQueries(storageRESTVolume, storageRESTFilePath, storageRESTOffset, storageRESTLength, storageRESTBitrotAlgo, storageRESTBitrotHash)...)
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodReadFileStream).HandlerFunc(httpTraceHdrs(server.ReadFileStreamHandler)).
				Queries(restQueries(storageRESTVolume, storageRESTFilePath, storageRESTOffset, storageRESTLength)...)
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodListDir).HandlerFunc(httpTraceHdrs(server.ListDirHandler)).
				Queries(restQueries(storageRESTVolume, storageRESTDirPath, storageRESTCount, storageRESTLeafFile)...)
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodWalk).HandlerFunc(httpTraceHdrs(server.WalkHandler)).
				Queries(restQueries(storageRESTVolume, storageRESTDirPath, storageRESTMarkerPath, storageRESTRecursive, storageRESTLeafFile)...)
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodDeleteFile).HandlerFunc(httpTraceHdrs(server.DeleteFileHandler)).
				Queries(restQueries(storageRESTVolume, storageRESTFilePath)...)
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodDeleteFileBulk).HandlerFunc(httpTraceHdrs(server.DeleteFileBulkHandler)).
				Queries(restQueries(storageRESTVolume, storageRESTFilePath)...)

			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodRenameFile).HandlerFunc(httpTraceHdrs(server.RenameFileHandler)).
				Queries(restQueries(storageRESTSrcVolume, storageRESTSrcPath, storageRESTDstVolume, storageRESTDstPath)...)
			subrouter.Methods(http.MethodPost).Path(storageRESTVersionPrefix + storageRESTMethodVerifyFile).HandlerFunc(httpTraceHdrs(server.VerifyFile)).
				Queries(restQueries(storageRESTVolume, storageRESTFilePath, storageRESTBitrotAlgo, storageRESTBitrotHash, storageRESTLength, storageRESTShardSize)...)
		}
	}
}
