/*
 * MinIO Cloud Storage, (C) 2018 MinIO, Inc.
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
	"fmt"
	"strconv"
	"strings"

	"github.com/minio/minio-go/v6/pkg/set"
	"github.com/minio/minio/cmd/config"
	"github.com/minio/minio/pkg/ellipses"
	"github.com/minio/minio/pkg/env"
)

// This file implements and supports ellipses pattern for
// `minio server` command line arguments.

// Endpoint set represents parsed ellipses values, also provides
// methods to get the sets of endpoints.
type endpointSet struct {
	argPatterns []ellipses.ArgPattern
	endpoints   []string   // Endpoints saved from previous GetEndpoints().
	setIndexes  [][]uint64 // All the sets.
}

// Supported set sizes this is used to find the optimal
// single set size.
var setSizes = []uint64{4, 6, 8, 10, 12, 14, 16}

// getDivisibleSize - returns a greatest common divisor of
// all the ellipses sizes.
func getDivisibleSize(totalSizes []uint64) (result uint64) {
	gcd := func(x, y uint64) uint64 {
		for y != 0 {
			x, y = y, x%y
		}
		return x
	}
	result = totalSizes[0]
	for i := 1; i < len(totalSizes); i++ {
		result = gcd(result, totalSizes[i])
	}
	return result
}

// isValidSetSize - checks whether given count is a valid set size for erasure coding.
var isValidSetSize = func(count uint64) bool {
	return (count >= setSizes[0] && count <= setSizes[len(setSizes)-1] && count%2 == 0)
}

// getSetIndexes returns list of indexes which provides the set size
// on each index, this function also determines the final set size
// The final set size has the affinity towards choosing smaller
// indexes (total sets)
func getSetIndexes(args []string, totalSizes []uint64, customSetDriveCount uint64) (setIndexes [][]uint64, err error) {
	if len(totalSizes) == 0 || len(args) == 0 {
		return nil, errInvalidArgument
	}

	setIndexes = make([][]uint64, len(totalSizes))
	for _, totalSize := range totalSizes {
		// Check if totalSize has minimum range upto setSize
		if totalSize < setSizes[0] || totalSize < customSetDriveCount {
			return nil, config.ErrInvalidNumberOfErasureEndpoints(nil)
		}
	}

	var setSize uint64

	commonSize := getDivisibleSize(totalSizes)
	if commonSize > setSizes[len(setSizes)-1] {
		prevD := commonSize / setSizes[0]
		for _, i := range setSizes {
			if commonSize%i == 0 {
				d := commonSize / i
				if d <= prevD {
					prevD = d
					setSize = i
				}
			}
		}
	} else {
		setSize = commonSize
	}

	possibleSetCounts := func(setSize uint64) (ss []uint64) {
		for _, s := range setSizes {
			if setSize%s == 0 {
				ss = append(ss, s)
			}
		}
		return ss
	}

	if customSetDriveCount > 0 {
		msg := fmt.Sprintf("Invalid set drive count, leads to non-uniform distribution for the given number of disks. Possible values for custom set count are %d", possibleSetCounts(setSize))
		if customSetDriveCount > setSize {
			return nil, config.ErrInvalidErasureSetSize(nil).Msg(msg)
		}
		if setSize%customSetDriveCount != 0 {
			return nil, config.ErrInvalidErasureSetSize(nil).Msg(msg)
		}
		setSize = customSetDriveCount
	}

	// Check whether setSize is with the supported range.
	if !isValidSetSize(setSize) {
		return nil, config.ErrInvalidNumberOfErasureEndpoints(nil)
	}

	for i := range totalSizes {
		for j := uint64(0); j < totalSizes[i]/setSize; j++ {
			setIndexes[i] = append(setIndexes[i], setSize)
		}
	}

	return setIndexes, nil
}

// Returns all the expanded endpoints, each argument is expanded separately.
func (s endpointSet) getEndpoints() (endpoints []string) {
	if len(s.endpoints) != 0 {
		return s.endpoints
	}
	for _, argPattern := range s.argPatterns {
		for _, lbls := range argPattern.Expand() {
			endpoints = append(endpoints, strings.Join(lbls, ""))
		}
	}
	s.endpoints = endpoints
	return endpoints
}

// Get returns the sets representation of the endpoints
// this function also intelligently decides on what will
// be the right set size etc.
func (s endpointSet) Get() (sets [][]string) {
	var k = uint64(0)
	endpoints := s.getEndpoints()
	for i := range s.setIndexes {
		for j := range s.setIndexes[i] {
			sets = append(sets, endpoints[k:s.setIndexes[i][j]+k])
			k = s.setIndexes[i][j] + k
		}
	}

	return sets
}

// Return the total size for each argument patterns.
func getTotalSizes(argPatterns []ellipses.ArgPattern) []uint64 {
	var totalSizes []uint64
	for _, argPattern := range argPatterns {
		var totalSize uint64 = 1
		for _, p := range argPattern {
			totalSize = totalSize * uint64(len(p.Seq))
		}
		totalSizes = append(totalSizes, totalSize)
	}
	return totalSizes
}

// Parses all arguments and returns an endpointSet which is a collection
// of endpoints following the ellipses pattern, this is what is used
// by the object layer for initializing itself.
func parseEndpointSet(customSetDriveCount uint64, args ...string) (ep endpointSet, err error) {
	var argPatterns = make([]ellipses.ArgPattern, len(args))
	for i, arg := range args {
		patterns, perr := ellipses.FindEllipsesPatterns(arg)
		if perr != nil {
			return endpointSet{}, config.ErrInvalidErasureEndpoints(nil).Msg(perr.Error())
		}
		argPatterns[i] = patterns
	}

	ep.setIndexes, err = getSetIndexes(args, getTotalSizes(argPatterns), customSetDriveCount)
	if err != nil {
		return endpointSet{}, config.ErrInvalidErasureEndpoints(nil).Msg(err.Error())
	}

	ep.argPatterns = argPatterns

	return ep, nil
}

// GetAllSets - parses all ellipses input arguments, expands them into
// corresponding list of endpoints chunked evenly in accordance with a
// specific set size.
// For example: {1...64} is divided into 4 sets each of size 16.
// This applies to even distributed setup syntax as well.
func GetAllSets(args ...string) ([][]string, error) {
	var customSetDriveCount uint64
	if v := env.Get("MINIO_ERASURE_SET_DRIVE_COUNT", ""); v != "" {
		customSetDriveCount, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, config.ErrInvalidErasureSetSize(err)
		}
		if !isValidSetSize(customSetDriveCount) {
			return nil, config.ErrInvalidErasureSetSize(nil)
		}
	}

	var setArgs [][]string
	if !ellipses.HasEllipses(args...) {
		var setIndexes [][]uint64
		// Check if we have more one args.
		if len(args) > 1 {
			var err error
			setIndexes, err = getSetIndexes(args, []uint64{uint64(len(args))}, customSetDriveCount)
			if err != nil {
				return nil, err
			}
		} else {
			// We are in FS setup, proceed forward.
			setIndexes = [][]uint64{{uint64(len(args))}}
		}
		s := endpointSet{
			endpoints:  args,
			setIndexes: setIndexes,
		}
		setArgs = s.Get()
	} else {
		s, err := parseEndpointSet(customSetDriveCount, args...)
		if err != nil {
			return nil, err
		}
		setArgs = s.Get()
	}

	uniqueArgs := set.NewStringSet()
	for _, sargs := range setArgs {
		for _, arg := range sargs {
			if uniqueArgs.Contains(arg) {
				return nil, config.ErrInvalidErasureEndpoints(nil).Msg(fmt.Sprintf("Input args (%s) has duplicate ellipses", args))
			}
			uniqueArgs.Add(arg)
		}
	}

	return setArgs, nil
}

// CreateServerEndpoints - validates and creates new endpoints from input args, supports
// both ellipses and without ellipses transparently.
func createServerEndpoints(serverAddr string, args ...string) (EndpointZones, int, SetupType, error) {
	if len(args) == 0 {
		return nil, -1, -1, errInvalidArgument
	}

	var endpointZones EndpointZones
	var setupType SetupType
	var drivesPerSet int
	if !ellipses.HasEllipses(args...) {
		setArgs, err := GetAllSets(args...)
		if err != nil {
			return nil, -1, -1, err
		}
		endpointList, newSetupType, err := CreateEndpoints(serverAddr, setArgs...)
		if err != nil {
			return nil, -1, -1, err
		}
		endpointZones = append(endpointZones, ZoneEndpoints{
			SetCount:     len(setArgs),
			DrivesPerSet: len(setArgs[0]),
			Endpoints:    endpointList,
		})
		setupType = newSetupType
		return endpointZones, len(setArgs[0]), setupType, nil
	}

	// Verify the args setup-type appropriately.
	{
		setArgs, err := GetAllSets(args...)
		if err != nil {
			return nil, -1, -1, err
		}

		_, setupType, err = CreateEndpoints(serverAddr, setArgs...)
		if err != nil {
			return nil, -1, -1, err
		}
	}

	for _, arg := range args {
		setArgs, err := GetAllSets(arg)
		if err != nil {
			return nil, -1, -1, err
		}
		endpointList, _, err := CreateEndpoints(serverAddr, setArgs...)
		if err != nil {
			return nil, -1, -1, err
		}
		if drivesPerSet != 0 && drivesPerSet != len(setArgs[0]) {
			return nil, -1, -1, fmt.Errorf("All zones should have same drive per set ratio - expected %d, got %d", drivesPerSet, len(setArgs[0]))
		}
		endpointZones = append(endpointZones, ZoneEndpoints{
			SetCount:     len(setArgs),
			DrivesPerSet: len(setArgs[0]),
			Endpoints:    endpointList,
		})
		drivesPerSet = len(setArgs[0])
	}

	return endpointZones, drivesPerSet, setupType, nil
}
