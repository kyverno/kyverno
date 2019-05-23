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
 */

package cmd

// SetupType - enum for setup type.
type SetupType int

const (
	// FSSetupType - FS setup type enum.
	FSSetupType SetupType = iota + 1

	// XLSetupType - XL setup type enum.
	XLSetupType

	// DistXLSetupType - Distributed XL setup type enum.
	DistXLSetupType

	// GatewaySetupType - gateway setup type enum.
	GatewaySetupType
)

func (setupType SetupType) String() string {
	switch setupType {
	case FSSetupType:
		return globalMinioModeFS
	case XLSetupType:
		return globalMinioModeXL
	case DistXLSetupType:
		return globalMinioModeDistXL
	case GatewaySetupType:
		return globalMinioModeGatewayPrefix
	}

	return ""
}
