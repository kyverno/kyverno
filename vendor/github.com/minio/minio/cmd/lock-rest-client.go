/*
 * MinIO Cloud Storage, (C) 2019 MinIO, Inc.
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
	"crypto/tls"
	"errors"
	"io"
	"sync/atomic"
	"time"

	"net/url"

	"github.com/minio/minio/cmd/http"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/cmd/rest"
	"github.com/minio/minio/pkg/dsync"
)

// lockRESTClient is authenticable lock REST client
type lockRESTClient struct {
	restClient *rest.Client
	endpoint   Endpoint
	connected  int32
}

func toLockError(err error) error {
	if err == nil {
		return nil
	}

	switch err.Error() {
	case errLockConflict.Error():
		return errLockConflict
	case errLockNotExpired.Error():
		return errLockNotExpired
	}
	return err
}

// String stringer *dsync.NetLocker* interface compatible method.
func (client *lockRESTClient) String() string {
	return client.endpoint.String()
}

// Wrapper to restClient.Call to handle network errors, in case of network error the connection is marked disconnected
// permanently. The only way to restore the connection is at the xl-sets layer by xlsets.monitorAndConnectEndpoints()
// after verifying format.json
func (client *lockRESTClient) call(method string, values url.Values, body io.Reader, length int64) (respBody io.ReadCloser, err error) {
	if !client.IsOnline() {
		return nil, errors.New("Lock rest server node is down")
	}

	if values == nil {
		values = make(url.Values)
	}

	respBody, err = client.restClient.Call(method, values, body, length)
	if err == nil {
		return respBody, nil
	}

	if isNetworkError(err) {
		time.AfterFunc(defaultRetryUnit*3, func() {
			// After 3 seconds, take this lock client online for a retry.
			atomic.StoreInt32(&client.connected, 1)
		})

		atomic.StoreInt32(&client.connected, 0)
	}

	return nil, toLockError(err)
}

// IsOnline - returns whether REST client failed to connect or not.
func (client *lockRESTClient) IsOnline() bool {
	return atomic.LoadInt32(&client.connected) == 1
}

// Close - marks the client as closed.
func (client *lockRESTClient) Close() error {
	atomic.StoreInt32(&client.connected, 0)
	client.restClient.Close()
	return nil
}

// restCall makes a call to the lock REST server.
func (client *lockRESTClient) restCall(call string, args dsync.LockArgs) (reply bool, err error) {
	values := url.Values{}
	values.Set(lockRESTUID, args.UID)
	values.Set(lockRESTSource, args.Source)
	values.Set(lockRESTResource, args.Resource)

	respBody, err := client.call(call, values, nil, -1)
	defer http.DrainBody(respBody)
	switch err {
	case nil:
		return true, nil
	case errLockConflict, errLockNotExpired:
		return false, nil
	default:
		return false, err
	}
}

// RLock calls read lock REST API.
func (client *lockRESTClient) RLock(args dsync.LockArgs) (reply bool, err error) {
	return client.restCall(lockRESTMethodRLock, args)
}

// Lock calls lock REST API.
func (client *lockRESTClient) Lock(args dsync.LockArgs) (reply bool, err error) {
	return client.restCall(lockRESTMethodLock, args)
}

// RUnlock calls read unlock REST API.
func (client *lockRESTClient) RUnlock(args dsync.LockArgs) (reply bool, err error) {
	return client.restCall(lockRESTMethodRUnlock, args)
}

// Unlock calls write unlock RPC.
func (client *lockRESTClient) Unlock(args dsync.LockArgs) (reply bool, err error) {
	return client.restCall(lockRESTMethodUnlock, args)
}

// Expired calls expired handler to check if lock args have expired.
func (client *lockRESTClient) Expired(args dsync.LockArgs) (expired bool, err error) {
	return client.restCall(lockRESTMethodExpired, args)
}

func closeLockers(lockers []dsync.NetLocker) {
	for _, locker := range lockers {
		locker.Close()
	}
}

func newLockAPI(endpoint Endpoint) dsync.NetLocker {
	if endpoint.IsLocal {
		return globalLockServers[endpoint]
	}
	return newlockRESTClient(endpoint)
}

// Returns a lock rest client.
func newlockRESTClient(endpoint Endpoint) *lockRESTClient {
	serverURL := &url.URL{
		Scheme: endpoint.Scheme,
		Host:   endpoint.Host,
		Path:   pathJoin(lockRESTPrefix, endpoint.Path, lockRESTVersion),
	}

	var tlsConfig *tls.Config
	if globalIsSSL {
		tlsConfig = &tls.Config{
			ServerName: endpoint.Hostname(),
			RootCAs:    globalRootCAs,
			NextProtos: []string{"http/1.1"}, // Force http1.1
		}
	}

	trFn := newCustomHTTPTransport(tlsConfig, rest.DefaultRESTTimeout, rest.DefaultRESTTimeout)
	restClient, err := rest.NewClient(serverURL, trFn, newAuthToken)
	if err != nil {
		logger.LogIf(context.Background(), err)
		return &lockRESTClient{endpoint: endpoint, restClient: restClient, connected: 0}
	}

	return &lockRESTClient{endpoint: endpoint, restClient: restClient, connected: 1}
}
