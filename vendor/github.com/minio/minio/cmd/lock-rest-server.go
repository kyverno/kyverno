/*
 * Minio Cloud Storage, (C) 2019 Minio, Inc.
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
	"errors"
	"math/rand"
	"net/http"
	"path"
	"time"

	"github.com/gorilla/mux"
	"github.com/minio/minio/pkg/dsync"
)

const (
	// Lock maintenance interval.
	lockMaintenanceInterval = 30 * time.Second

	// Lock validity check interval.
	lockValidityCheckInterval = 2 * time.Minute
)

// To abstract a node over network.
type lockRESTServer struct {
	ll *localLocker
}

func (l *lockRESTServer) writeErrorResponse(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(err.Error()))
}

// IsValid - To authenticate and verify the time difference.
func (l *lockRESTServer) IsValid(w http.ResponseWriter, r *http.Request) bool {
	if err := storageServerRequestValidate(r); err != nil {
		l.writeErrorResponse(w, err)
		return false
	}
	return true
}

func getLockArgs(r *http.Request) dsync.LockArgs {
	return dsync.LockArgs{
		UID:      r.URL.Query().Get(lockRESTUID),
		Source:   r.URL.Query().Get(lockRESTSource),
		Resource: r.URL.Query().Get(lockRESTResource),
	}
}

// LockHandler - Acquires a lock.
func (l *lockRESTServer) LockHandler(w http.ResponseWriter, r *http.Request) {
	if !l.IsValid(w, r) {
		l.writeErrorResponse(w, errors.New("Invalid request"))
		return
	}

	success, err := l.ll.Lock(getLockArgs(r))
	if err == nil && !success {
		err = errLockConflict
	}
	if err != nil {
		l.writeErrorResponse(w, err)
		return
	}
}

// UnlockHandler - releases the acquired lock.
func (l *lockRESTServer) UnlockHandler(w http.ResponseWriter, r *http.Request) {
	if !l.IsValid(w, r) {
		l.writeErrorResponse(w, errors.New("Invalid request"))
		return
	}

	_, err := l.ll.Unlock(getLockArgs(r))
	// Ignore the Unlock() "reply" return value because if err == nil, "reply" is always true
	// Consequently, if err != nil, reply is always false
	if err != nil {
		l.writeErrorResponse(w, err)
		return
	}
}

// LockHandler - Acquires an RLock.
func (l *lockRESTServer) RLockHandler(w http.ResponseWriter, r *http.Request) {
	if !l.IsValid(w, r) {
		l.writeErrorResponse(w, errors.New("Invalid request"))
		return
	}

	success, err := l.ll.RLock(getLockArgs(r))
	if err == nil && !success {
		err = errLockConflict
	}
	if err != nil {
		l.writeErrorResponse(w, err)
		return
	}
}

// RUnlockHandler - releases the acquired read lock.
func (l *lockRESTServer) RUnlockHandler(w http.ResponseWriter, r *http.Request) {
	if !l.IsValid(w, r) {
		l.writeErrorResponse(w, errors.New("Invalid request"))
		return
	}

	// Ignore the RUnlock() "reply" return value because if err == nil, "reply" is always true.
	// Consequently, if err != nil, reply is always false
	_, err := l.ll.RUnlock(getLockArgs(r))
	if err != nil {
		l.writeErrorResponse(w, err)
		return
	}
}

// ExpiredHandler - query expired lock status.
func (l *lockRESTServer) ExpiredHandler(w http.ResponseWriter, r *http.Request) {
	if !l.IsValid(w, r) {
		l.writeErrorResponse(w, errors.New("Invalid request"))
		return
	}

	lockArgs := getLockArgs(r)

	l.ll.mutex.Lock()
	defer l.ll.mutex.Unlock()
	// Lock found, proceed to verify if belongs to given uid.
	if lri, ok := l.ll.lockMap[lockArgs.Resource]; ok {
		// Check whether uid is still active
		for _, entry := range lri {
			if entry.UID == lockArgs.UID {
				l.writeErrorResponse(w, errLockNotExpired)
				return
			}
		}
	}
}

// nameLockRequesterInfoPair is a helper type for lock maintenance
type nameLockRequesterInfoPair struct {
	name string
	lri  lockRequesterInfo
}

// getLongLivedLocks returns locks that are older than a certain time and
// have not been 'checked' for validity too soon enough
func getLongLivedLocks(interval time.Duration) map[Endpoint][]nameLockRequesterInfoPair {
	nlripMap := make(map[Endpoint][]nameLockRequesterInfoPair)
	for endpoint, locker := range globalLockServers {
		rslt := []nameLockRequesterInfoPair{}
		locker.mutex.Lock()
		for name, lriArray := range locker.lockMap {
			for idx := range lriArray {
				// Check whether enough time has gone by since last check
				if time.Since(lriArray[idx].TimeLastCheck) >= interval {
					rslt = append(rslt, nameLockRequesterInfoPair{name: name, lri: lriArray[idx]})
					lriArray[idx].TimeLastCheck = UTCNow()
				}
			}
		}
		nlripMap[endpoint] = rslt
		locker.mutex.Unlock()
	}
	return nlripMap
}

// lockMaintenance loops over locks that have been active for some time and checks back
// with the original server whether it is still alive or not
//
// Following logic inside ignores the errors generated for Dsync.Active operation.
// - server at client down
// - some network error (and server is up normally)
//
// We will ignore the error, and we will retry later to get a resolve on this lock
func lockMaintenance(interval time.Duration) {
	// Validate if long lived locks are indeed clean.
	// Get list of long lived locks to check for staleness.
	for lendpoint, nlrips := range getLongLivedLocks(interval) {
		for _, nlrip := range nlrips {
			for _, ep := range globalEndpoints {
				for _, endpoint := range ep.Endpoints {
					if endpoint.String() == lendpoint.String() {
						continue
					}

					c := newLockAPI(endpoint)
					if !c.IsOnline() {
						continue
					}

					// Call back to original server verify whether the lock is still active (based on name & uid)
					expired, err := c.Expired(dsync.LockArgs{
						UID:      nlrip.lri.UID,
						Resource: nlrip.name,
					})

					if err != nil {
						c.Close()
						continue
					}

					// For successful response, verify if lock was indeed active or stale.
					if expired {
						// The lock is no longer active at server that originated
						// the lock, attempt to remove the lock.
						globalLockServers[lendpoint].mutex.Lock()
						// Purge the stale entry if it exists.
						globalLockServers[lendpoint].removeEntryIfExists(nlrip)
						globalLockServers[lendpoint].mutex.Unlock()
					}

					// Close the connection regardless of the call response.
					c.Close()
				}
			}
		}
	}
}

// Start lock maintenance from all lock servers.
func startLockMaintenance() {
	// Initialize a new ticker with a minute between each ticks.
	ticker := time.NewTicker(lockMaintenanceInterval)
	// Stop the timer upon service closure and cleanup the go-routine.
	defer ticker.Stop()

	for {
		// Verifies every minute for locks held more than 2 minutes.
		select {
		case <-GlobalServiceDoneCh:
			return
		case <-ticker.C:
			// Start with random sleep time, so as to avoid
			// "synchronous checks" between servers
			r := rand.New(rand.NewSource(UTCNow().UnixNano()))
			duration := time.Duration(r.Float64() * float64(lockMaintenanceInterval))
			time.Sleep(duration)
			lockMaintenance(lockValidityCheckInterval)
		}
	}
}

// registerLockRESTHandlers - register lock rest router.
func registerLockRESTHandlers(router *mux.Router, endpointZones EndpointZones) {
	queries := restQueries(lockRESTUID, lockRESTSource, lockRESTResource)
	for _, ep := range endpointZones {
		for _, endpoint := range ep.Endpoints {
			if !endpoint.IsLocal {
				continue
			}

			lockServer := &lockRESTServer{
				ll: newLocker(endpoint),
			}

			subrouter := router.PathPrefix(path.Join(lockRESTPrefix, endpoint.Path)).Subrouter()
			subrouter.Methods(http.MethodPost).Path(lockRESTVersionPrefix + lockRESTMethodLock).HandlerFunc(httpTraceHdrs(lockServer.LockHandler)).Queries(queries...)
			subrouter.Methods(http.MethodPost).Path(lockRESTVersionPrefix + lockRESTMethodRLock).HandlerFunc(httpTraceHdrs(lockServer.RLockHandler)).Queries(queries...)
			subrouter.Methods(http.MethodPost).Path(lockRESTVersionPrefix + lockRESTMethodUnlock).HandlerFunc(httpTraceHdrs(lockServer.UnlockHandler)).Queries(queries...)
			subrouter.Methods(http.MethodPost).Path(lockRESTVersionPrefix + lockRESTMethodRUnlock).HandlerFunc(httpTraceHdrs(lockServer.RUnlockHandler)).Queries(queries...)
			subrouter.Methods(http.MethodPost).Path(lockRESTVersionPrefix + lockRESTMethodExpired).HandlerFunc(httpTraceAll(lockServer.ExpiredHandler)).Queries(queries...)

			globalLockServers[endpoint] = lockServer.ll
		}
	}

	go startLockMaintenance()
}
