DirectIO
========

This is library for the Go language to enable use of Direct IO under
all supported OSes of Go (except openbsd and plan9).

Direct IO does IO to and from disk without buffering data in the OS.
It is useful when you are reading or writing lots of data you don't
want to fill the OS cache up with.

See here for package docs

  http://godoc.org/github.com/ncw/directio

Install
-------

Directio is a Go library and installs in the usual way

    go get github.com/ncw/directio

Usage
-----

Instead of using os.OpenFile use directio.OpenFile

	in, err := directio.OpenFile(file, os.O_RDONLY, 0666)

And when reading or writing blocks, make sure you do them in chunks of
directio.BlockSize using memory allocated by directio.AlignedBlock

	block := directio.AlignedBlock(directio.BlockSize)
        _, err := io.ReadFull(in, block)

License
-------

This is free software under the terms of MIT the license (check the
COPYING file included in this package).

Contact and support
-------------------

The project website is at:

- https://github.com/ncw/directio

There you can file bug reports, ask for help or contribute patches.

Authors
-------

- Nick Craig-Wood <nick@craig-wood.com>

Contributors
------------

- Pavel Odintsov <pavel.odintsov@gmail.com>
