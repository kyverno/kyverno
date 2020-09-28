// Direct IO for windows

package directio

import (
	"os"
	"syscall"
	"unicode/utf16"
)

const (
	// Size to align the buffer to
	AlignSize = 4096

	// Minimum block size
	BlockSize = 4096

	// Extra flags for windows
	FILE_FLAG_NO_BUFFERING  = 0x20000000
	FILE_FLAG_WRITE_THROUGH = 0x80000000
)

// utf16FromString returns the UTF-16 encoding of the UTF-8 string
// s, with a terminating NUL added. If s contains a NUL byte at any
// location, it returns (nil, EINVAL).
//
// FIXME copied from go source
func utf16FromString(s string) ([]uint16, error) {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return nil, syscall.EINVAL
		}
	}
	return utf16.Encode([]rune(s + "\x00")), nil
}

// OpenFile is a modified version of os.OpenFile which sets the
// passes the following flags to windows CreateFile.
//
// The FILE_FLAG_NO_BUFFERING takes this concept one step further and
// eliminates all read-ahead file buffering and disk caching as well,
// so that all reads are guaranteed to come from the file and not from
// any system buffer or disk cache. When using FILE_FLAG_NO_BUFFERING,
// disk reads and writes must be done on sector boundaries, and buffer
// addresses must be aligned on disk sector boundaries in memory.
//
// FIXME copied from go source then modified
func OpenFile(path string, mode int, perm os.FileMode) (file *os.File, err error) {
	if len(path) == 0 {
		return nil, &os.PathError{"open", path, syscall.ERROR_FILE_NOT_FOUND}
	}
	pathp, err := utf16FromString(path)
	if err != nil {
		return nil, &os.PathError{"open", path, err}
	}
	var access uint32
	switch mode & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR) {
	case os.O_RDONLY:
		access = syscall.GENERIC_READ
	case os.O_WRONLY:
		access = syscall.GENERIC_WRITE
	case os.O_RDWR:
		access = syscall.GENERIC_READ | syscall.GENERIC_WRITE
	}
	if mode&syscall.O_CREAT != 0 {
		access |= syscall.GENERIC_WRITE
	}
	if mode&os.O_APPEND != 0 {
		access &^= syscall.GENERIC_WRITE
		access |= syscall.FILE_APPEND_DATA
	}
	sharemode := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE)
	var sa *syscall.SecurityAttributes
	var createmode uint32
	switch {
	case mode&(syscall.O_CREAT|os.O_EXCL) == (syscall.O_CREAT | os.O_EXCL):
		createmode = syscall.CREATE_NEW
	case mode&(syscall.O_CREAT|os.O_TRUNC) == (syscall.O_CREAT | os.O_TRUNC):
		createmode = syscall.CREATE_ALWAYS
	case mode&syscall.O_CREAT == syscall.O_CREAT:
		createmode = syscall.OPEN_ALWAYS
	case mode&os.O_TRUNC == os.O_TRUNC:
		createmode = syscall.TRUNCATE_EXISTING
	default:
		createmode = syscall.OPEN_EXISTING
	}
	h, e := syscall.CreateFile(&pathp[0], access, sharemode, sa, createmode, syscall.FILE_ATTRIBUTE_NORMAL|FILE_FLAG_NO_BUFFERING|FILE_FLAG_WRITE_THROUGH, 0)
	if e != nil {
		return nil, &os.PathError{"open", path, e}
	}
	return os.NewFile(uintptr(h), path), nil
}
