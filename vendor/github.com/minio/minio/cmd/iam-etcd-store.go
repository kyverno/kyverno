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
	"encoding/json"
	"errors"
	"path"
	"strings"
	"sync"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/minio/minio-go/v6/pkg/set"
	"github.com/minio/minio/cmd/logger"
	"github.com/minio/minio/pkg/auth"
	iampolicy "github.com/minio/minio/pkg/iam/policy"
	"github.com/minio/minio/pkg/madmin"
)

var defaultContextTimeout = 30 * time.Second

func etcdKvsToSet(prefix string, kvs []*mvccpb.KeyValue) set.StringSet {
	users := set.NewStringSet()
	for _, kv := range kvs {
		// Extract user by stripping off the `prefix` value as suffix,
		// then strip off the remaining basename to obtain the prefix
		// value, usually in the following form.
		//
		//  key := "config/iam/users/newuser/identity.json"
		//  prefix := "config/iam/users/"
		//  v := trim(trim(key, prefix), base(key)) == "newuser"
		//
		user := path.Clean(strings.TrimSuffix(strings.TrimPrefix(string(kv.Key), prefix), path.Base(string(kv.Key))))
		users.Add(user)
	}
	return users
}

func etcdKvsToSetPolicyDB(prefix string, kvs []*mvccpb.KeyValue) set.StringSet {
	items := set.NewStringSet()
	for _, kv := range kvs {
		// Extract user item by stripping off prefix and then
		// stripping of ".json" suffix.
		//
		// key := "config/iam/policydb/users/myuser1.json"
		// prefix := "config/iam/policydb/users/"
		// v := trimSuffix(trimPrefix(key, prefix), ".json")
		key := string(kv.Key)
		item := path.Clean(strings.TrimSuffix(strings.TrimPrefix(key, prefix), ".json"))
		items.Add(item)
	}
	return items
}

// IAMEtcdStore implements IAMStorageAPI
type IAMEtcdStore struct {
	sync.RWMutex
	ctx context.Context

	client *etcd.Client
}

func newIAMEtcdStore() *IAMEtcdStore {
	return &IAMEtcdStore{client: globalEtcdClient}
}

func (ies *IAMEtcdStore) getContext() context.Context {
	ies.RLock()
	defer ies.RUnlock()

	if ies.ctx == nil {
		return context.Background()
	}
	return ies.ctx
}

func (ies *IAMEtcdStore) setContext(ctx context.Context) {
	ies.Lock()
	defer ies.Unlock()

	ies.ctx = ctx
}

func (ies *IAMEtcdStore) clearContext() {
	ies.Lock()
	defer ies.Unlock()

	ies.ctx = nil
}

func (ies *IAMEtcdStore) saveIAMConfig(item interface{}, path string) error {
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	if globalConfigEncrypted {
		data, err = madmin.EncryptData(globalActiveCred.String(), data)
		if err != nil {
			return err
		}
	}
	return saveKeyEtcd(ies.getContext(), ies.client, path, data)
}

func (ies *IAMEtcdStore) loadIAMConfig(item interface{}, path string) error {
	pdata, err := readKeyEtcd(ies.getContext(), ies.client, path)
	if err != nil {
		return err
	}

	if globalConfigEncrypted {
		pdata, err = madmin.DecryptData(globalActiveCred.String(), bytes.NewReader(pdata))
		if err != nil {
			return err
		}
	}

	return json.Unmarshal(pdata, item)
}

func (ies *IAMEtcdStore) deleteIAMConfig(path string) error {
	return deleteKeyEtcd(ies.getContext(), ies.client, path)
}

func (ies *IAMEtcdStore) migrateUsersConfigToV1(isSTS bool) error {
	basePrefix := iamConfigUsersPrefix
	if isSTS {
		basePrefix = iamConfigSTSPrefix
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultContextTimeout)
	defer cancel()
	ies.setContext(ctx)
	defer ies.clearContext()
	r, err := ies.client.Get(ctx, basePrefix, etcd.WithPrefix(), etcd.WithKeysOnly())
	if err != nil {
		return err
	}

	users := etcdKvsToSet(basePrefix, r.Kvs)
	for _, user := range users.ToSlice() {
		{
			// 1. check if there is a policy file in the old loc.
			oldPolicyPath := pathJoin(basePrefix, user, iamPolicyFile)
			var policyName string
			err := ies.loadIAMConfig(&policyName, oldPolicyPath)
			if err != nil {
				switch err {
				case errConfigNotFound:
					// No mapped policy or already migrated.
				default:
					// corrupt data/read error, etc
				}
				goto next
			}

			// 2. copy policy to new loc.
			mp := newMappedPolicy(policyName)
			path := getMappedPolicyPath(user, isSTS, false)
			if err := ies.saveIAMConfig(mp, path); err != nil {
				return err
			}

			// 3. delete policy file in old loc.
			deleteKeyEtcd(ctx, ies.client, oldPolicyPath)
		}

	next:
		// 4. check if user identity has old format.
		identityPath := pathJoin(basePrefix, user, iamIdentityFile)
		var cred auth.Credentials
		if err := ies.loadIAMConfig(&cred, identityPath); err != nil {
			switch err {
			case errConfigNotFound:
				// This case should not happen.
			default:
				// corrupt file or read error
			}
			continue
		}

		// If the file is already in the new format,
		// then the parsed auth.Credentials will have
		// the zero value for the struct.
		var zeroCred auth.Credentials
		if cred == zeroCred {
			// nothing to do
			continue
		}

		// Found a id file in old format. Copy value
		// into new format and save it.
		cred.AccessKey = user
		u := newUserIdentity(cred)
		if err := ies.saveIAMConfig(u, identityPath); err != nil {
			logger.LogIf(context.Background(), err)
			return err
		}

		// Nothing to delete as identity file location
		// has not changed.
	}
	return nil
}

func (ies *IAMEtcdStore) migrateToV1() error {
	var iamFmt iamFormat
	path := getIAMFormatFilePath()
	if err := ies.loadIAMConfig(&iamFmt, path); err != nil {
		switch err {
		case errConfigNotFound:
			// Need to migrate to V1.
		default:
			return err
		}
	} else {
		if iamFmt.Version >= iamFormatVersion1 {
			// Already migrated to V1 of higher!
			return nil
		}
		// This case should not happen
		// (i.e. Version is 0 or negative.)
		return errors.New("got an invalid IAM format version")

	}

	// Migrate long-term users
	if err := ies.migrateUsersConfigToV1(false); err != nil {
		logger.LogIf(context.Background(), err)
		return err
	}
	// Migrate STS users
	if err := ies.migrateUsersConfigToV1(true); err != nil {
		logger.LogIf(context.Background(), err)
		return err
	}
	// Save iam version file.
	if err := ies.saveIAMConfig(newIAMFormatVersion1(), path); err != nil {
		logger.LogIf(context.Background(), err)
		return err
	}
	return nil
}

// Should be called under config migration lock
func (ies *IAMEtcdStore) migrateBackendFormat(objAPI ObjectLayer) error {
	if err := ies.migrateToV1(); err != nil {
		return err
	}
	return nil
}

func (ies *IAMEtcdStore) loadPolicyDoc(policy string, m map[string]iampolicy.Policy) error {
	var p iampolicy.Policy
	err := ies.loadIAMConfig(&p, getPolicyDocPath(policy))
	if err != nil {
		return err
	}
	m[policy] = p
	return nil
}

func (ies *IAMEtcdStore) loadPolicyDocs(m map[string]iampolicy.Policy) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultContextTimeout)
	defer cancel()
	ies.setContext(ctx)
	defer ies.clearContext()
	r, err := ies.client.Get(ctx, iamConfigPoliciesPrefix, etcd.WithPrefix(), etcd.WithKeysOnly())
	if err != nil {
		return err
	}

	policies := etcdKvsToSet(iamConfigPoliciesPrefix, r.Kvs)

	// Reload config and policies for all policys.
	for _, policyName := range policies.ToSlice() {
		err = ies.loadPolicyDoc(policyName, m)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ies *IAMEtcdStore) loadUser(user string, isSTS bool, m map[string]auth.Credentials) error {
	var u UserIdentity
	err := ies.loadIAMConfig(&u, getUserIdentityPath(user, isSTS))
	if err != nil {
		if err == errConfigNotFound {
			return errNoSuchUser
		}
		return err
	}

	if u.Credentials.IsExpired() {
		// Delete expired identity.
		ctx := ies.getContext()
		deleteKeyEtcd(ctx, ies.client, getUserIdentityPath(user, isSTS))
		deleteKeyEtcd(ctx, ies.client, getMappedPolicyPath(user, isSTS, false))
		return nil
	}

	if u.Credentials.AccessKey == "" {
		u.Credentials.AccessKey = user
	}
	m[user] = u.Credentials
	return nil

}

func (ies *IAMEtcdStore) loadUsers(isSTS bool, m map[string]auth.Credentials) error {
	basePrefix := iamConfigUsersPrefix
	if isSTS {
		basePrefix = iamConfigSTSPrefix
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultContextTimeout)
	defer cancel()
	ies.setContext(ctx)
	defer ies.clearContext()
	r, err := ies.client.Get(ctx, basePrefix, etcd.WithPrefix(), etcd.WithKeysOnly())
	if err != nil {
		return err
	}

	users := etcdKvsToSet(basePrefix, r.Kvs)

	// Reload config for all users.
	for _, user := range users.ToSlice() {
		if err = ies.loadUser(user, isSTS, m); err != nil {
			return err
		}
	}
	return nil
}

func (ies *IAMEtcdStore) loadGroup(group string, m map[string]GroupInfo) error {
	var gi GroupInfo
	err := ies.loadIAMConfig(&gi, getGroupInfoPath(group))
	if err != nil {
		if err == errConfigNotFound {
			return errNoSuchGroup
		}
		return err
	}
	m[group] = gi
	return nil

}

func (ies *IAMEtcdStore) loadGroups(m map[string]GroupInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultContextTimeout)
	defer cancel()
	ies.setContext(ctx)
	defer ies.clearContext()
	r, err := ies.client.Get(ctx, iamConfigGroupsPrefix, etcd.WithPrefix(), etcd.WithKeysOnly())
	if err != nil {
		return err
	}

	groups := etcdKvsToSet(iamConfigGroupsPrefix, r.Kvs)

	// Reload config for all groups.
	for _, group := range groups.ToSlice() {
		if err = ies.loadGroup(group, m); err != nil {
			return err
		}
	}
	return nil

}

func (ies *IAMEtcdStore) loadMappedPolicy(name string, isSTS, isGroup bool, m map[string]MappedPolicy) error {
	var p MappedPolicy
	err := ies.loadIAMConfig(&p, getMappedPolicyPath(name, isSTS, isGroup))
	if err != nil {
		if err == errConfigNotFound {
			return errNoSuchPolicy
		}
		return err
	}
	m[name] = p
	return nil

}

func (ies *IAMEtcdStore) loadMappedPolicies(isSTS, isGroup bool, m map[string]MappedPolicy) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultContextTimeout)
	defer cancel()
	ies.setContext(ctx)
	defer ies.clearContext()
	var basePrefix string
	switch {
	case isSTS:
		basePrefix = iamConfigPolicyDBSTSUsersPrefix
	case isGroup:
		basePrefix = iamConfigPolicyDBGroupsPrefix
	default:
		basePrefix = iamConfigPolicyDBUsersPrefix
	}
	r, err := ies.client.Get(ctx, basePrefix, etcd.WithPrefix(), etcd.WithKeysOnly())
	if err != nil {
		return err
	}

	users := etcdKvsToSetPolicyDB(basePrefix, r.Kvs)

	// Reload config and policies for all users.
	for _, user := range users.ToSlice() {
		if err = ies.loadMappedPolicy(user, isSTS, isGroup, m); err != nil {
			return err
		}
	}
	return nil

}

func (ies *IAMEtcdStore) loadAll(sys *IAMSys, objectAPI ObjectLayer) error {
	iamUsersMap := make(map[string]auth.Credentials)
	iamGroupsMap := make(map[string]GroupInfo)
	iamPolicyDocsMap := make(map[string]iampolicy.Policy)
	iamUserPolicyMap := make(map[string]MappedPolicy)
	iamGroupPolicyMap := make(map[string]MappedPolicy)

	isMinIOUsersSys := false
	sys.RLock()
	if sys.usersSysType == MinIOUsersSysType {
		isMinIOUsersSys = true
	}
	sys.RUnlock()

	if err := ies.loadPolicyDocs(iamPolicyDocsMap); err != nil {
		return err
	}

	// load STS temp users
	if err := ies.loadUsers(true, iamUsersMap); err != nil {
		return err
	}

	if isMinIOUsersSys {
		// load long term users
		if err := ies.loadUsers(false, iamUsersMap); err != nil {
			return err
		}
		if err := ies.loadGroups(iamGroupsMap); err != nil {
			return err
		}
		if err := ies.loadMappedPolicies(false, false, iamUserPolicyMap); err != nil {
			return err
		}
	}

	// load STS policy mappings into the same map
	if err := ies.loadMappedPolicies(true, false, iamUserPolicyMap); err != nil {
		return err
	}
	// load policies mapped to groups
	if err := ies.loadMappedPolicies(false, true, iamGroupPolicyMap); err != nil {
		return err
	}

	// Sets default canned policies, if none are set.
	setDefaultCannedPolicies(iamPolicyDocsMap)

	sys.Lock()
	defer sys.Unlock()

	sys.iamUsersMap = iamUsersMap
	sys.iamGroupsMap = iamGroupsMap
	sys.iamUserPolicyMap = iamUserPolicyMap
	sys.iamPolicyDocsMap = iamPolicyDocsMap
	sys.iamGroupPolicyMap = iamGroupPolicyMap
	sys.buildUserGroupMemberships()

	return nil
}

func (ies *IAMEtcdStore) savePolicyDoc(policyName string, p iampolicy.Policy) error {
	return ies.saveIAMConfig(&p, getPolicyDocPath(policyName))
}

func (ies *IAMEtcdStore) saveMappedPolicy(name string, isSTS, isGroup bool, mp MappedPolicy) error {
	return ies.saveIAMConfig(mp, getMappedPolicyPath(name, isSTS, isGroup))
}

func (ies *IAMEtcdStore) saveUserIdentity(name string, isSTS bool, u UserIdentity) error {
	return ies.saveIAMConfig(u, getUserIdentityPath(name, isSTS))
}

func (ies *IAMEtcdStore) saveGroupInfo(name string, gi GroupInfo) error {
	return ies.saveIAMConfig(gi, getGroupInfoPath(name))
}

func (ies *IAMEtcdStore) deletePolicyDoc(name string) error {
	err := ies.deleteIAMConfig(getPolicyDocPath(name))
	if err == errConfigNotFound {
		err = errNoSuchPolicy
	}
	return err
}

func (ies *IAMEtcdStore) deleteMappedPolicy(name string, isSTS, isGroup bool) error {
	err := ies.deleteIAMConfig(getMappedPolicyPath(name, isSTS, isGroup))
	if err == errConfigNotFound {
		err = errNoSuchPolicy
	}
	return err
}

func (ies *IAMEtcdStore) deleteUserIdentity(name string, isSTS bool) error {
	err := ies.deleteIAMConfig(getUserIdentityPath(name, isSTS))
	if err == errConfigNotFound {
		err = errNoSuchUser
	}
	return err
}

func (ies *IAMEtcdStore) deleteGroupInfo(name string) error {
	err := ies.deleteIAMConfig(getGroupInfoPath(name))
	if err == errConfigNotFound {
		err = errNoSuchGroup
	}
	return err
}

func (ies *IAMEtcdStore) watch(sys *IAMSys) {
	watchEtcd := func() {
		for {
		outerLoop:
			// Refresh IAMSys with etcd watch.
			watchCh := ies.client.Watch(context.Background(),
				iamConfigPrefix, etcd.WithPrefix(), etcd.WithKeysOnly())

			for {
				select {
				case <-GlobalServiceDoneCh:
					return
				case watchResp, ok := <-watchCh:
					if !ok {
						time.Sleep(1 * time.Second)
						// Upon an error on watch channel
						// re-init the watch channel.
						goto outerLoop
					}
					if err := watchResp.Err(); err != nil {
						logger.LogIf(context.Background(), err)
						// log and retry.
						time.Sleep(1 * time.Second)
						// Upon an error on watch channel
						// re-init the watch channel.
						goto outerLoop
					}
					for _, event := range watchResp.Events {
						sys.Lock()
						ies.reloadFromEvent(sys, event)
						sys.Unlock()
					}
				}
			}
		}
	}
	go watchEtcd()
}

// sys.RLock is held by caller.
func (ies *IAMEtcdStore) reloadFromEvent(sys *IAMSys, event *etcd.Event) {
	eventCreate := event.IsModify() || event.IsCreate()
	eventDelete := event.Type == etcd.EventTypeDelete
	usersPrefix := strings.HasPrefix(string(event.Kv.Key), iamConfigUsersPrefix)
	groupsPrefix := strings.HasPrefix(string(event.Kv.Key), iamConfigGroupsPrefix)
	stsPrefix := strings.HasPrefix(string(event.Kv.Key), iamConfigSTSPrefix)
	policyPrefix := strings.HasPrefix(string(event.Kv.Key), iamConfigPoliciesPrefix)
	policyDBUsersPrefix := strings.HasPrefix(string(event.Kv.Key), iamConfigPolicyDBUsersPrefix)
	policyDBSTSUsersPrefix := strings.HasPrefix(string(event.Kv.Key), iamConfigPolicyDBSTSUsersPrefix)
	policyDBGroupsPrefix := strings.HasPrefix(string(event.Kv.Key), iamConfigPolicyDBGroupsPrefix)

	switch {
	case eventCreate:
		switch {
		case usersPrefix:
			accessKey := path.Dir(strings.TrimPrefix(string(event.Kv.Key),
				iamConfigUsersPrefix))
			ies.loadUser(accessKey, false, sys.iamUsersMap)
		case stsPrefix:
			accessKey := path.Dir(strings.TrimPrefix(string(event.Kv.Key),
				iamConfigSTSPrefix))
			ies.loadUser(accessKey, true, sys.iamUsersMap)
		case groupsPrefix:
			group := path.Dir(strings.TrimPrefix(string(event.Kv.Key),
				iamConfigGroupsPrefix))
			ies.loadGroup(group, sys.iamGroupsMap)
			gi := sys.iamGroupsMap[group]
			sys.removeGroupFromMembershipsMap(group)
			sys.updateGroupMembershipsMap(group, &gi)
		case policyPrefix:
			policyName := path.Dir(strings.TrimPrefix(string(event.Kv.Key),
				iamConfigPoliciesPrefix))
			ies.loadPolicyDoc(policyName, sys.iamPolicyDocsMap)
		case policyDBUsersPrefix:
			policyMapFile := strings.TrimPrefix(string(event.Kv.Key),
				iamConfigPolicyDBUsersPrefix)
			user := strings.TrimSuffix(policyMapFile, ".json")
			ies.loadMappedPolicy(user, false, false, sys.iamUserPolicyMap)
		case policyDBSTSUsersPrefix:
			policyMapFile := strings.TrimPrefix(string(event.Kv.Key),
				iamConfigPolicyDBSTSUsersPrefix)
			user := strings.TrimSuffix(policyMapFile, ".json")
			ies.loadMappedPolicy(user, true, false, sys.iamUserPolicyMap)
		case policyDBGroupsPrefix:
			policyMapFile := strings.TrimPrefix(string(event.Kv.Key),
				iamConfigPolicyDBGroupsPrefix)
			user := strings.TrimSuffix(policyMapFile, ".json")
			ies.loadMappedPolicy(user, false, true, sys.iamGroupPolicyMap)
		}
	case eventDelete:
		switch {
		case usersPrefix:
			accessKey := path.Dir(strings.TrimPrefix(string(event.Kv.Key),
				iamConfigUsersPrefix))
			delete(sys.iamUsersMap, accessKey)
		case stsPrefix:
			accessKey := path.Dir(strings.TrimPrefix(string(event.Kv.Key),
				iamConfigSTSPrefix))
			delete(sys.iamUsersMap, accessKey)
		case groupsPrefix:
			group := path.Dir(strings.TrimPrefix(string(event.Kv.Key),
				iamConfigGroupsPrefix))
			sys.removeGroupFromMembershipsMap(group)
			delete(sys.iamGroupsMap, group)
			delete(sys.iamGroupPolicyMap, group)
		case policyPrefix:
			policyName := path.Dir(strings.TrimPrefix(string(event.Kv.Key),
				iamConfigPoliciesPrefix))
			delete(sys.iamPolicyDocsMap, policyName)
		case policyDBUsersPrefix:
			policyMapFile := strings.TrimPrefix(string(event.Kv.Key),
				iamConfigPolicyDBUsersPrefix)
			user := strings.TrimSuffix(policyMapFile, ".json")
			delete(sys.iamUserPolicyMap, user)
		case policyDBSTSUsersPrefix:
			policyMapFile := strings.TrimPrefix(string(event.Kv.Key),
				iamConfigPolicyDBSTSUsersPrefix)
			user := strings.TrimSuffix(policyMapFile, ".json")
			delete(sys.iamUserPolicyMap, user)
		case policyDBGroupsPrefix:
			policyMapFile := strings.TrimPrefix(string(event.Kv.Key),
				iamConfigPolicyDBGroupsPrefix)
			user := strings.TrimSuffix(policyMapFile, ".json")
			delete(sys.iamGroupPolicyMap, user)
		}
	}
}
