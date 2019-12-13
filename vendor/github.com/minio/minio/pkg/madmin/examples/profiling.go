// +build ignore

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

package main

import (
	"io"
	"log"
	"os"
	"time"

	"github.com/minio/minio/pkg/madmin"
)

func main() {
	// Note: YOUR-ACCESSKEYID, YOUR-SECRETACCESSKEY are
	// dummy values, please replace them with original values.

	// Note: YOUR-ACCESSKEYID, YOUR-SECRETACCESSKEY are
	// dummy values, please replace them with original values.

	// API requests are secure (HTTPS) if secure=true and insecure (HTTP) otherwise.
	// New returns an MinIO Admin client object.
	madmClnt, err := madmin.New("your-minio.example.com:9000", "YOUR-ACCESSKEYID", "YOUR-SECRETACCESSKEY", true)
	if err != nil {
		log.Fatalln(err)
	}

	profiler := madmin.ProfilerCPU
	log.Println("Starting " + profiler + " profiling..")

	startResults, err := madmClnt.StartProfiling(profiler)
	if err != nil {
		log.Fatalln(err)
	}

	for _, result := range startResults {
		if !result.Success {
			log.Printf("Unable to start profiling on node `%s`, reason = `%s`\n", result.NodeName, result.Error)
			continue
		}
		log.Printf("Profiling successfully started on node `%s`\n", result.NodeName)
	}

	sleep := time.Duration(10)
	time.Sleep(time.Second * sleep)

	log.Println("Stopping profiling..")

	profilingData, err := madmClnt.DownloadProfilingData()
	if err != nil {
		log.Fatalln(err)
	}

	profilingFile, err := os.Create("/tmp/profiling-" + string(profiler) + ".zip")
	if err != nil {
		log.Fatal(err)
	}

	if _, err := io.Copy(profilingFile, profilingData); err != nil {
		log.Fatal(err)
	}

	if err := profilingFile.Close(); err != nil {
		log.Fatal(err)
	}

	if err := profilingData.Close(); err != nil {
		log.Fatal(err)
	}

	log.Println("Profiling files " + profilingFile.Name() + " successfully downloaded.")
}
