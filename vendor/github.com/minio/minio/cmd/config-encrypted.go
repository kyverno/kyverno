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
	"errors"
	"os"
	"strings"
	"unicode/utf8"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/minio/minio/cmd/config"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/env"
	"github.com/minio/minio/pkg/madmin"
)

func handleEncryptedConfigBackend(objAPI ObjectLayer, server bool) error {
	if !server {
		return nil
	}

	// If its server mode or nas gateway, migrate the backend.
	doneCh := make(chan struct{})
	defer close(doneCh)

	var encrypted bool
	var err error

	// Construct path to config/transaction.lock for locking
	transactionConfigPrefix := minioConfigPrefix + "/transaction.lock"

	// Make sure to hold lock for entire migration to avoid
	// such that only one server should migrate the entire config
	// at a given time, this big transaction lock ensures this
	// appropriately. This is also true for rotation of encrypted
	// content.
	objLock := globalNSMutex.NewNSLock(context.Background(), minioMetaBucket, transactionConfigPrefix)
	if err := objLock.GetLock(globalOperationTimeout); err != nil {
		return err
	}
	defer objLock.Unlock()

	// Migrating Config backend needs a retry mechanism for
	// the following reasons:
	//  - Read quorum is lost just after the initialization
	//    of the object layer.
	for range newRetryTimerSimple(doneCh) {
		if encrypted, err = checkBackendEncrypted(objAPI); err != nil {
			if err == errDiskNotFound ||
				strings.Contains(err.Error(), InsufficientReadQuorum{}.Error()) {
				logger.Info("Waiting for config backend to be encrypted..")
				continue
			}
			return err
		}
		break
	}

	if encrypted {
		// backend is encrypted, but credentials are not specified
		// we shall fail right here. if not proceed forward.
		if !globalConfigEncrypted || !globalActiveCred.IsValid() {
			return config.ErrMissingCredentialsBackendEncrypted(nil)
		}
	} else {
		// backend is not yet encrypted, check if encryption of
		// backend is requested if not return nil and proceed
		// forward.
		if !globalConfigEncrypted {
			return nil
		}
		if !globalActiveCred.IsValid() {
			return errInvalidArgument
		}
	}

	activeCredOld, err := getOldCreds()
	if err != nil {
		return err
	}

	// Migrating Config backend needs a retry mechanism for
	// the following reasons:
	//  - Read quorum is lost just after the initialization
	//    of the object layer.
	for range newRetryTimerSimple(doneCh) {
		// Migrate IAM configuration
		if err = migrateConfigPrefixToEncrypted(objAPI, activeCredOld, encrypted); err != nil {
			if err == errDiskNotFound ||
				strings.Contains(err.Error(), InsufficientReadQuorum{}.Error()) ||
				strings.Contains(err.Error(), InsufficientWriteQuorum{}.Error()) {
				logger.Info("Waiting for config backend to be encrypted..")
				continue
			}
			return err
		}
		break
	}
	return nil
}

const (
	backendEncryptedFile = "backend-encrypted"
)

var (
	backendEncryptedMigrationIncomplete = []byte("incomplete")
	backendEncryptedMigrationComplete   = []byte("encrypted")
)

func checkBackendEtcdEncrypted(ctx context.Context, client *etcd.Client) (bool, error) {
	data, err := readKeyEtcd(ctx, client, backendEncryptedFile)
	if err != nil && err != errConfigNotFound {
		return false, err
	}
	return err == nil && bytes.Equal(data, backendEncryptedMigrationComplete), nil
}

func checkBackendEncrypted(objAPI ObjectLayer) (bool, error) {
	data, err := readConfig(context.Background(), objAPI, backendEncryptedFile)
	if err != nil && err != errConfigNotFound {
		return false, err
	}
	return err == nil && bytes.Equal(data, backendEncryptedMigrationComplete), nil
}

// decryptData - decrypts input data with more that one credentials,
func decryptData(edata []byte, creds ...auth.Credentials) ([]byte, error) {
	var err error
	var data []byte
	for _, cred := range creds {
		data, err = madmin.DecryptData(cred.String(), bytes.NewReader(edata))
		if err != nil {
			if err == madmin.ErrMaliciousData {
				continue
			}
			return nil, err
		}
		break
	}
	return data, err
}

func getOldCreds() (activeCredOld auth.Credentials, err error) {
	accessKeyOld := env.Get(config.EnvAccessKeyOld, "")
	secretKeyOld := env.Get(config.EnvSecretKeyOld, "")
	if accessKeyOld != "" && secretKeyOld != "" {
		activeCredOld, err = auth.CreateCredentials(accessKeyOld, secretKeyOld)
		if err != nil {
			return activeCredOld, err
		}
		// Once we have obtained the rotating creds
		os.Unsetenv(config.EnvAccessKeyOld)
		os.Unsetenv(config.EnvSecretKeyOld)
	}
	return activeCredOld, nil
}

func migrateIAMConfigsEtcdToEncrypted(client *etcd.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultContextTimeout)
	defer cancel()

	encrypted, err := checkBackendEtcdEncrypted(ctx, client)
	if err != nil {
		return err
	}

	if encrypted {
		// backend is encrypted, but credentials are not specified
		// we shall fail right here. if not proceed forward.
		if !globalConfigEncrypted || !globalActiveCred.IsValid() {
			return config.ErrMissingCredentialsBackendEncrypted(nil)
		}
	} else {
		// backend is not yet encrypted, check if encryption of
		// backend is requested if not return nil and proceed
		// forward.
		if !globalConfigEncrypted {
			return nil
		}
		if !globalActiveCred.IsValid() {
			return errInvalidArgument
		}
	}

	activeCredOld, err := getOldCreds()
	if err != nil {
		return err
	}

	if encrypted {
		// No key rotation requested, and backend is
		// already encrypted. We proceed without migration.
		if !activeCredOld.IsValid() {
			return nil
		}

		// No real reason to rotate if old and new creds are same.
		if activeCredOld.Equal(globalActiveCred) {
			return nil
		}

		logger.Info("Attempting rotation of encrypted IAM users and policies on etcd with newly supplied credentials")
	} else {
		logger.Info("Attempting encryption of all IAM users and policies on etcd")
	}

	r, err := client.Get(ctx, minioConfigPrefix, etcd.WithPrefix(), etcd.WithKeysOnly())
	if err != nil {
		return err
	}

	if err = saveKeyEtcd(ctx, client, backendEncryptedFile, backendEncryptedMigrationIncomplete); err != nil {
		return err
	}

	for _, kv := range r.Kvs {
		var (
			cdata    []byte
			cencdata []byte
		)
		cdata, err = readKeyEtcd(ctx, client, string(kv.Key))
		if err != nil {
			switch err {
			case errConfigNotFound:
				// Perhaps not present or someone deleted it.
				continue
			}
			return err
		}

		var data []byte
		// Is rotating of creds requested?
		if activeCredOld.IsValid() {
			data, err = decryptData(cdata, activeCredOld, globalActiveCred)
			if err != nil {
				if err == madmin.ErrMaliciousData {
					return config.ErrInvalidRotatingCredentialsBackendEncrypted(nil)
				}
				return err
			}
		} else {
			data = cdata
		}

		if !utf8.Valid(data) {
			return errors.New("config data not in plain-text form")
		}

		cencdata, err = madmin.EncryptData(globalActiveCred.String(), data)
		if err != nil {
			return err
		}

		if err = saveKeyEtcd(ctx, client, string(kv.Key), cencdata); err != nil {
			return err
		}
	}
	if encrypted && globalActiveCred.IsValid() {
		logger.Info("Rotation complete, please make sure to unset MINIO_ACCESS_KEY_OLD and MINIO_SECRET_KEY_OLD envs")
	}
	return saveKeyEtcd(ctx, client, backendEncryptedFile, backendEncryptedMigrationComplete)
}

func migrateConfigPrefixToEncrypted(objAPI ObjectLayer, activeCredOld auth.Credentials, encrypted bool) error {
	if encrypted {
		// No key rotation requested, and backend is
		// already encrypted. We proceed without migration.
		if !activeCredOld.IsValid() {
			return nil
		}

		// No real reason to rotate if old and new creds are same.
		if activeCredOld.Equal(globalActiveCred) {
			return nil
		}
		logger.Info("Attempting rotation of encrypted config, IAM users and policies on MinIO with newly supplied credentials")
	} else {
		logger.Info("Attempting encryption of all config, IAM users and policies on MinIO backend")
	}

	err := saveConfig(context.Background(), objAPI, backendEncryptedFile, backendEncryptedMigrationIncomplete)
	if err != nil {
		return err
	}

	var marker string
	for {
		res, err := objAPI.ListObjects(context.Background(), minioMetaBucket, minioConfigPrefix, marker, "", maxObjectList)
		if err != nil {
			return err
		}
		for _, obj := range res.Objects {
			var (
				cdata    []byte
				cencdata []byte
			)

			cdata, err = readConfig(context.Background(), objAPI, obj.Name)
			if err != nil {
				return err
			}

			var data []byte
			// Is rotating of creds requested?
			if activeCredOld.IsValid() {
				data, err = decryptData(cdata, activeCredOld, globalActiveCred)
				if err != nil {
					if err == madmin.ErrMaliciousData {
						return config.ErrInvalidRotatingCredentialsBackendEncrypted(nil)
					}
					return err
				}
			} else {
				data = cdata
			}

			if !utf8.Valid(data) {
				return errors.New("config data not in plain-text form")
			}

			cencdata, err = madmin.EncryptData(globalActiveCred.String(), data)
			if err != nil {
				return err
			}

			if err = saveConfig(context.Background(), objAPI, obj.Name, cencdata); err != nil {
				return err
			}
		}

		if !res.IsTruncated {
			break
		}

		marker = res.NextMarker
	}

	if encrypted && globalActiveCred.IsValid() {
		logger.Info("Rotation complete, please make sure to unset MINIO_ACCESS_KEY_OLD and MINIO_SECRET_KEY_OLD envs")
	}

	return saveConfig(context.Background(), objAPI, backendEncryptedFile, backendEncryptedMigrationComplete)
}
