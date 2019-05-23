// +build ignore

package main

/*
 * MinIO Cloud Storage, (C) 2017 MinIO, Inc.
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
 *
 */

import (
	"fmt"
	"log"

	"github.com/minio/minio/pkg/madmin"
)

func main() {

	// Note: YOUR-ACCESSKEYID, YOUR-SECRETACCESSKEY are
	// dummy values, please replace them with original values.

	// API requests are secure (HTTPS) if secure=true and insecure (HTTPS) otherwise.
	// New returns an MinIO Admin client object.
	madmClnt, err := madmin.New("your-minio.example.com:9000", "YOUR-ACCESSKEYID", "YOUR-SECRETACCESSKEY", true)
	if err != nil {
		log.Fatalln(err)
	}

	// List buckets that need healing
	healBucketsList, err := madmClnt.ListBucketsHeal()
	if err != nil {
		log.Fatalln(err)
	}

	for _, bucket := range healBucketsList {
		if bucket.HealBucketInfo != nil {
			switch healInfo := *bucket.HealBucketInfo; healInfo.Status {
			case madmin.CanHeal:
				fmt.Println(bucket.Name, " can be healed.")
			case madmin.QuorumUnavailable:
				fmt.Println(bucket.Name, " can't be healed until quorum is available.")
			case madmin.Corrupted:
				fmt.Println(bucket.Name, " can't be healed, not enough information.")
			}
		}
		fmt.Println("bucket: ", bucket)
	}
}
