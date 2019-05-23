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
	"context"
	"errors"
	"hash"
	"io"

	"github.com/minio/highwayhash"
	"github.com/minio/minio/cmd/logger"
	sha256 "github.com/minio/sha256-simd"
	"golang.org/x/crypto/blake2b"
)

// magic HH-256 key as HH-256 hash of the first 100 decimals of π as utf-8 string with a zero key.
var magicHighwayHash256Key = []byte("\x4b\xe7\x34\xfa\x8e\x23\x8a\xcd\x26\x3e\x83\xe6\xbb\x96\x85\x52\x04\x0f\x93\x5d\xa3\x9f\x44\x14\x97\xe0\x9d\x13\x22\xde\x36\xa0")

// BitrotAlgorithm specifies a algorithm used for bitrot protection.
type BitrotAlgorithm uint

const (
	// SHA256 represents the SHA-256 hash function
	SHA256 BitrotAlgorithm = 1 + iota
	// HighwayHash256 represents the HighwayHash-256 hash function
	HighwayHash256
	// HighwayHash256S represents the Streaming HighwayHash-256 hash function
	HighwayHash256S
	// BLAKE2b512 represents the BLAKE2b-512 hash function
	BLAKE2b512
)

// DefaultBitrotAlgorithm is the default algorithm used for bitrot protection.
const (
	DefaultBitrotAlgorithm = HighwayHash256S
)

var bitrotAlgorithms = map[BitrotAlgorithm]string{
	SHA256:          "sha256",
	BLAKE2b512:      "blake2b",
	HighwayHash256:  "highwayhash256",
	HighwayHash256S: "highwayhash256S",
}

// New returns a new hash.Hash calculating the given bitrot algorithm.
func (a BitrotAlgorithm) New() hash.Hash {
	switch a {
	case SHA256:
		return sha256.New()
	case BLAKE2b512:
		b2, _ := blake2b.New512(nil) // New512 never returns an error if the key is nil
		return b2
	case HighwayHash256:
		hh, _ := highwayhash.New(magicHighwayHash256Key) // New will never return error since key is 256 bit
		return hh
	case HighwayHash256S:
		hh, _ := highwayhash.New(magicHighwayHash256Key) // New will never return error since key is 256 bit
		return hh
	default:
		logger.CriticalIf(context.Background(), errors.New("Unsupported bitrot algorithm"))
		return nil
	}
}

// Available reports whether the given algorithm is available.
func (a BitrotAlgorithm) Available() bool {
	_, ok := bitrotAlgorithms[a]
	return ok
}

// String returns the string identifier for a given bitrot algorithm.
// If the algorithm is not supported String panics.
func (a BitrotAlgorithm) String() string {
	name, ok := bitrotAlgorithms[a]
	if !ok {
		logger.CriticalIf(context.Background(), errors.New("Unsupported bitrot algorithm"))
	}
	return name
}

// NewBitrotVerifier returns a new BitrotVerifier implementing the given algorithm.
func NewBitrotVerifier(algorithm BitrotAlgorithm, checksum []byte) *BitrotVerifier {
	return &BitrotVerifier{algorithm, checksum}
}

// BitrotVerifier can be used to verify protected data.
type BitrotVerifier struct {
	algorithm BitrotAlgorithm
	sum       []byte
}

// BitrotAlgorithmFromString returns a bitrot algorithm from the given string representation.
// It returns 0 if the string representation does not match any supported algorithm.
// The zero value of a bitrot algorithm is never supported.
func BitrotAlgorithmFromString(s string) (a BitrotAlgorithm) {
	for alg, name := range bitrotAlgorithms {
		if name == s {
			return alg
		}
	}
	return
}

func newBitrotWriter(disk StorageAPI, volume, filePath string, length int64, algo BitrotAlgorithm, shardSize int64) io.Writer {
	if algo == HighwayHash256S {
		return newStreamingBitrotWriter(disk, volume, filePath, length, algo, shardSize)
	}
	return newWholeBitrotWriter(disk, volume, filePath, algo, shardSize)
}

func newBitrotReader(disk StorageAPI, bucket string, filePath string, tillOffset int64, algo BitrotAlgorithm, sum []byte, shardSize int64) io.ReaderAt {
	if algo == HighwayHash256S {
		return newStreamingBitrotReader(disk, bucket, filePath, tillOffset, algo, shardSize)
	}
	return newWholeBitrotReader(disk, bucket, filePath, algo, tillOffset, sum)
}

// Close all the readers.
func closeBitrotReaders(rs []io.ReaderAt) {
	for _, r := range rs {
		if br, ok := r.(io.Closer); ok {
			br.Close()
		}
	}
}

// Close all the writers.
func closeBitrotWriters(ws []io.Writer) {
	for _, w := range ws {
		if bw, ok := w.(io.Closer); ok {
			bw.Close()
		}
	}
}

// Returns hash sum for whole-bitrot, nil for streaming-bitrot.
func bitrotWriterSum(w io.Writer) []byte {
	if bw, ok := w.(*wholeBitrotWriter); ok {
		return bw.Sum(nil)
	}
	return nil
}

// Verify if a file has bitrot error.
func bitrotCheckFile(disk StorageAPI, volume string, filePath string, tillOffset int64, algo BitrotAlgorithm, sum []byte, shardSize int64) (err error) {
	if algo != HighwayHash256S {
		buf := []byte{}
		// For whole-file bitrot we don't need to read the entire file as the bitrot verify happens on the server side even if we read 0-bytes.
		_, err = disk.ReadFile(volume, filePath, 0, buf, NewBitrotVerifier(algo, sum))
		return err
	}
	buf := make([]byte, shardSize)
	r := newStreamingBitrotReader(disk, volume, filePath, tillOffset, algo, shardSize)
	defer closeBitrotReaders([]io.ReaderAt{r})
	var offset int64
	for {
		if offset == tillOffset {
			break
		}
		var n int
		tmpBuf := buf
		if int64(len(tmpBuf)) > (tillOffset - offset) {
			tmpBuf = tmpBuf[:(tillOffset - offset)]
		}
		n, err = r.ReadAt(tmpBuf, offset)
		if err != nil {
			return err
		}
		offset += int64(n)
	}
	return nil
}
