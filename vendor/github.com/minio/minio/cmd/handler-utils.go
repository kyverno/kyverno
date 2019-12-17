/*
 * MinIO Cloud Storage, (C) 2015, 2016, 2017 MinIO, Inc.
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
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	xhttp "github.com/minio/minio/cmd/http"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/handlers"
	"github.com/minio/minio/pkg/madmin"
)

// Parses location constraint from the incoming reader.
func parseLocationConstraint(r *http.Request) (location string, s3Error APIErrorCode) {
	// If the request has no body with content-length set to 0,
	// we do not have to validate location constraint. Bucket will
	// be created at default region.
	locationConstraint := createBucketLocationConfiguration{}
	err := xmlDecoder(r.Body, &locationConstraint, r.ContentLength)
	if err != nil && r.ContentLength != 0 {
		logger.LogIf(context.Background(), err)
		// Treat all other failures as XML parsing errors.
		return "", ErrMalformedXML
	} // else for both err as nil or io.EOF
	location = locationConstraint.Location
	if location == "" {
		location = globalServerRegion
	}
	return location, ErrNone
}

// Validates input location is same as configured region
// of MinIO server.
func isValidLocation(location string) bool {
	return globalServerRegion == "" || globalServerRegion == location
}

// Supported headers that needs to be extracted.
var supportedHeaders = []string{
	"content-type",
	"cache-control",
	"content-language",
	"content-encoding",
	"content-disposition",
	xhttp.AmzStorageClass,
	"expires",
	// Add more supported headers here.
}

// isMetadataDirectiveValid - check if metadata-directive is valid.
func isMetadataDirectiveValid(h http.Header) bool {
	_, ok := h[http.CanonicalHeaderKey(xhttp.AmzMetadataDirective)]
	if ok {
		// Check atleast set metadata-directive is valid.
		return (isMetadataCopy(h) || isMetadataReplace(h))
	}
	// By default if x-amz-metadata-directive is not we
	// treat it as 'COPY' this function returns true.
	return true
}

// Check if the metadata COPY is requested.
func isMetadataCopy(h http.Header) bool {
	return h.Get(xhttp.AmzMetadataDirective) == "COPY"
}

// Check if the metadata REPLACE is requested.
func isMetadataReplace(h http.Header) bool {
	return h.Get(xhttp.AmzMetadataDirective) == "REPLACE"
}

// Splits an incoming path into bucket and object components.
func path2BucketAndObject(path string) (bucket, object string) {
	// Skip the first element if it is '/', split the rest.
	path = strings.TrimPrefix(path, SlashSeparator)
	pathComponents := strings.SplitN(path, SlashSeparator, 2)

	// Save the bucket and object extracted from path.
	switch len(pathComponents) {
	case 1:
		bucket = pathComponents[0]
	case 2:
		bucket = pathComponents[0]
		object = pathComponents[1]
	}
	return bucket, object
}

// userMetadataKeyPrefixes contains the prefixes of used-defined metadata keys.
// All values stored with a key starting with one of the following prefixes
// must be extracted from the header.
var userMetadataKeyPrefixes = []string{
	"X-Amz-Meta-",
	"X-Minio-Meta-",
}

// extractMetadata extracts metadata from HTTP header and HTTP queryString.
func extractMetadata(ctx context.Context, r *http.Request) (metadata map[string]string, err error) {
	query := r.URL.Query()
	header := r.Header
	metadata = make(map[string]string)
	// Extract all query values.
	err = extractMetadataFromMap(ctx, query, metadata)
	if err != nil {
		return nil, err
	}

	// Extract all header values.
	err = extractMetadataFromMap(ctx, header, metadata)
	if err != nil {
		return nil, err
	}

	// Set content-type to default value if it is not set.
	if _, ok := metadata["content-type"]; !ok {
		metadata["content-type"] = "application/octet-stream"
	}
	// Success.
	return metadata, nil
}

// extractMetadata extracts metadata from map values.
func extractMetadataFromMap(ctx context.Context, v map[string][]string, m map[string]string) error {
	if v == nil {
		logger.LogIf(ctx, errInvalidArgument)
		return errInvalidArgument
	}
	// Save all supported headers.
	for _, supportedHeader := range supportedHeaders {
		if value, ok := v[http.CanonicalHeaderKey(supportedHeader)]; ok {
			m[supportedHeader] = value[0]
		} else if value, ok := v[supportedHeader]; ok {
			m[supportedHeader] = value[0]
		}
	}
	for key := range v {
		for _, prefix := range userMetadataKeyPrefixes {
			if !strings.HasPrefix(strings.ToLower(key), strings.ToLower(prefix)) {
				continue
			}
			value, ok := v[key]
			if ok {
				m[key] = strings.Join(value, ",")
				break
			}
		}
	}
	return nil
}

// The Query string for the redirect URL the client is
// redirected on successful upload.
func getRedirectPostRawQuery(objInfo ObjectInfo) string {
	redirectValues := make(url.Values)
	redirectValues.Set("bucket", objInfo.Bucket)
	redirectValues.Set("key", objInfo.Name)
	redirectValues.Set("etag", "\""+objInfo.ETag+"\"")
	return redirectValues.Encode()
}

// Returns access credentials in the request Authorization header.
func getReqAccessCred(r *http.Request, region string) (cred auth.Credentials) {
	cred, _, _ = getReqAccessKeyV4(r, region, serviceS3)
	if cred.AccessKey == "" {
		cred, _, _ = getReqAccessKeyV2(r)
	}
	if cred.AccessKey == "" {
		claims, owner, _ := webRequestAuthenticate(r)
		if owner {
			return globalActiveCred
		}
		cred, _ = globalIAMSys.GetUser(claims.AccessKey())
	}
	return cred
}

// Extract request params to be sent with event notifiation.
func extractReqParams(r *http.Request) map[string]string {
	if r == nil {
		return nil
	}

	region := globalServerRegion
	cred := getReqAccessCred(r, region)

	// Success.
	return map[string]string{
		"region":          region,
		"accessKey":       cred.AccessKey,
		"sourceIPAddress": handlers.GetSourceIP(r),
		// Add more fields here.
	}
}

// Extract response elements to be sent with event notifiation.
func extractRespElements(w http.ResponseWriter) map[string]string {

	return map[string]string{
		"requestId":      w.Header().Get(xhttp.AmzRequestID),
		"content-length": w.Header().Get(xhttp.ContentLength),
		// Add more fields here.
	}
}

// Trims away `aws-chunked` from the content-encoding header if present.
// Streaming signature clients can have custom content-encoding such as
// `aws-chunked,gzip` here we need to only save `gzip`.
// For more refer http://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-streaming.html
func trimAwsChunkedContentEncoding(contentEnc string) (trimmedContentEnc string) {
	if contentEnc == "" {
		return contentEnc
	}
	var newEncs []string
	for _, enc := range strings.Split(contentEnc, ",") {
		if enc != streamingContentEncoding {
			newEncs = append(newEncs, enc)
		}
	}
	return strings.Join(newEncs, ",")
}

// Validate form field size for s3 specification requirement.
func validateFormFieldSize(ctx context.Context, formValues http.Header) error {
	// Iterate over form values
	for k := range formValues {
		// Check if value's field exceeds S3 limit
		if int64(len(formValues.Get(k))) > maxFormFieldSize {
			logger.LogIf(ctx, errSizeUnexpected)
			return errSizeUnexpected
		}
	}

	// Success.
	return nil
}

// Extract form fields and file data from a HTTP POST Policy
func extractPostPolicyFormValues(ctx context.Context, form *multipart.Form) (filePart io.ReadCloser, fileName string, fileSize int64, formValues http.Header, err error) {
	/// HTML Form values
	fileName = ""

	// Canonicalize the form values into http.Header.
	formValues = make(http.Header)
	for k, v := range form.Value {
		formValues[http.CanonicalHeaderKey(k)] = v
	}

	// Validate form values.
	if err = validateFormFieldSize(ctx, formValues); err != nil {
		return nil, "", 0, nil, err
	}

	// this means that filename="" was not specified for file key and Go has
	// an ugly way of handling this situation. Refer here
	// https://golang.org/src/mime/multipart/formdata.go#L61
	if len(form.File) == 0 {
		var b = &bytes.Buffer{}
		for _, v := range formValues["File"] {
			b.WriteString(v)
		}
		fileSize = int64(b.Len())
		filePart = ioutil.NopCloser(b)
		return filePart, fileName, fileSize, formValues, nil
	}

	// Iterator until we find a valid File field and break
	for k, v := range form.File {
		canonicalFormName := http.CanonicalHeaderKey(k)
		if canonicalFormName == "File" {
			if len(v) == 0 {
				logger.LogIf(ctx, errInvalidArgument)
				return nil, "", 0, nil, errInvalidArgument
			}
			// Fetch fileHeader which has the uploaded file information
			fileHeader := v[0]
			// Set filename
			fileName = fileHeader.Filename
			// Open the uploaded part
			filePart, err = fileHeader.Open()
			if err != nil {
				logger.LogIf(ctx, err)
				return nil, "", 0, nil, err
			}
			// Compute file size
			fileSize, err = filePart.(io.Seeker).Seek(0, 2)
			if err != nil {
				logger.LogIf(ctx, err)
				return nil, "", 0, nil, err
			}
			// Reset Seek to the beginning
			_, err = filePart.(io.Seeker).Seek(0, 0)
			if err != nil {
				logger.LogIf(ctx, err)
				return nil, "", 0, nil, err
			}
			// File found and ready for reading
			break
		}
	}
	return filePart, fileName, fileSize, formValues, nil
}

// Log headers and body.
func httpTraceAll(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !globalHTTPTrace.HasSubscribers() {
			f.ServeHTTP(w, r)
			return
		}
		trace := Trace(f, true, w, r)
		globalHTTPTrace.Publish(trace)
	}
}

// Log only the headers.
func httpTraceHdrs(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !globalHTTPTrace.HasSubscribers() {
			f.ServeHTTP(w, r)
			return
		}
		trace := Trace(f, false, w, r)
		globalHTTPTrace.Publish(trace)
	}
}

func collectAPIStats(api string, f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		isS3Request := !strings.HasPrefix(r.URL.Path, minioReservedBucketPath)
		apiStatsWriter := &recordAPIStats{w, UTCNow(), false, 0, isS3Request}

		// Time start before the call is about to start.
		tBefore := UTCNow()

		if isS3Request {
			globalHTTPStats.currentS3Requests.Inc(api)
		}
		// Execute the request
		f.ServeHTTP(apiStatsWriter, r)

		if isS3Request {
			globalHTTPStats.currentS3Requests.Dec(api)
		}

		// Firstbyte read.
		tAfter := apiStatsWriter.TTFB

		// Time duration in secs since the call started.
		//
		// We don't need to do nanosecond precision in this
		// simply for the fact that it is not human readable.
		durationSecs := tAfter.Sub(tBefore).Seconds()

		// Update http statistics
		globalHTTPStats.updateStats(api, r, apiStatsWriter, durationSecs)
	}
}

// Returns "/bucketName/objectName" for path-style or virtual-host-style requests.
func getResource(path string, host string, domains []string) (string, error) {
	if len(domains) == 0 {
		return path, nil
	}
	// If virtual-host-style is enabled construct the "resource" properly.
	if strings.Contains(host, ":") {
		// In bucket.mydomain.com:9000, strip out :9000
		var err error
		if host, _, err = net.SplitHostPort(host); err != nil {
			reqInfo := (&logger.ReqInfo{}).AppendTags("host", host)
			reqInfo.AppendTags("path", path)
			ctx := logger.SetReqInfo(context.Background(), reqInfo)
			logger.LogIf(ctx, err)
			return "", err
		}
	}
	for _, domain := range domains {
		if !strings.HasSuffix(host, "."+domain) {
			continue
		}
		bucket := strings.TrimSuffix(host, "."+domain)
		return SlashSeparator + pathJoin(bucket, path), nil
	}
	return path, nil
}

var regexVersion = regexp.MustCompile(`(\w\d+)`)

func extractAPIVersion(r *http.Request) string {
	return regexVersion.FindString(r.URL.Path)
}

// If none of the http routes match respond with appropriate errors
func errorResponseHandler(w http.ResponseWriter, r *http.Request) {
	version := extractAPIVersion(r)
	switch {
	case strings.HasPrefix(r.URL.Path, peerRESTPrefix):
		desc := fmt.Sprintf("Expected 'peer' API version '%s', instead found '%s', please upgrade the servers",
			peerRESTVersion, version)
		writeErrorResponseString(r.Context(), w, APIError{
			Code:           "XMinioPeerVersionMismatch",
			Description:    desc,
			HTTPStatusCode: http.StatusBadRequest,
		}, r.URL)
	case strings.HasPrefix(r.URL.Path, storageRESTPrefix):
		desc := fmt.Sprintf("Expected 'storage' API version '%s', instead found '%s', please upgrade the servers",
			storageRESTVersion, version)
		writeErrorResponseString(r.Context(), w, APIError{
			Code:           "XMinioStorageVersionMismatch",
			Description:    desc,
			HTTPStatusCode: http.StatusBadRequest,
		}, r.URL)
	case strings.HasPrefix(r.URL.Path, lockRESTPrefix):
		desc := fmt.Sprintf("Expected 'lock' API version '%s', instead found '%s', please upgrade the servers",
			lockRESTVersion, version)
		writeErrorResponseString(r.Context(), w, APIError{
			Code:           "XMinioLockVersionMismatch",
			Description:    desc,
			HTTPStatusCode: http.StatusBadRequest,
		}, r.URL)
	case strings.HasPrefix(r.URL.Path, adminPathPrefix):
		var desc string
		if version == "v1" {
			desc = fmt.Sprintf("Server expects client requests with 'admin' API version '%s', found '%s', please upgrade the client to latest releases", madmin.AdminAPIVersion, version)
		} else if version == madmin.AdminAPIVersion {
			desc = fmt.Sprintf("This 'admin' API is not supported by server in '%s'", getMinioMode())
		} else {
			desc = fmt.Sprintf("Unexpected client 'admin' API version found '%s', expected '%s', please downgrade the client to older releases", version, madmin.AdminAPIVersion)
		}
		writeErrorResponseJSON(r.Context(), w, APIError{
			Code:           "XMinioAdminVersionMismatch",
			Description:    desc,
			HTTPStatusCode: http.StatusBadRequest,
		}, r.URL)
	default:
		desc := fmt.Sprintf("Unknown API request at %s", r.URL.Path)
		writeErrorResponse(r.Context(), w, APIError{
			Code:           "XMinioUnknownAPIRequest",
			Description:    desc,
			HTTPStatusCode: http.StatusBadRequest,
		}, r.URL, guessIsBrowserReq(r))
	}
}

// gets host name for current node
func getHostName(r *http.Request) (hostName string) {
	if globalIsDistXL {
		hostName = GetLocalPeer(globalEndpoints)
	} else {
		hostName = r.Host
	}
	return
}
