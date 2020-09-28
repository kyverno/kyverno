// Copyright (c) 2019 Andreas Auernhammer. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

// +build gofuzz

package sio

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	mrand "math/rand"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
)

var BufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(nil)
	},
}

var (
	wStream *Stream
	rStream *Stream
)

func init() {
	// AES-128-GCM
	key := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, key)
	if err != nil {
		panic(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	rStream = NewStream(gcm, BufSize)

	// ChaCha20-Poly1305
	key = make([]byte, 32)
	_, err = io.ReadFull(rand.Reader, key)
	if err != nil {
		panic(err)
	}
	c20p1305, err := chacha20poly1305.New(key)
	if err != nil {
		panic(err)
	}
	wStream = NewStream(c20p1305, BufSize)
}

var maxDataLen int

func FuzzAll(data []byte) int {
	v := FuzzReader(data)
	v += FuzzReadByte(data)
	v += FuzzReaderAt(data)
	v += FuzzWriteTo(data)
	v += FuzzWrite(data)
	v += FuzzWriteByte(data)
	v += FuzzReadFrom(data)

	if len(data) > maxDataLen { // Prefer longer inputs
		maxDataLen = len(data)
		v++
	}
	return v
}

func FuzzReader(data []byte) int {
	nonce := make([]byte, rStream.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err)
	}

	plaintext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(plaintext)
	ciphertext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(ciphertext)

	dLen := int64(len(data))
	buffer := make([]byte, (1+mrand.Intn(64))*1024)

	plaintext.Reset()
	dec := rStream.DecryptReader(rStream.EncryptReader(bytes.NewReader(data), nonce, data), nonce, data)
	if n, err := copyBuffer(plaintext, dec, buffer); int(n) != len(data) || err != nil {
		panic(fmt.Sprintf("N: %d, Err: %v", n, err))
	}
	if !bytes.Equal(plaintext.Bytes(), data) {
		panic("plaintext does not match origin data")
	}

	ciphertext.Reset()
	enc := rStream.EncryptReader(bytes.NewReader(data), nonce, data)
	if _, err := copyBuffer(ciphertext, enc, buffer); err != nil {
		panic(err)
	}

	plaintext.Reset()
	dec = rStream.DecryptReader(bytes.NewReader(ciphertext.Bytes()), nonce, data)
	if _, err := copyBuffer(plaintext, io.LimitReader(dec, dLen/2), buffer); err != nil {
		panic(err)
	}
	if _, err := copyBuffer(plaintext, io.LimitReader(dec, dLen-(dLen/2)), buffer); err != nil {
		panic(err)
	}
	if !bytes.Equal(plaintext.Bytes(), data) {
		panic("plaintext does not match origin data")
	}

	dec = rStream.DecryptReader(bytes.NewReader(data), nonce, data)
	if n, err := copyBuffer(ioutil.Discard, dec, buffer); n != 0 || err != NotAuthentic {
		panic(fmt.Sprintf("N: %d, Err: %v", n, err))
	}
	return 0
}

func FuzzReaderAt(data []byte) int {
	nonce := make([]byte, rStream.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err)
	}

	plaintext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(plaintext)
	ciphertext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(ciphertext)

	buffer := make([]byte, (1+mrand.Intn(64))*1024)

	ciphertext.Reset()
	enc := rStream.EncryptReader(bytes.NewReader(data), nonce, data)
	if _, err := copyBuffer(ciphertext, enc, buffer); err != nil {
		panic(err)
	}

	plaintext.Reset()
	dec := rStream.DecryptReader(bytes.NewReader(ciphertext.Bytes()), nonce, data)
	if _, err := copyBuffer(plaintext, dec, buffer); err != nil {
		panic(err)
	}
	if !bytes.Equal(plaintext.Bytes(), data) {
		panic("plaintext does not match origin data")
	}

	if len(data) > 0 {
		r := io.NewSectionReader(rStream.DecryptReaderAt(bytes.NewReader(data), nonce, data), 0, int64(len(data)))
		if n, err := copyBuffer(ioutil.Discard, r, buffer); n != 0 || err != NotAuthentic {
			panic(fmt.Sprintf("N: %d, Err: %v", n, err))
		}
	}
	return 0
}

func FuzzWriteTo(data []byte) int {
	nonce := make([]byte, rStream.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err)
	}

	plaintext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(plaintext)
	ciphertext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(ciphertext)

	plaintext.Reset()
	dec := rStream.DecryptReader(rStream.EncryptReader(bytes.NewReader(data), nonce, data), nonce, data)
	if n, err := dec.WriteTo(plaintext); int(n) != len(data) || err != nil {
		panic(fmt.Sprintf("N: %d, Err: %v", n, err))
	}
	if !bytes.Equal(plaintext.Bytes(), data) {
		panic("plaintext does not match origin data")
	}

	ciphertext.Reset()
	enc := rStream.EncryptReader(bytes.NewReader(data), nonce, data)
	if _, err := enc.WriteTo(ciphertext); err != nil {
		panic(err)
	}

	plaintext.Reset()
	dec = rStream.DecryptReader(bytes.NewReader(ciphertext.Bytes()), nonce, data)
	if _, err := dec.WriteTo(plaintext); err != nil {
		panic(err)
	}
	if !bytes.Equal(plaintext.Bytes(), data) {
		panic("plaintext does not match origin data")
	}

	dec = rStream.DecryptReader(bytes.NewReader(data), nonce, data)
	if n, err := dec.WriteTo(ioutil.Discard); n != 0 || err != NotAuthentic {
		panic(fmt.Sprintf("N: %d, Err: %v", n, err))
	}
	return 0
}

func FuzzWrite(data []byte) int {
	nonce := make([]byte, wStream.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err)
	}

	plaintext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(plaintext)
	ciphertext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(ciphertext)

	buffer := make([]byte, (1+mrand.Intn(64))*1024)

	plaintext.Reset()
	enc := wStream.EncryptWriter(wStream.DecryptWriter(plaintext, nonce, data), nonce, data)
	if n, err := copyBuffer(enc, bytes.NewReader(data), buffer); n != int64(len(data)) || err != nil {
		panic(fmt.Sprintf("N: %d, Err: %v", n, err))
	}
	if err := enc.Close(); err != nil {
		panic(err)
	}

	ciphertext.Reset()
	enc = wStream.EncryptWriter(ciphertext, nonce, data)
	if _, err := copyBuffer(enc, bytes.NewReader(data), buffer); err != nil {
		panic(err)
	}
	if err := enc.Close(); err != nil {
		panic(err)
	}

	plaintext.Reset()
	dec := wStream.DecryptWriter(plaintext, nonce, data)
	if _, err := copyBuffer(dec, bytes.NewReader(ciphertext.Bytes()), buffer); err != nil {
		panic(err)
	}
	if err := dec.Close(); err != nil {
		panic(err)
	}

	dec = wStream.DecryptWriter(ioutil.Discard, nonce, data)
	if _, err := copyBuffer(dec, bytes.NewReader(data), buffer); err != NotAuthentic {
		if cErr := dec.Close(); err != nil || cErr != NotAuthentic {
			panic(fmt.Sprintf("Write: %v, Close: %v", err, cErr))
		}
	}
	return 0
}

func FuzzReadFrom(data []byte) int {
	nonce := make([]byte, wStream.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err)
	}

	plaintext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(plaintext)
	ciphertext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(ciphertext)

	plaintext.Reset()
	enc := wStream.EncryptWriter(wStream.DecryptWriter(plaintext, nonce, data), nonce, data)
	if n, err := enc.ReadFrom(bytes.NewReader(data)); n != int64(len(data)) || err != nil {
		panic(fmt.Sprintf("N: %d, Err: %v", n, err))
	}
	if err := enc.Close(); err != nil {
		panic(err)
	}

	ciphertext.Reset()
	enc = wStream.EncryptWriter(ciphertext, nonce, data)
	if _, err := enc.ReadFrom(bytes.NewReader(data)); err != nil {
		panic(err)
	}
	if err := enc.Close(); err != nil {
		panic(err)
	}

	plaintext.Reset()
	dec := wStream.DecryptWriter(plaintext, nonce, data)
	if _, err := dec.ReadFrom(bytes.NewReader(ciphertext.Bytes())); err != nil {
		panic(err)
	}
	if err := dec.Close(); err != nil {
		panic(err)
	}

	dec = wStream.DecryptWriter(ioutil.Discard, nonce, data)
	if _, err := dec.ReadFrom(bytes.NewReader(data)); err != NotAuthentic {
		if cErr := dec.Close(); err != nil || cErr != NotAuthentic {
			panic(fmt.Sprintf("Write: %v, Close: %v", err, cErr))
		}
	}
	return 0
}

func FuzzReadByte(data []byte) int {
	nonce := make([]byte, rStream.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err)
	}

	plaintext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(plaintext)
	ciphertext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(ciphertext)

	plaintext.Reset()
	dec := rStream.DecryptReader(rStream.EncryptReader(bytes.NewReader(data), nonce, data), nonce, data)
	if err := copySingleBytes(plaintext, dec); err != nil {
		panic(err)
	}
	if !bytes.Equal(plaintext.Bytes(), data) {
		panic("plaintext does not match origin data")
	}

	dec = rStream.DecryptReader(bytes.NewReader(data), nonce, data)
	if err := copySingleBytes(discard{}, dec); err != NotAuthentic {
		panic(err)
	}
	return 0
}

func FuzzWriteByte(data []byte) int {
	nonce := make([]byte, wStream.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err)
	}

	plaintext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(plaintext)
	ciphertext := BufferPool.Get().(*bytes.Buffer)
	defer BufferPool.Put(ciphertext)

	plaintext.Reset()
	enc := wStream.EncryptWriter(wStream.DecryptWriter(plaintext, nonce, data), nonce, data)
	if err := copySingleBytes(enc, bytes.NewReader(data)); err != nil {
		panic(err)
	}
	if err := enc.Close(); err != nil {
		panic(err)
	}
	if !bytes.Equal(plaintext.Bytes(), data) {
		panic("plaintext does not match origin data")
	}

	dec := rStream.DecryptWriter(ioutil.Discard, nonce, data)
	if err := copySingleBytes(dec, bytes.NewReader(data)); err != NotAuthentic {
		if cErr := dec.Close(); err != nil || cErr != NotAuthentic {
			panic(err)
		}
	}
	return 0
}

func copyBuffer(dst io.Writer, src io.Reader, buf []byte) (int64, error) {
	var written int64
	var err error
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

func copySingleBytes(dst io.ByteWriter, src io.ByteReader) error {
	for {
		b, err := src.ReadByte()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err = dst.WriteByte(b); err != nil {
			return err
		}
	}
}

type discard struct{}

func (discard) WriteByte(p byte) error { return nil }
