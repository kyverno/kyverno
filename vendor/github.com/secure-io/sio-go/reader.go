// Copyright (c) 2019 Andreas Auernhammer. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

package sio

import (
	"crypto/cipher"
	"encoding/binary"
	"io"
	"io/ioutil"
	"math"
	"sync"
)

// An EncReader encrypts and authenticates everything it reads
// from an underlying io.Reader.
type EncReader struct {
	r       io.Reader
	cipher  cipher.AEAD
	bufSize int

	seqNum         uint32
	nonce          []byte
	associatedData []byte

	buffer           []byte
	ciphertextBuffer []byte
	offset           int

	err               error
	carry             byte
	firstRead, closed bool
}

// Read behaves as specified by the io.Reader interface.
// In particular, Read reads up to len(p) encrypted bytes
// into p. It returns the number of bytes read (0 <= n <= len(p))
// and any error encountered while reading from the underlying
// io.Reader.
//
// When Read cannot encrypt any more bytes securely it returns
// ErrExceeded.
func (r *EncReader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return n, r.err
	}
	if r.firstRead {
		r.firstRead = false
		n, err = r.readFragment(p, 0)
		if err != nil {
			return n, err
		}
		p = p[n:]
	}
	if r.offset > 0 {
		nn := copy(p, r.ciphertextBuffer[r.offset:])
		n += nn
		if nn == len(p) {
			r.offset += nn
			return n, nil
		}
		p = p[nn:]
		r.offset = 0
	}
	if r.closed {
		return n, io.EOF
	}
	nn, err := r.readFragment(p, 1)
	return n + nn, err
}

// ReadByte behaves as specified by the io.ByteReader
// interface. In particular, ReadByte returns the next
// encrypted byte or any error encountered.
//
// When ReadByte cannot encrypt one more byte
// securely it returns ErrExceeded.
func (r *EncReader) ReadByte() (byte, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.firstRead {
		r.firstRead = false
		if _, err := r.readFragment(nil, 0); err != nil {
			return 0, err
		}
		b := r.ciphertextBuffer[0]
		r.offset = 1
		return b, nil
	}

	if r.offset > 0 && r.offset < len(r.ciphertextBuffer) {
		b := r.ciphertextBuffer[r.offset]
		r.offset++
		return b, nil
	}
	if r.closed {
		return 0, io.EOF
	}

	r.offset = 0
	if _, err := r.readFragment(nil, 1); err != nil {
		return 0, err
	}
	b := r.ciphertextBuffer[0]
	r.offset = 1
	return b, nil
}

// WriteTo behaves as specified by the io.WriterTo
// interface. In particular, WriteTo writes encrypted
// data to w until either there's no more data to write,
// an error occurs or no more data can be encrypted
// securely. When WriteTo cannot encrypt more data
// securely it returns ErrExceeded.
func (r *EncReader) WriteTo(w io.Writer) (int64, error) {
	var n int64
	if r.firstRead {
		r.firstRead = false
		nn, err := r.readFragment(r.buffer, 0)
		if err != nil && err != io.EOF {
			return n, err
		}
		nn, err = writeTo(w, r.buffer[:nn])
		if err != nil {
			return n, err
		}
		n += int64(nn)
		if r.closed {
			return n, nil
		}
	}
	if r.err != nil {
		return n, r.err
	}
	if r.offset > 0 {
		nn, err := writeTo(w, r.ciphertextBuffer[r.offset:])
		if err != nil {
			r.err = err
			return n, err
		}
		r.offset = 0
		n += int64(nn)
	}
	if r.closed {
		return n, io.EOF
	}
	for {
		nn, err := r.readFragment(r.buffer, 1)
		if err != nil && err != io.EOF {
			return n, err
		}
		nn, err = writeTo(w, r.buffer[:nn])
		if err != nil {
			r.err = err
			return n, err
		}
		n += int64(nn)
		if r.closed {
			return n, nil
		}
	}
}

func (r *EncReader) readFragment(p []byte, firstReadOffset int) (int, error) {
	if r.seqNum == 0 {
		r.err = ErrExceeded
		return 0, r.err
	}
	binary.LittleEndian.PutUint32(r.nonce[r.cipher.NonceSize()-4:], r.seqNum)
	r.seqNum++

	r.buffer[0] = r.carry
	n, err := readFrom(r.r, r.buffer[firstReadOffset:1+r.bufSize])
	switch {
	default:
		r.carry = r.buffer[r.bufSize]
		if len(p) < r.bufSize+r.cipher.Overhead() {
			r.ciphertextBuffer = r.cipher.Seal(r.buffer[:0], r.nonce, r.buffer[:r.bufSize], r.associatedData)
			r.offset = copy(p, r.ciphertextBuffer)
			return r.offset, nil
		}
		r.cipher.Seal(p[:0], r.nonce, r.buffer[:r.bufSize], r.associatedData)
		return r.bufSize + r.cipher.Overhead(), nil
	case err == io.EOF:
		r.closed = true
		r.associatedData[0] = 0x80
		if len(p) < firstReadOffset+n+r.cipher.Overhead() {
			r.ciphertextBuffer = r.cipher.Seal(r.buffer[:0], r.nonce, r.buffer[:firstReadOffset+n], r.associatedData)
			r.offset = copy(p, r.ciphertextBuffer)
			return r.offset, nil
		}
		r.cipher.Seal(p[:0], r.nonce, r.buffer[:firstReadOffset+n], r.associatedData)
		return firstReadOffset + n + r.cipher.Overhead(), io.EOF
	case err != nil:
		r.err = err
		return 0, r.err
	}
}

// A DecReader decrypts and verifies everything it reads
// from an underlying io.Reader. A DecReader never returns
// invalid (i.e. not authentic) data.
type DecReader struct {
	r      io.Reader
	cipher cipher.AEAD

	bufSize        int
	seqNum         uint32
	nonce          []byte
	associatedData []byte

	buffer          []byte
	plaintextBuffer []byte
	offset          int

	err               error
	carry             byte
	firstRead, closed bool
}

// Read behaves like specified by the io.Reader interface.
// In particular, Read reads up to len(p) decrypted bytes
// into p. It returns the number of bytes read (0 <= n <= len(p))
// and any error encountered while reading from the underlying
// io.Reader.
//
// When Read fails to decrypt some data returned by the underlying
// io.Reader it returns NotAuthentic. This error indicates
// that the encrypted data has been (maliciously) modified.
//
// When Read cannot decrypt more bytes securely it returns
// ErrExceeded. However, this can only happen when the
// underlying io.Reader returns valid but too many
// encrypted bytes. Therefore, ErrExceeded indicates
// a misbehaving producer of encrypted data.
func (r *DecReader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return n, r.err
	}
	if r.firstRead {
		r.firstRead = false
		n, err = r.readFragment(p, 0)
		if err != nil {
			return n, err
		}
		p = p[n:]
	}
	if r.offset > 0 {
		nn := copy(p, r.plaintextBuffer[r.offset:])
		n += nn
		if nn == len(p) {
			r.offset += nn
			return n, nil
		}
		p = p[nn:]
		r.offset = 0
	}
	if r.closed {
		return n, io.EOF
	}
	nn, err := r.readFragment(p, 1)
	return n + nn, err
}

// ReadByte behaves as specified by the io.ByteReader
// interface. In particular, ReadByte returns the next
// decrypted byte or any error encountered.
//
// When ReadByte fails to decrypt the next byte returned by
// the underlying io.Reader it returns NotAuthentic. This
// error indicates that the encrypted byte has been
// (maliciously) modified.
//
// When Read cannot decrypt one more byte securely it
// returns ErrExceeded. However, this can only happen
// when the underlying io.Reader returns valid but too
// many encrypted bytes. Therefore, ErrExceeded indicates
// a misbehaving producer of encrypted data.
func (r *DecReader) ReadByte() (byte, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.firstRead {
		r.firstRead = false
		if _, err := r.readFragment(nil, 0); err != nil {
			return 0, err
		}
		b := r.plaintextBuffer[0]
		r.offset = 1
		return b, nil
	}
	if r.offset > 0 && r.offset < len(r.plaintextBuffer) {
		b := r.plaintextBuffer[r.offset]
		r.offset++
		return b, nil
	}
	if r.closed {
		return 0, io.EOF
	}

	r.offset = 0
	if _, err := r.readFragment(nil, 1); err != nil {
		return 0, err
	}
	b := r.plaintextBuffer[0]
	r.offset = 1
	return b, nil
}

// WriteTo behaves as specified by the io.WriterTo
// interface. In particular, WriteTo writes decrypted
// data to w until either there's no more data to write,
// an error occurs or the encrypted data is invalid.
//
// When WriteTo fails to decrypt some data it returns
// NotAuthentic. This error indicates that the encrypted
// bytes has been (maliciously) modified.
//
// When WriteTo cannot decrypt any more bytes securely it
// returns ErrExceeded. However, this can only happen
// when the underlying io.Reader returns valid but too
// many encrypted bytes. Therefore, ErrExceeded indicates
// a misbehaving producer of encrypted data.
func (r *DecReader) WriteTo(w io.Writer) (int64, error) {
	var n int64
	if r.err != nil {
		return n, r.err
	}
	if r.firstRead {
		r.firstRead = false
		nn, err := r.readFragment(r.buffer, 0)
		if err != nil && err != io.EOF {
			return n, err
		}
		nn, err = writeTo(w, r.buffer[:nn])
		if err != nil {
			return n, err
		}
		n += int64(nn)
		if r.closed {
			return n, nil
		}
	}
	if r.offset > 0 {
		nn, err := writeTo(w, r.plaintextBuffer[r.offset:])
		if err != nil {
			r.err = err
			return n, err
		}
		r.offset = 0
		n += int64(nn)
	}
	if r.closed {
		return n, io.EOF
	}
	for {
		nn, err := r.readFragment(r.buffer, 1)
		if err != nil && err != io.EOF {
			return n, err
		}
		nn, err = writeTo(w, r.buffer[:nn])
		if err != nil {
			r.err = err
			return n, err
		}
		n += int64(nn)
		if r.closed {
			return n, nil
		}
	}
}

func (r *DecReader) readFragment(p []byte, firstReadOffset int) (int, error) {
	if r.seqNum == 0 {
		r.err = ErrExceeded
		return 0, r.err
	}
	binary.LittleEndian.PutUint32(r.nonce[r.cipher.NonceSize()-4:], r.seqNum)
	r.seqNum++

	ciphertextLen := r.bufSize + r.cipher.Overhead()

	r.buffer[0] = r.carry
	n, err := readFrom(r.r, r.buffer[firstReadOffset:1+ciphertextLen])
	switch {
	default:
		r.carry = r.buffer[ciphertextLen]
		if len(p) < r.bufSize {
			r.plaintextBuffer, err = r.cipher.Open(r.buffer[:0], r.nonce, r.buffer[:ciphertextLen], r.associatedData)
			if err != nil {
				r.err = NotAuthentic
				return 0, r.err
			}
			r.offset = copy(p, r.plaintextBuffer)
			return r.offset, nil
		}
		if _, err = r.cipher.Open(p[:0], r.nonce, r.buffer[:ciphertextLen], r.associatedData); err != nil {
			r.err = NotAuthentic
			return 0, r.err
		}
		return r.bufSize, nil
	case err == io.EOF:
		if firstReadOffset+n < r.cipher.Overhead() {
			r.err = NotAuthentic
			return 0, r.err
		}
		r.closed = true
		r.associatedData[0] = 0x80
		if len(p) < firstReadOffset+n-r.cipher.Overhead() {
			r.plaintextBuffer, err = r.cipher.Open(r.buffer[:0], r.nonce, r.buffer[:firstReadOffset+n], r.associatedData)
			if err != nil {
				r.err = NotAuthentic
				return 0, r.err
			}
			r.offset = copy(p, r.plaintextBuffer)
			return r.offset, nil
		}
		if _, err = r.cipher.Open(p[:0], r.nonce, r.buffer[:firstReadOffset+n], r.associatedData); err != nil {
			r.err = NotAuthentic
			return 0, r.err

		}
		return firstReadOffset + n - r.cipher.Overhead(), io.EOF
	case err != nil:
		r.err = err
		return 0, r.err
	}
}

// A DecReaderAt decrypts and verifies everything it reads
// from an underlying io.ReaderAt. A DecReaderAt never returns
// invalid (i.e. not authentic) data.
type DecReaderAt struct {
	r      io.ReaderAt
	cipher cipher.AEAD

	bufPool sync.Pool
	bufSize int

	nonce          []byte
	associatedData []byte
}

// ReadAt behaves like specified by the io.ReaderAt interface.
// In particular, ReadAt reads len(p) decrypted bytes into p.
// It returns the number of bytes read (0 <= n <= len(p))
// and any error encountered while reading from the underlying
// io.Reader. When ReadAt returns n < len(p), it returns a non-nil
// error explaining why more bytes were not returned.
//
// When ReadAt fails to decrypt some data returned by the underlying
// io.ReaderAt it returns NotAuthentic. This error indicates
// that the encrypted data has been (maliciously) modified.
//
// When ReadAt cannot decrypt more bytes securely it returns
// ErrExceeded. However, this can only happen when the
// underlying io.ReaderAt returns valid but too many
// encrypted bytes. Therefore, ErrExceeded indicates
// a misbehaving producer of encrypted data.
func (r *DecReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	if offset < 0 {
		return 0, errorType("sio: DecReaderAt.ReadAt: offset is negative")
	}

	t := offset / int64(r.bufSize)
	if t+1 > math.MaxUint32 {
		return 0, ErrExceeded
	}

	buffer := r.bufPool.Get().(*[]byte)
	defer r.bufPool.Put(buffer)

	decReader := DecReader{
		r:              &sectionReader{r: r.r, off: t * int64(r.bufSize+r.cipher.Overhead())},
		cipher:         r.cipher,
		bufSize:        r.bufSize,
		nonce:          make([]byte, r.cipher.NonceSize()),
		associatedData: make([]byte, 1+r.cipher.Overhead()),
		seqNum:         1 + uint32(t),
		buffer:         *buffer,
		firstRead:      true,
	}
	copy(decReader.nonce, r.nonce)
	copy(decReader.associatedData, r.associatedData)

	if k := offset % int64(r.bufSize); k > 0 {
		if _, err := io.CopyN(ioutil.Discard, &decReader, k); err != nil {
			return 0, err
		}
	}
	return readFrom(&decReader, p)
}

// Use a custom sectionReader since io.SectionReader
// demands a read limit.

type sectionReader struct {
	r   io.ReaderAt
	off int64
	err error
}

func (r *sectionReader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}

	var n int
	n, r.err = r.r.ReadAt(p, r.off)
	r.off += int64(n)
	return n, r.err
}
