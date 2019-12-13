/*
 * Minio Cloud Storage, (C) 2016 Minio, Inc.
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

package dsync

// LockArgs is minimal required values for any dsync compatible lock operation.
type LockArgs struct {
	// Unique ID of lock/unlock request.
	UID string

	// Resource contains a entity to be locked/unlocked.
	Resource string

	// Source contains the line number, function and file name of the code
	// on the client node that requested the lock.
	Source string
}

// NetLocker is dsync compatible locker interface.
type NetLocker interface {
	// Do read lock for given LockArgs.  It should return
	// * a boolean to indicate success/failure of the operation
	// * an error on failure of lock request operation.
	RLock(args LockArgs) (bool, error)

	// Do write lock for given LockArgs. It should return
	// * a boolean to indicate success/failure of the operation
	// * an error on failure of lock request operation.
	Lock(args LockArgs) (bool, error)

	// Do read unlock for given LockArgs. It should return
	// * a boolean to indicate success/failure of the operation
	// * an error on failure of unlock request operation.
	RUnlock(args LockArgs) (bool, error)

	// Do write unlock for given LockArgs. It should return
	// * a boolean to indicate success/failure of the operation
	// * an error on failure of unlock request operation.
	Unlock(args LockArgs) (bool, error)

	// Expired returns if current lock args has expired.
	Expired(args LockArgs) (bool, error)

	// Returns underlying endpoint of this lock client instance.
	String() string

	// Close closes any underlying connection to the service endpoint
	Close() error

	// Is the underlying connection online? (is always true for any local lockers)
	IsOnline() bool
}
