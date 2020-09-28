// Copyright (c) 2019 Andreas Auernhammer. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

// Package sio implements a provable secure authenticated encryption
// scheme for continuous byte streams.
package sio

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"io"
	"math"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	// MaxBufSize is the maximum buffer size for streams.
	MaxBufSize = (1 << 24) - 1

	// BufSize is the recommended buffer size for streams.
	BufSize = 1 << 14
)

const (
	// NotAuthentic is returned when the decryption of a data stream fails.
	// It indicates that the encrypted data is invalid - i.e. it has been
	// (maliciously) modified.
	NotAuthentic errorType = "sio: data is not authentic"

	// ErrExceeded is returned when no more data can be encrypted /
	// decrypted securely. It indicates that the data stream is too
	// large to be encrypted / decrypted with a single key-nonce
	// combination.
	//
	// It depends on the buffer size how many bytes can be
	// encrypted / decrypted securely using the same key-nonce
	// combination. For BufSize the limit is ~64 TiB.
	ErrExceeded errorType = "sio: data limit exceeded"
)

type errorType string

func (e errorType) Error() string { return string(e) }

// The constants above specify concrete AEAD algorithms
// that can be used to encrypt and decrypt data streams.
//
// For example, you can create a new Stream using AES-GCM like this:
//   stream, err := sio.AES_128_GCM.Stream(key)
const (
	AES_128_GCM       Algorithm = "AES-128-GCM"        // The secret key must be 16 bytes long. See: https://golang.org/pkg/crypto/cipher/#NewGCM
	AES_256_GCM       Algorithm = "AES-256-GCM"        // The secret key must be 32 bytes long. See: https://golang.org/pkg/crypto/cipher/#NewGCM
	ChaCha20Poly1305  Algorithm = "ChaCha20-Poly1305"  // The secret key must be 32 bytes long. See: https://godoc.org/golang.org/x/crypto/chacha20poly1305#New
	XChaCha20Poly1305 Algorithm = "XChaCha20-Poly1305" // The secret key must be 32 bytes long. See: https://godoc.org/golang.org/x/crypto/chacha20poly1305#NewX
)

// Algorithm specifies an AEAD algorithm that
// can be used to en/decrypt data streams.
//
// Its main purpose is to simplify code that
// wants to use commonly used AEAD algorithms,
// like AES-GCM, by providing a way to directly
// create Streams from secret keys.
type Algorithm string

// String returns the string representation of an
// AEAD algorithm.
func (a Algorithm) String() string { return string(a) }

// Stream returns a new Stream using the given
// secret key and AEAD algorithm.
// The returned Stream uses the default buffer size: BufSize.
func (a Algorithm) Stream(key []byte) (*Stream, error) { return a.streamWithBufSize(key, BufSize) }

func (a Algorithm) streamWithBufSize(key []byte, bufSize int) (*Stream, error) {
	var (
		aead cipher.AEAD
		err  error
	)
	switch a {
	case AES_128_GCM:
		if len(key) != 128/8 {
			return nil, aes.KeySizeError(len(key))
		}
		aead, err = newAESGCM(key)
	case AES_256_GCM:
		if len(key) != 256/8 {
			return nil, aes.KeySizeError(len(key))
		}
		aead, err = newAESGCM(key)
	case ChaCha20Poly1305:
		aead, err = chacha20poly1305.New(key)
	case XChaCha20Poly1305:
		aead, err = chacha20poly1305.NewX(key)
	default:
		return nil, errorType("sio: invalid algorithm name")
	}
	if err != nil {
		return nil, err
	}
	return NewStream(aead, bufSize), nil
}

func newAESGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// NewStream creates a new Stream that encrypts or decrypts data
// streams with the cipher using bufSize large chunks. Therefore,
// the bufSize must be the same for encryption and decryption. If
// you don't have special requirements just use the default BufSize.
//
// The cipher must support a NonceSize() >= 4 and the
// bufSize must be between 1 (inclusive) and MaxBufSize (inclusive).
func NewStream(cipher cipher.AEAD, bufSize int) *Stream {
	if cipher.NonceSize() < 4 {
		panic("sio: NonceSize() of cipher is too small")
	}
	if bufSize > MaxBufSize {
		panic("sio: bufSize is too large")
	}
	if bufSize <= 0 {
		panic("sio: bufSize is too small")
	}
	return &Stream{
		cipher:  cipher,
		bufSize: bufSize,
	}
}

// A Stream encrypts or decrypts continuous byte streams.
type Stream struct {
	cipher  cipher.AEAD
	bufSize int
}

// NonceSize returns the size of the unique nonce that must be
// provided when encrypting or decrypting a data stream.
func (s *Stream) NonceSize() int { return s.cipher.NonceSize() - 4 }

// Overhead returns the overhead added when encrypting a
// data stream. For a plaintext stream of a non-negative
// size, the size of an encrypted data stream will be:
//
//   encSize = size + stream.Overhead(size) // 0 <= size <= (2³² - 1) * bufSize
//         0 = stream.Overhead(size)        // size > (2³² - 1) * bufSize
//        -1 = stream.Overhead(size)        // size < 0
//
// In general, the size of an encrypted data stream is
// always greater than the size of the corresponding
// plaintext stream. If size is too large (i.e.
// greater than (2³² - 1) * bufSize) then Overhead
// returns 0. If size is negative Overhead returns -1.
func (s *Stream) Overhead(size int64) int64 {
	if size < 0 {
		return -1
	}

	bufSize := int64(s.bufSize)
	if size > (bufSize * math.MaxUint32) {
		return 0
	}

	overhead := int64(s.cipher.Overhead())
	if size == 0 {
		return overhead
	}

	t := size / bufSize
	if r := size % bufSize; r > 0 {
		return (t * overhead) + overhead
	}
	return t * overhead
}

// EncryptWriter returns a new EncWriter that wraps w and
// encrypts and authenticates everything before writing
// it to w.
//
// The nonce must be Stream.NonceSize() bytes long and unique
// for the same key. The same nonce must be provided when
// decrypting the data stream.
//
// The associatedData is only authenticated but not encrypted
// and not written to w. Instead, the same associatedData must
// be provided when decrypting the data stream again. It is
// safe to set:
//   associatedData = nil
func (s *Stream) EncryptWriter(w io.Writer, nonce, associatedData []byte) *EncWriter {
	if len(nonce) != s.NonceSize() {
		panic("sio: nonce has invalid length")
	}
	ew := &EncWriter{
		w:              w,
		cipher:         s.cipher,
		bufSize:        s.bufSize,
		nonce:          make([]byte, s.cipher.NonceSize()),
		associatedData: make([]byte, 1+s.cipher.Overhead()),
		buffer:         make([]byte, s.bufSize+s.cipher.Overhead()),
	}
	copy(ew.nonce, nonce)
	nextNonce, _ := ew.nextNonce()
	ew.associatedData[0] = 0x00
	ew.cipher.Seal(ew.associatedData[1:1], nextNonce, nil, associatedData)
	return ew
}

// DecryptWriter returns a new DecWriter that wraps w and
// decrypts and verifies everything before writing
// it to w.
//
// The nonce must be Stream.NonceSize() bytes long and
// must match the value used when encrypting the data stream.
//
// The associatedData must match the value used when encrypting
// the data stream.
func (s *Stream) DecryptWriter(w io.Writer, nonce, associatedData []byte) *DecWriter {
	if len(nonce) != s.NonceSize() {
		panic("sio: nonce has invalid length")
	}
	dw := &DecWriter{
		w:              w,
		cipher:         s.cipher,
		bufSize:        s.bufSize,
		nonce:          make([]byte, s.cipher.NonceSize()),
		associatedData: make([]byte, 1+s.cipher.Overhead()),
		buffer:         make([]byte, s.bufSize+s.cipher.Overhead(), 1+s.bufSize+s.cipher.Overhead()),
	}
	copy(dw.nonce, nonce)
	nextNonce, _ := dw.nextNonce()
	dw.associatedData[0] = 0x00
	dw.cipher.Seal(dw.associatedData[1:1], nextNonce, nil, associatedData)
	return dw
}

// EncryptReader returns a new EncReader that wraps r and
// encrypts and authenticates it reads from r.
//
// The nonce must be Stream.NonceSize() bytes long and unique
// for the same key. The same nonce must be provided when
// decrypting the data stream.
//
// The associatedData is only authenticated but not encrypted
// and not written to w. Instead, the same associatedData must
// be provided when decrypting the data stream again. It is
// safe to set:
//   associatedData = nil
func (s *Stream) EncryptReader(r io.Reader, nonce, associatedData []byte) *EncReader {
	if len(nonce) != s.NonceSize() {
		panic("sio: nonce has invalid length")
	}
	er := &EncReader{
		r:              r,
		cipher:         s.cipher,
		bufSize:        s.bufSize,
		nonce:          make([]byte, s.cipher.NonceSize()),
		associatedData: make([]byte, 1+s.cipher.Overhead()),
		buffer:         make([]byte, 1+s.bufSize+s.cipher.Overhead()),
		firstRead:      true,
	}
	copy(er.nonce, nonce)
	er.associatedData[0] = 0x00
	binary.LittleEndian.PutUint32(er.nonce[er.cipher.NonceSize()-4:], er.seqNum)
	er.cipher.Seal(er.associatedData[1:1], er.nonce, nil, associatedData)
	er.seqNum = 1
	return er
}

// DecryptReader returns a new DecReader that wraps r and
// decrypts and verifies everything it reads from r.
//
// The nonce must be Stream.NonceSize() bytes long and
// must match the value used when encrypting the data stream.
//
// The associatedData must match the value used when encrypting
// the data stream.
func (s *Stream) DecryptReader(r io.Reader, nonce, associatedData []byte) *DecReader {
	if len(nonce) != s.NonceSize() {
		panic("sio: nonce has invalid length")
	}
	dr := &DecReader{
		r:              r,
		cipher:         s.cipher,
		bufSize:        s.bufSize,
		nonce:          make([]byte, s.cipher.NonceSize()),
		associatedData: make([]byte, 1+s.cipher.Overhead()),
		buffer:         make([]byte, 1+s.bufSize+s.cipher.Overhead()),
		firstRead:      true,
	}
	copy(dr.nonce, nonce)
	dr.associatedData[0] = 0x00
	binary.LittleEndian.PutUint32(dr.nonce[dr.cipher.NonceSize()-4:], dr.seqNum)
	dr.cipher.Seal(dr.associatedData[1:1], dr.nonce, nil, associatedData)
	dr.seqNum = 1
	return dr
}

// DecryptReaderAt returns a new DecReaderAt that wraps r and
// decrypts and verifies everything it reads from r.
//
// The nonce must be Stream.NonceSize() bytes long and
// must match the value used when encrypting the data stream.
//
// The associatedData must match the value used when encrypting
// the data stream.
func (s *Stream) DecryptReaderAt(r io.ReaderAt, nonce, associatedData []byte) *DecReaderAt {
	if len(nonce) != s.NonceSize() {
		panic("sio: nonce has invalid length")
	}
	dr := &DecReaderAt{
		r:              r,
		cipher:         s.cipher,
		bufSize:        s.bufSize,
		nonce:          make([]byte, s.cipher.NonceSize()),
		associatedData: make([]byte, 1+s.cipher.Overhead()),
	}
	copy(dr.nonce, nonce)
	dr.associatedData[0] = 0x00
	binary.LittleEndian.PutUint32(dr.nonce[s.NonceSize():], 0)
	dr.cipher.Seal(dr.associatedData[1:1], dr.nonce, nil, associatedData)

	dr.bufPool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 1+dr.bufSize+dr.cipher.Overhead())
			return &b
		},
	}
	return dr
}

// writeTo writes p to w. It returns the first error that occurs during
// writing, if any. If w violates the io.Writer contract and returns less than
// len(p) bytes but no error then writeTo returns io.ErrShortWrite.
func writeTo(w io.Writer, p []byte) (int, error) {
	n, err := w.Write(p)
	if err != nil {
		return n, err
	}
	if n != len(p) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

// readFrom reads len(p) bytes from r into p. It returns the first error that
// occurs during reading, if any. If the returned n < len(p) than the returned
// error is not nil.
func readFrom(r io.Reader, p []byte) (n int, err error) {
	for n < len(p) && err == nil {
		var nn int
		nn, err = r.Read(p[n:])
		n += nn
	}
	if err == io.EOF && n == len(p) {
		err = nil
	}
	return n, err
}
