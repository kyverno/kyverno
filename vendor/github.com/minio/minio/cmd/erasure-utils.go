/*
 * MinIO Cloud Storage, (C) 2016 MinIO, Inc.
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
	"io"

	"github.com/klauspost/reedsolomon"
	"github.com/minio/minio/cmd/logger"
)

// getDataBlockLen - get length of data blocks from encoded blocks.
func getDataBlockLen(enBlocks [][]byte, dataBlocks int) int {
	size := 0
	// Figure out the data block length.
	for _, block := range enBlocks[:dataBlocks] {
		size += len(block)
	}
	return size
}

// Writes all the data blocks from encoded blocks until requested
// outSize length. Provides a way to skip bytes until the offset.
func writeDataBlocks(ctx context.Context, dst io.Writer, enBlocks [][]byte, dataBlocks int, offset int64, length int64) (int64, error) {
	// Offset and out size cannot be negative.
	if offset < 0 || length < 0 {
		logger.LogIf(ctx, errUnexpected)
		return 0, errUnexpected
	}

	// Do we have enough blocks?
	if len(enBlocks) < dataBlocks {
		logger.LogIf(ctx, reedsolomon.ErrTooFewShards)
		return 0, reedsolomon.ErrTooFewShards
	}

	// Do we have enough data?
	if int64(getDataBlockLen(enBlocks, dataBlocks)) < length {
		logger.LogIf(ctx, reedsolomon.ErrShortData)
		return 0, reedsolomon.ErrShortData
	}

	// Counter to decrement total left to write.
	write := length

	// Counter to increment total written.
	var totalWritten int64

	// Write all data blocks to dst.
	for _, block := range enBlocks[:dataBlocks] {
		// Skip blocks until we have reached our offset.
		if offset >= int64(len(block)) {
			// Decrement offset.
			offset -= int64(len(block))
			continue
		} else {
			// Skip until offset.
			block = block[offset:]

			// Reset the offset for next iteration to read everything
			// from subsequent blocks.
			offset = 0
		}
		// We have written all the blocks, write the last remaining block.
		if write < int64(len(block)) {
			n, err := io.Copy(dst, bytes.NewReader(block[:write]))
			if err != nil {
				if err != io.ErrClosedPipe {
					logger.LogIf(ctx, err)
				}
				return 0, err
			}
			totalWritten += n
			break
		}
		// Copy the block.
		n, err := io.Copy(dst, bytes.NewReader(block))
		if err != nil {
			// The writer will be closed incase of range queries, which will emit ErrClosedPipe.
			if err != io.ErrClosedPipe {
				logger.LogIf(ctx, err)
			}
			return 0, err
		}

		// Decrement output size.
		write -= n

		// Increment written.
		totalWritten += n
	}

	// Success.
	return totalWritten, nil
}
