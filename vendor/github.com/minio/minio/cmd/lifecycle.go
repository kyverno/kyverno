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
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/lifecycle"
)

const (

	// Disabled means the lifecycle rule is inactive
	Disabled = "Disabled"
)

// LifecycleSys - Bucket lifecycle subsystem.
type LifecycleSys struct {
	sync.RWMutex
	bucketLifecycleMap map[string]lifecycle.Lifecycle
}

// Set - sets lifecycle config to given bucket name.
func (sys *LifecycleSys) Set(bucketName string, lifecycle lifecycle.Lifecycle) {
	if globalIsGateway {
		// no-op
		return
	}

	sys.Lock()
	defer sys.Unlock()

	sys.bucketLifecycleMap[bucketName] = lifecycle
}

// Get - gets lifecycle config associated to a given bucket name.
func (sys *LifecycleSys) Get(bucketName string) (lifecycle lifecycle.Lifecycle, ok bool) {
	if globalIsGateway {
		// When gateway is enabled, no cached value
		// is used to validate life cycle policies.
		objAPI := newObjectLayerWithoutSafeModeFn()
		if objAPI == nil {
			return
		}
		l, err := objAPI.GetBucketLifecycle(context.Background(), bucketName)
		return *l, err == nil
	}
	sys.Lock()
	defer sys.Unlock()

	l, ok := sys.bucketLifecycleMap[bucketName]
	return l, ok
}

func saveLifecycleConfig(ctx context.Context, objAPI ObjectLayer, bucketName string, bucketLifecycle *lifecycle.Lifecycle) error {
	data, err := xml.Marshal(bucketLifecycle)
	if err != nil {
		return err
	}

	// Construct path to lifecycle.xml for the given bucket.
	configFile := path.Join(bucketConfigPrefix, bucketName, bucketLifecycleConfig)
	return saveConfig(ctx, objAPI, configFile, data)
}

// getLifecycleConfig - get lifecycle config for given bucket name.
func getLifecycleConfig(objAPI ObjectLayer, bucketName string) (*lifecycle.Lifecycle, error) {
	// Construct path to lifecycle.xml for the given bucket.
	configFile := path.Join(bucketConfigPrefix, bucketName, bucketLifecycleConfig)
	configData, err := readConfig(context.Background(), objAPI, configFile)
	if err != nil {
		if err == errConfigNotFound {
			err = BucketLifecycleNotFound{Bucket: bucketName}
		}
		return nil, err
	}

	return lifecycle.ParseLifecycleConfig(bytes.NewReader(configData))
}

func removeLifecycleConfig(ctx context.Context, objAPI ObjectLayer, bucketName string) error {
	// Construct path to lifecycle.xml for the given bucket.
	configFile := path.Join(bucketConfigPrefix, bucketName, bucketLifecycleConfig)

	if err := objAPI.DeleteObject(ctx, minioMetaBucket, configFile); err != nil {
		if _, ok := err.(ObjectNotFound); ok {
			return BucketLifecycleNotFound{Bucket: bucketName}
		}
		return err
	}
	return nil
}

// NewLifecycleSys - creates new lifecycle system.
func NewLifecycleSys() *LifecycleSys {
	return &LifecycleSys{
		bucketLifecycleMap: make(map[string]lifecycle.Lifecycle),
	}
}

// Init - initializes lifecycle system from lifecycle.xml of all buckets.
func (sys *LifecycleSys) Init(buckets []BucketInfo, objAPI ObjectLayer) error {
	if objAPI == nil {
		return errServerNotInitialized
	}

	// In gateway mode, we always fetch the bucket lifecycle configuration from the gateway backend.
	// So, this is a no-op for gateway servers.
	if globalIsGateway {
		return nil
	}

	doneCh := make(chan struct{})
	defer close(doneCh)

	// Initializing lifecycle needs a retry mechanism for
	// the following reasons:
	//  - Read quorum is lost just after the initialization
	//    of the object layer.
	retryTimerCh := newRetryTimerSimple(doneCh)
	for {
		select {
		case <-retryTimerCh:
			// Load LifecycleSys once during boot.
			if err := sys.load(buckets, objAPI); err != nil {
				if err == errDiskNotFound ||
					strings.Contains(err.Error(), InsufficientReadQuorum{}.Error()) ||
					strings.Contains(err.Error(), InsufficientWriteQuorum{}.Error()) {
					logger.Info("Waiting for lifecycle subsystem to be initialized..")
					continue
				}
				return err
			}
			return nil
		case <-globalOSSignalCh:
			return fmt.Errorf("Initializing Lifecycle sub-system gracefully stopped")
		}
	}
}

// Loads lifecycle policies for all buckets into LifecycleSys.
func (sys *LifecycleSys) load(buckets []BucketInfo, objAPI ObjectLayer) error {
	for _, bucket := range buckets {
		config, err := objAPI.GetBucketLifecycle(context.Background(), bucket.Name)
		if err != nil {
			if _, ok := err.(BucketLifecycleNotFound); ok {
				sys.Remove(bucket.Name)
			}
			continue
		}

		sys.Set(bucket.Name, *config)
	}

	return nil
}

// Remove - removes policy for given bucket name.
func (sys *LifecycleSys) Remove(bucketName string) {
	sys.Lock()
	defer sys.Unlock()

	delete(sys.bucketLifecycleMap, bucketName)
}
