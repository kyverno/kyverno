/*
 * MinIO Cloud Storage, (C) 2016, 2017, 2018 MinIO, Inc.
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
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/gorilla/mux"
	xhttp "github.com/minio/minio/cmd/http"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/event"
	"github.com/minio/minio/pkg/event/target"
	xnet "github.com/minio/minio/pkg/net"
	"github.com/minio/minio/pkg/policy"
)

const (
	bucketConfigPrefix       = "buckets"
	bucketNotificationConfig = "notification.xml"
	bucketListenerConfig     = "listener.json"
)

var errNoSuchNotifications = errors.New("The specified bucket does not have bucket notifications")

// GetBucketNotificationHandler - This HTTP handler returns event notification configuration
// as per http://docs.aws.amazon.com/AmazonS3/latest/dev/NotificationHowTo.html.
// It returns empty configuration if its not set.
func (api objectAPIHandlers) GetBucketNotificationHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "GetBucketNotification")

	defer logger.AuditLog(w, r, "GetBucketNotification", mustGetClaimsFromToken(r))

	vars := mux.Vars(r)
	bucketName := vars["bucket"]
	var config *event.Config

	objAPI := api.ObjectAPI()
	if objAPI == nil {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrServerNotInitialized), r.URL, guessIsBrowserReq(r))
		return
	}

	if !objAPI.IsNotificationSupported() {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrNotImplemented), r.URL, guessIsBrowserReq(r))
		return
	}

	if s3Error := checkRequestAuthType(ctx, r, policy.GetBucketNotificationAction, bucketName, ""); s3Error != ErrNone {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(s3Error), r.URL, guessIsBrowserReq(r))
		return
	}

	_, err := objAPI.GetBucketInfo(ctx, bucketName)
	if err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	// Construct path to notification.xml for the given bucket.
	configFile := path.Join(bucketConfigPrefix, bucketName, bucketNotificationConfig)

	configData, err := readConfig(ctx, objAPI, configFile)
	if err != nil {
		if err != errConfigNotFound {
			writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
			return
		}
		config = &event.Config{}
	} else {
		if err = xml.NewDecoder(bytes.NewReader(configData)).Decode(&config); err != nil {
			writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
			return
		}
	}

	config.SetRegion(globalServerRegion)

	// If xml namespace is empty, set a default value before returning.
	if config.XMLNS == "" {
		config.XMLNS = "http://s3.amazonaws.com/doc/2006-03-01/"
	}

	notificationBytes, err := xml.Marshal(config)
	if err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	writeSuccessResponseXML(w, notificationBytes)
}

// PutBucketNotificationHandler - This HTTP handler stores given notification configuration as per
// http://docs.aws.amazon.com/AmazonS3/latest/dev/NotificationHowTo.html.
func (api objectAPIHandlers) PutBucketNotificationHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "PutBucketNotification")

	defer logger.AuditLog(w, r, "PutBucketNotification", mustGetClaimsFromToken(r))

	objectAPI := api.ObjectAPI()
	if objectAPI == nil {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrServerNotInitialized), r.URL, guessIsBrowserReq(r))
		return
	}

	if !objectAPI.IsNotificationSupported() {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrNotImplemented), r.URL, guessIsBrowserReq(r))
		return
	}

	vars := mux.Vars(r)
	bucketName := vars["bucket"]

	if s3Error := checkRequestAuthType(ctx, r, policy.PutBucketNotificationAction, bucketName, ""); s3Error != ErrNone {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(s3Error), r.URL, guessIsBrowserReq(r))
		return
	}

	_, err := objectAPI.GetBucketInfo(ctx, bucketName)
	if err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	// PutBucketNotification always needs a Content-Length.
	if r.ContentLength <= 0 {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrMissingContentLength), r.URL, guessIsBrowserReq(r))
		return
	}

	var config *event.Config
	config, err = event.ParseConfig(io.LimitReader(r.Body, r.ContentLength), globalServerRegion, globalNotificationSys.targetList)
	if err != nil {
		apiErr := errorCodes.ToAPIErr(ErrMalformedXML)
		if event.IsEventError(err) {
			apiErr = toAPIError(ctx, err)
		}
		if _, ok := err.(*event.ErrARNNotFound); !ok {
			writeErrorResponse(ctx, w, apiErr, r.URL, guessIsBrowserReq(r))
			return
		}
	}

	if err = saveNotificationConfig(ctx, objectAPI, bucketName, config); err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	rulesMap := config.ToRulesMap()
	globalNotificationSys.AddRulesMap(bucketName, rulesMap)
	globalNotificationSys.PutBucketNotification(ctx, bucketName, rulesMap)

	writeSuccessResponseHeadersOnly(w)
}

func (api objectAPIHandlers) ListenBucketNotificationHandlerV2(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "ListenBucketNotificationV2")

	defer logger.AuditLog(w, r, "ListenBucketNotificationV2", mustGetClaimsFromToken(r))

	// Validate if bucket exists.
	objAPI := api.ObjectAPI()
	if objAPI == nil {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrServerNotInitialized), r.URL, guessIsBrowserReq(r))
		return
	}

	if !objAPI.IsNotificationSupported() {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrNotImplemented), r.URL, guessIsBrowserReq(r))
		return
	}

	if !objAPI.IsListenBucketSupported() {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrNotImplemented), r.URL, guessIsBrowserReq(r))
		return
	}

	vars := mux.Vars(r)
	bucketName := vars["bucket"]

	values := r.URL.Query()

	var prefix string
	if len(values["prefix"]) > 1 {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrFilterNamePrefix), r.URL, guessIsBrowserReq(r))
		return
	}

	if len(values["prefix"]) == 1 {
		if err := event.ValidateFilterRuleValue(values["prefix"][0]); err != nil {
			writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
			return
		}

		prefix = values["prefix"][0]
	}

	var suffix string
	if len(values["suffix"]) > 1 {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrFilterNameSuffix), r.URL, guessIsBrowserReq(r))
		return
	}

	if len(values["suffix"]) == 1 {
		if err := event.ValidateFilterRuleValue(values["suffix"][0]); err != nil {
			writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
			return
		}

		suffix = values["suffix"][0]
	}

	pattern := event.NewPattern(prefix, suffix)

	eventNames := []event.Name{}
	for _, s := range values["events"] {
		eventName, err := event.ParseName(s)
		if err != nil {
			writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
			return
		}

		eventNames = append(eventNames, eventName)
	}

	if _, err := objAPI.GetBucketInfo(ctx, bucketName); err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	host, err := xnet.ParseHost(r.RemoteAddr)
	if err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	rulesMap := event.NewRulesMap(eventNames, pattern,
		event.TargetID{ID: "listen" + "+" + mustGetUUID() + "+" + host.Name, Name: host.Port.String()})

	w.Header().Set(xhttp.ContentType, "text/event-stream")

	doneCh := make(chan struct{})
	defer close(doneCh)

	// Listen Publisher and peer-listen-client uses nonblocking send and hence does not wait for slow receivers.
	// Use buffered channel to take care of burst sends or slow w.Write()
	listenCh := make(chan interface{}, 4000)

	peers := getRestClients(globalEndpoints)

	globalHTTPListen.Subscribe(listenCh, doneCh, func(evI interface{}) bool {
		ev, ok := evI.(event.Event)
		if !ok {
			return false
		}
		objectName, uerr := url.QueryUnescape(ev.S3.Object.Key)
		if uerr != nil {
			return false
		}
		return len(rulesMap.Match(ev.EventName, objectName).ToSlice()) != 0
	})

	for _, peer := range peers {
		if peer == nil {
			continue
		}
		peer.Listen(listenCh, doneCh)
	}

	keepAliveTicker := time.NewTicker(500 * time.Millisecond)
	defer keepAliveTicker.Stop()

	enc := json.NewEncoder(w)
	for {
		select {
		case evI := <-listenCh:
			ev := evI.(event.Event)
			if err := enc.Encode(struct{ Records []event.Event }{[]event.Event{ev}}); err != nil {
				return
			}
			w.(http.Flusher).Flush()
		case <-keepAliveTicker.C:
			if _, err := w.Write([]byte(" ")); err != nil {
				return
			}
			w.(http.Flusher).Flush()
		case <-GlobalServiceDoneCh:
			return
		}
	}

}

// ListenBucketNotificationHandler - This HTTP handler sends events to the connected HTTP client.
// Client should send prefix/suffix object name to match and events to watch as query parameters.
func (api objectAPIHandlers) ListenBucketNotificationHandler(w http.ResponseWriter, r *http.Request) {
	ctx := newContext(r, w, "ListenBucketNotification")

	defer logger.AuditLog(w, r, "ListenBucketNotification", mustGetClaimsFromToken(r))

	// Validate if bucket exists.
	objAPI := api.ObjectAPI()
	if objAPI == nil {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrServerNotInitialized), r.URL, guessIsBrowserReq(r))
		return
	}

	if !objAPI.IsNotificationSupported() {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrNotImplemented), r.URL, guessIsBrowserReq(r))
		return
	}

	if !objAPI.IsListenBucketSupported() {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrNotImplemented), r.URL, guessIsBrowserReq(r))
		return
	}
	vars := mux.Vars(r)
	bucketName := vars["bucket"]

	if s3Error := checkRequestAuthType(ctx, r, policy.ListenBucketNotificationAction, bucketName, ""); s3Error != ErrNone {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(s3Error), r.URL, guessIsBrowserReq(r))
		return
	}

	values := r.URL.Query()

	var prefix string
	if len(values["prefix"]) > 1 {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrFilterNamePrefix), r.URL, guessIsBrowserReq(r))
		return
	}

	if len(values["prefix"]) == 1 {
		if err := event.ValidateFilterRuleValue(values["prefix"][0]); err != nil {
			writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
			return
		}

		prefix = values["prefix"][0]
	}

	var suffix string
	if len(values["suffix"]) > 1 {
		writeErrorResponse(ctx, w, errorCodes.ToAPIErr(ErrFilterNameSuffix), r.URL, guessIsBrowserReq(r))
		return
	}

	if len(values["suffix"]) == 1 {
		if err := event.ValidateFilterRuleValue(values["suffix"][0]); err != nil {
			writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
			return
		}

		suffix = values["suffix"][0]
	}

	pattern := event.NewPattern(prefix, suffix)

	eventNames := []event.Name{}
	for _, s := range values["events"] {
		eventName, err := event.ParseName(s)
		if err != nil {
			writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
			return
		}

		eventNames = append(eventNames, eventName)
	}

	if _, err := objAPI.GetBucketInfo(ctx, bucketName); err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	host, err := xnet.ParseHost(r.RemoteAddr)
	if err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	w.Header().Set(xhttp.ContentType, "text/event-stream")

	target, err := target.NewHTTPClientTarget(*host, w)
	if err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	rulesMap := event.NewRulesMap(eventNames, pattern, target.ID())

	if err = globalNotificationSys.AddRemoteTarget(bucketName, target, rulesMap); err != nil {
		logger.GetReqInfo(ctx).AppendTags("target", target.ID().Name)
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}
	defer globalNotificationSys.RemoveRemoteTarget(bucketName, target.ID())
	defer globalNotificationSys.RemoveRulesMap(bucketName, rulesMap)

	thisAddr, err := xnet.ParseHost(GetLocalPeer(globalEndpoints))
	if err != nil {
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	if err = SaveListener(objAPI, bucketName, eventNames, pattern, target.ID(), *thisAddr); err != nil {
		logger.GetReqInfo(ctx).AppendTags("target", target.ID().Name)
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}

	globalNotificationSys.ListenBucketNotification(ctx, bucketName, eventNames, pattern, target.ID(), *thisAddr)

	<-target.DoneCh

	if err = RemoveListener(objAPI, bucketName, target.ID(), *thisAddr); err != nil {
		logger.GetReqInfo(ctx).AppendTags("target", target.ID().Name)
		writeErrorResponse(ctx, w, toAPIError(ctx, err), r.URL, guessIsBrowserReq(r))
		return
	}
}
