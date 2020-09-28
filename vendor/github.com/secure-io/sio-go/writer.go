// Copyright (c) 2019 Andreas Auernhammer. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

package sio

import (
	"crypto/cipher"
	"encoding/binary"
	"io"
	"math"
)

// An EncWriter encrypts and authenticates everything it
// writes to an underlying io.Writer.
//
// An EncWriter must always be closed successfully.
// Otherwise, the encrypted data written to the underlying
// io.Writer would be incomplete. Also, no more data must
// be written to a closed EncWriter.
//
// Closing an EncWriter also closes the underlying io.Writer
// if it implements io.Closer.
type EncWriter struct {
	w       io.Writer
	cipher  cipher.AEAD
	bufSize int

	seqNum         uint32
	nonce          []byte
	associatedData []byte

	buffer []byte
	offset int

	err    error
	closed bool
}

// Write behaves as specified by the io.Writer interface.
// In particular, Write encrypts len(p) bytes from p and
// writes the encrypted bytes to the underlying data stream. It
// returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
//
// When Write cannot encrypt any more bytes securely it
// returns ErrExceeded. However, the EncWriter  must still
// be closed to complete the encryption and flush any
// remaining data to the underlying data stream.
//
// Write must not be called once the EncWriter has
// been closed.
func (w *EncWriter) Write(p []byte) (n int, err error) {
	if w.closed {
		panic("sio: EncWriter is closed")
	}
	if w.err != nil {
		return 0, w.err
	}
	if w.offset > 0 {
		n = copy(w.buffer[w.offset:w.bufSize], p)
		if n == len(p) {
			w.offset += n
			return n, nil
		}
		p = p[n:]
		w.offset = 0

		nonce, err := w.nextNonce()
		if err != nil {
			w.err = err
			return n, w.err
		}
		ciphertext := w.cipher.Seal(w.buffer[:0], nonce, w.buffer[:w.bufSize], w.associatedData)
		if _, err = writeTo(w.w, ciphertext); err != nil {
			w.err = err
			return n, w.err
		}
	}
	for len(p) > w.bufSize {
		nonce, err := w.nextNonce()
		if err != nil {
			w.err = err
			return n, w.err
		}
		ciphertext := w.cipher.Seal(w.buffer[:0], nonce, p[:w.bufSize], w.associatedData)
		if _, err = writeTo(w.w, ciphertext); err != nil {
			w.err = err
			return n, w.err
		}
		p = p[w.bufSize:]
		n += w.bufSize
	}
	w.offset = copy(w.buffer, p)
	n += w.offset
	return n, nil
}

// WriteByte behaves as specified by the io.ByteWriter interface.
// In particular, WriteByte encrypts b and writes the encrypted
// byte to the underlying data stream.
//
// When WriteByte cannot encrypt one more byte securely it
// returns ErrExceeded. However, the EncWriter  must still
// be closed to complete the encryption and flush any
// remaining data to the underlying data stream.
//
// WriteByte must not be called once the EncWriter has
// been closed.
func (w *EncWriter) WriteByte(b byte) error {
	if w.closed {
		panic("sio: EncWriter is closed")
	}
	if w.err != nil {
		return w.err
	}

	if w.offset < w.bufSize {
		w.buffer[w.offset] = b
		w.offset++
		return nil
	}

	nonce, err := w.nextNonce()
	if err != nil {
		w.err = err
		return w.err
	}
	ciphertext := w.cipher.Seal(w.buffer[:0], nonce, w.buffer[:w.bufSize], w.associatedData)
	if _, err = writeTo(w.w, ciphertext); err != nil {
		w.err = err
		return w.err
	}

	w.buffer[0] = b
	w.offset = 1
	return nil
}

// Close writes any remaining encrypted bytes to the
// underlying io.Writer and returns any error
// encountered. If the underlying io.Writer implements
// Close it closes the underlying data stream as well.
//
// It safe to call close multiple times.
func (w *EncWriter) Close() error {
	if w.err != nil && w.err != ErrExceeded {
		return w.err
	}
	if !w.closed {
		w.closed = true

		w.associatedData[0] = 0x80
		binary.LittleEndian.PutUint32(w.nonce[w.cipher.NonceSize()-4:], w.seqNum)
		ciphertext := w.cipher.Seal(w.buffer[:0], w.nonce, w.buffer[:w.offset], w.associatedData)
		if _, w.err = writeTo(w.w, ciphertext); w.err != nil {
			return w.err
		}
		if c, ok := w.w.(io.Closer); ok {
			w.err = c.Close()
			return w.err
		}
	}
	return nil
}

// ReadFrom behaves as specified by the io.ReadFrom interface.
// In particular, ReadFrom reads data from r until io.EOF or any
// error occurs, encrypts the data and writes the encrypted data
// to the underlying io.Writer. ReadFrom does not close the
// EncWriter nor the underlying data stream.
//
// ReadFrom returns the number of bytes read and any error except
// io.EOF.
//
// When ReadFrom cannot encrypt any more data securely it
// returns ErrExceeded. However, the EncWriter  must still
// be closed to complete the encryption and flush any
// remaining data to the underlying data stream.
//
// ReadFrom must not be called once the EncWriter has
// been closed.
func (w *EncWriter) ReadFrom(r io.Reader) (int64, error) {
	if w.closed {
		panic("sio: EncWriter is closed")
	}
	if w.err != nil {
		return 0, w.err
	}

	nn, err := readFrom(r, w.buffer[:w.bufSize+1])
	if err == io.EOF {
		w.offset = nn
		return int64(nn), nil
	}
	if err != nil {
		w.err = err
		return int64(nn), err
	}
	carry := w.buffer[w.bufSize]

	nonce, err := w.nextNonce()
	if err != nil {
		w.err = err
		return int64(nn), w.err
	}
	ciphertext := w.cipher.Seal(w.buffer[:0], nonce, w.buffer[:w.bufSize], w.associatedData)
	if _, err = writeTo(w.w, ciphertext); err != nil {
		w.err = err
		return int64(nn), w.err
	}

	n := int64(nn)
	for {
		w.buffer[0] = carry
		nn, err = readFrom(r, w.buffer[1:1+w.bufSize])
		n += int64(nn)
		if err == io.EOF {
			w.offset = 1 + nn
			return n, nil
		}
		if err != nil {
			w.err = err
			return n, w.err
		}
		carry = w.buffer[w.bufSize]

		nonce, err = w.nextNonce()
		if err != nil {
			w.err = err
			return n, w.err
		}
		ciphertext = w.cipher.Seal(w.buffer[:0], nonce, w.buffer[:w.bufSize], w.associatedData)
		if _, err = writeTo(w.w, ciphertext); err != nil {
			w.err = err
			return n, w.err
		}
	}
}

func (w *EncWriter) nextNonce() ([]byte, error) {
	if w.seqNum == math.MaxUint32 {
		return nil, ErrExceeded
	}
	binary.LittleEndian.PutUint32(w.nonce[w.cipher.NonceSize()-4:], w.seqNum)
	w.seqNum++
	return w.nonce, nil
}

// A DecWriter decrypts and verifies everything it
// writes to an underlying io.Writer. It never writes
// invalid (i.e. not authentic) data to the underlying
// data stream.
//
// A DecWriter must always be closed and the returned
// error must be checked. Otherwise, the decrypted data
// written to the underlying io.Writer would be incomplete.
// Also, no more data must be written to a closed DecWriter.
//
// Closing a DecWriter also closes the underlying io.Writer
// if it implements io.Closer.
type DecWriter struct {
	w       io.Writer
	cipher  cipher.AEAD
	bufSize int

	seqNum         uint32
	nonce          []byte
	associatedData []byte

	buffer []byte
	offset int

	err    error
	closed bool
}

// Write behaves as specified by the io.Writer interface.
// In particular, Write decrypts len(p) bytes from p and
// writes the decrypted bytes to the underlying data stream. It
// returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
//
// When Write fails to decrypt some data it returns NotAuthentic.
// This error indicates that the encrypted bytes have been
// (maliciously) modified.
//
// When Write cannot decrypt any more bytes securely it
// returns ErrExceeded. However, the DecWriter  must still
// be closed to complete the decryption and flush any
// remaining data to the underlying data stream.
//
// Write must not be called once the DecWriter has
// been closed.
func (w *DecWriter) Write(p []byte) (n int, err error) {
	if w.closed {
		panic("sio: DecWriter is closed")
	}
	if w.err != nil {
		return 0, w.err
	}
	if w.offset > 0 {
		n = copy(w.buffer[w.offset:], p)
		if n == len(p) {
			w.offset += n
			return n, nil
		}
		p = p[n:]
		w.offset = 0

		nonce, err := w.nextNonce()
		if err != nil {
			w.err = err
			return n, w.err
		}
		plaintext, err := w.cipher.Open(w.buffer[:0], nonce, w.buffer, w.associatedData)
		if err != nil {
			w.err = NotAuthentic
			return n, w.err
		}
		if _, err = writeTo(w.w, plaintext); err != nil {
			w.err = err
			return n, w.err
		}
	}
	ciphertextLen := w.bufSize + w.cipher.Overhead()
	for len(p) > ciphertextLen {
		nonce, err := w.nextNonce()
		if err != nil {
			w.err = err
			return n, w.err
		}
		plaintext, err := w.cipher.Open(w.buffer[:0], nonce, p[:ciphertextLen], w.associatedData)
		if err != nil {
			w.err = NotAuthentic
			return n, w.err
		}
		if _, err = writeTo(w.w, plaintext); err != nil {
			w.err = err
			return n, w.err
		}
		p = p[ciphertextLen:]
		n += ciphertextLen

	}
	w.offset = copy(w.buffer, p)
	n += w.offset
	return n, nil
}

// WriteByte behaves as specified by the io.ByteWriter interface.
// In particular, WriteByte decrypts b and writes the decrypted
// byte to the underlying data stream.
//
// When WriteByte fails to decrypt b it returns NotAuthentic.
// This error indicates that some encrypted bytes have been
// (maliciously) modified.
//
// When WriteByte cannot decrypt one more byte securely it
// returns ErrExceeded. However, the DecWriter  must still
// be closed to complete the decryption and flush any
// remaining data to the underlying data stream.
//
// WriteByte must not be called once the DecWriter has
// been closed.
func (w *DecWriter) WriteByte(b byte) error {
	if w.closed {
		panic("sio: DecWriter is closed")
	}
	if w.err != nil {
		return w.err
	}
	if w.offset < w.bufSize+w.cipher.Overhead() {
		w.buffer[w.offset] = b
		w.offset++
		return nil
	}

	nonce, err := w.nextNonce()
	if err != nil {
		w.err = err
		return w.err
	}
	plaintext, err := w.cipher.Open(w.buffer[:0], nonce, w.buffer, w.associatedData)
	if err != nil {
		w.err = NotAuthentic
		return w.err
	}
	if _, err = writeTo(w.w, plaintext); err != nil {
		w.err = err
		return w.err
	}

	w.buffer[0] = b
	w.offset = 1
	return nil
}

// Close writes any remaining decrypted bytes to the
// underlying io.Writer and returns any error
// encountered. If the underlying io.Writer implements
// Close it closes the underlying data stream as well.
//
// It is important to check the returned error
// since Close may return NotAuthentic indicating
// that some remaining bytes are invalid encrypted
// data.
//
// It safe to call close multiple times.
func (w *DecWriter) Close() error {
	if w.err != nil && w.err != ErrExceeded {
		return w.err
	}
	if !w.closed {
		w.closed = true

		w.associatedData[0] = 0x80
		binary.LittleEndian.PutUint32(w.nonce[w.cipher.NonceSize()-4:], w.seqNum)
		plaintext, err := w.cipher.Open(w.buffer[:0], w.nonce, w.buffer[:w.offset], w.associatedData)
		if err != nil {
			w.err = NotAuthentic
			return w.err
		}
		if _, w.err = writeTo(w.w, plaintext); w.err != nil {
			return w.err
		}
		if c, ok := w.w.(io.Closer); ok {
			w.err = c.Close()
			return w.err
		}
	}
	return nil
}

// ReadFrom behaves as specified by the io.ReadFrom interface.
// In particular, ReadFrom reads data from r until io.EOF or any
// error occurs, decrypts the data and writes the decrypted data
// to the underlying io.Writer. ReadFrom does not close the
// DecWriter nor the underlying data stream.
//
// ReadFrom returns the number of bytes read and any error except
// io.EOF. When ReadFrom fails to decrypt some data it returns
// NotAuthentic. This error indicates that some encrypted bytes
// have been (maliciously) modified.
//
// When ReadFrom cannot decrypt any more data securely it
// returns ErrExceeded. However, the DecWriter  must still
// be closed to complete the decryption and flush any
// remaining data to the underlying data stream.
//
// ReadFrom must not be called once the DecWriter has
// been closed.
func (w *DecWriter) ReadFrom(r io.Reader) (int64, error) {
	if w.closed {
		panic("sio: DecWriter is closed")
	}
	if w.err != nil {
		return 0, w.err
	}

	ciphertextLen := w.bufSize + w.cipher.Overhead()
	buffer := w.buffer[:1+ciphertextLen]

	nn, err := readFrom(r, buffer[:1+ciphertextLen])
	if err == io.EOF {
		w.offset = nn
		return int64(nn), nil
	}
	if err != nil {
		w.err = err
		return int64(nn), err
	}
	carry := buffer[ciphertextLen]

	nonce, err := w.nextNonce()
	if err != nil {
		w.err = err
		return int64(nn), w.err
	}
	plaintext, err := w.cipher.Open(buffer[:0], nonce, buffer[:ciphertextLen], w.associatedData)
	if err != nil {
		w.err = NotAuthentic
		return int64(nn), w.err
	}
	if _, err = writeTo(w.w, plaintext); err != nil {
		w.err = err
		return int64(nn), w.err
	}

	n := int64(nn)
	for {
		w.buffer[0] = carry
		nn, err = readFrom(r, buffer[1:1+ciphertextLen])
		n += int64(nn)
		if err == io.EOF {
			w.offset = 1 + nn
			return n, nil
		}
		if err != nil {
			w.err = err
			return n, w.err
		}
		carry = buffer[ciphertextLen]

		nonce, err = w.nextNonce()
		if err != nil {
			w.err = err
			return n, w.err
		}
		plaintext, err = w.cipher.Open(buffer[:0], nonce, buffer[:ciphertextLen], w.associatedData)
		if err != nil {
			w.err = NotAuthentic
			return n, w.err
		}
		if _, err = writeTo(w.w, plaintext); err != nil {
			w.err = err
			return n, w.err
		}
	}
}

func (w *DecWriter) nextNonce() ([]byte, error) {
	if w.seqNum == math.MaxUint32 {
		return nil, ErrExceeded
	}
	binary.LittleEndian.PutUint32(w.nonce[w.cipher.NonceSize()-4:], w.seqNum)
	w.seqNum++
	return w.nonce, nil
}
