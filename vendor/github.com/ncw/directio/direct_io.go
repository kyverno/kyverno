// This is library for the Go language to enable use of Direct IO under
// all supported OSes of Go.
//
// Direct IO does IO to and from disk without buffering data in the OS.
// It is useful when you are reading or writing lots of data you don't
// want to fill the OS cache up with.
//
// Instead of using os.OpenFile use directio.OpenFile
//
// 	in, err := directio.OpenFile(file, os.O_RDONLY, 0666)
//
// And when reading or writing blocks, make sure you do them in chunks of
// directio.BlockSize using memory allocated by directio.AlignedBlock
//
// 	block := directio.AlignedBlock(BlockSize)
//         _, err := io.ReadFull(in, block)
package directio

import (
	"log"
	"unsafe"
)

// alignment returns alignment of the block in memory
// with reference to AlignSize
//
// Can't check alignment of a zero sized block as &block[0] is invalid
func alignment(block []byte, AlignSize int) int {
	return int(uintptr(unsafe.Pointer(&block[0])) & uintptr(AlignSize-1))
}

// AlignedBlock returns []byte of size BlockSize aligned to a multiple
// of AlignSize in memory (must be power of two)
func AlignedBlock(BlockSize int) []byte {
	block := make([]byte, BlockSize+AlignSize)
	if AlignSize == 0 {
		return block
	}
	a := alignment(block, AlignSize)
	offset := 0
	if a != 0 {
		offset = AlignSize - a
	}
	block = block[offset : offset+BlockSize]
	// Can't check alignment of a zero sized block
	if BlockSize != 0 {
		a = alignment(block, AlignSize)
		if a != 0 {
			log.Fatal("Failed to align block")
		}
	}
	return block
}
