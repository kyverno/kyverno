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

package ldap

import "github.com/minio/minio/cmd/config"

// Help template for LDAP identity feature.
var (
	Help = config.HelpKVS{
		config.HelpKV{
			Key:         ServerAddr,
<<<<<<< HEAD
			Description: `AD/LDAP server address e.g. "myldapserver.com:636"`,
=======
			Description: `AD/LDAP server address eg: "myldapserver.com:636"`,
>>>>>>> 524_bug
			Type:        "address",
		},
		config.HelpKV{
			Key:         UsernameFormat,
<<<<<<< HEAD
			Description: `AD/LDAP format of full username DN e.g. "uid={username},cn=accounts,dc=myldapserver,dc=com"`,
=======
			Description: `AD/LDAP format of full username DN eg: "uid={username},cn=accounts,dc=myldapserver,dc=com"`,
>>>>>>> 524_bug
			Type:        "string",
		},
		config.HelpKV{
			Key:         GroupSearchFilter,
<<<<<<< HEAD
			Description: `search filter to find groups of a user (optional) e.g. "(&(objectclass=groupOfNames)(member={usernamedn}))"`,
=======
			Description: `Search filter to find groups of a user (optional) eg: "(&(objectclass=groupOfNames)(member={usernamedn}))"`,
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         GroupNameAttribute,
<<<<<<< HEAD
			Description: `attribute of search results to use as group name (optional) e.g. "cn"`,
=======
			Description: `Attribute of search results to use as group name (optional) eg: "cn"`,
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         GroupSearchBaseDN,
<<<<<<< HEAD
			Description: `base DN in AD/LDAP hierarchy to use in search requests (optional) e.g. "dc=myldapserver,dc=com"`,
=======
			Description: `Base DN in AD/LDAP hierarchy to use in search requests (optional) eg: "dc=myldapserver,dc=com"`,
>>>>>>> 524_bug
			Optional:    true,
			Type:        "string",
		},
		config.HelpKV{
			Key:         STSExpiry,
<<<<<<< HEAD
			Description: `AD/LDAP STS credentials validity duration e.g. "1h"`,
=======
			Description: `AD/LDAP STS credentials validity duration eg: "1h"`,
>>>>>>> 524_bug
			Optional:    true,
			Type:        "duration",
		},
		config.HelpKV{
			Key:         TLSSkipVerify,
<<<<<<< HEAD
			Description: "enable this to disable client verification of server certificates",
=======
			Description: "Set this to 'on', to disable client verification of server certificates",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "on|off",
		},
		config.HelpKV{
			Key:         config.Comment,
<<<<<<< HEAD
			Description: config.DefaultComment,
=======
			Description: "A comment to describe the LDAP/AD identity setting",
>>>>>>> 524_bug
			Optional:    true,
			Type:        "sentence",
		},
	}
)
