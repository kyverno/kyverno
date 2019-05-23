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

import (
	"context"

	"github.com/klauspost/reedsolomon"
	"github.com/minio/minio/cmd/logger"
)

// Erasure - erasure encoding details.
type Erasure struct {
	encoder                  reedsolomon.Encoder
	dataBlocks, parityBlocks int
	blockSize                int64
}

// NewErasure creates a new ErasureStorage.
func NewErasure(ctx context.Context, dataBlocks, parityBlocks int, blockSize int64) (e Erasure, err error) {
	e = Erasure{
		dataBlocks:   dataBlocks,
		parityBlocks: parityBlocks,
		blockSize:    blockSize,
	}
	e.encoder, err = reedsolomon.New(dataBlocks, parityBlocks, reedsolomon.WithAutoGoroutines(int(e.ShardSize())))
	if err != nil {
		logger.LogIf(ctx, err)
		return e, err
	}
	return
}

// EncodeData encodes the given data and returns the erasure-coded data.
// It returns an error if the erasure coding failed.
func (e *Erasure) EncodeData(ctx context.Context, data []byte) ([][]byte, error) {
	if len(data) == 0 {
		return make([][]byte, e.dataBlocks+e.parityBlocks), nil
	}
	encoded, err := e.encoder.Split(data)
	if err != nil {
		logger.LogIf(ctx, err)
		return nil, err
	}
	if err = e.encoder.Encode(encoded); err != nil {
		logger.LogIf(ctx, err)
		return nil, err
	}
	return encoded, nil
}

// DecodeDataBlocks decodes the given erasure-coded data.
// It only decodes the data blocks but does not verify them.
// It returns an error if the decoding failed.
func (e *Erasure) DecodeDataBlocks(data [][]byte) error {
	needsReconstruction := false
	for _, b := range data[:e.dataBlocks] {
		if b == nil {
			needsReconstruction = true
			break
		}
	}
	if !needsReconstruction {
		return nil
	}
	if err := e.encoder.ReconstructData(data); err != nil {
		return err
	}
	return nil
}

// DecodeDataAndParityBlocks decodes the given erasure-coded data and verifies it.
// It returns an error if the decoding failed.
func (e *Erasure) DecodeDataAndParityBlocks(ctx context.Context, data [][]byte) error {
	if err := e.encoder.Reconstruct(data); err != nil {
		logger.LogIf(ctx, err)
		return err
	}
	return nil
}

// ShardSize - returns actual shared size from erasure blockSize.
func (e *Erasure) ShardSize() int64 {
	return ceilFrac(e.blockSize, int64(e.dataBlocks))
}

// ShardFileSize - returns final erasure size from original size.
func (e *Erasure) ShardFileSize(totalLength int64) int64 {
	if totalLength == 0 {
		return 0
	}
	if totalLength == -1 {
		return -1
	}
	numShards := totalLength / e.blockSize
	lastBlockSize := totalLength % int64(e.blockSize)
	lastShardSize := ceilFrac(lastBlockSize, int64(e.dataBlocks))
	return numShards*e.ShardSize() + lastShardSize
}

// ShardFileTillOffset - returns the effectiv eoffset where erasure reading begins.
func (e *Erasure) ShardFileTillOffset(startOffset, length, totalLength int64) int64 {
	shardSize := e.ShardSize()
	shardFileSize := e.ShardFileSize(totalLength)
	endShard := (startOffset + int64(length)) / e.blockSize
	tillOffset := endShard*shardSize + shardSize
	if tillOffset > shardFileSize {
		tillOffset = shardFileSize
	}
	return tillOffset
}
