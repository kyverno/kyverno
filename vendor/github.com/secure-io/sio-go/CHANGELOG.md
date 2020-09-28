# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.9] - 2019-10-29
### Added
 - The [`Algorithm`](https://godoc.org/github.com/secure-io/sio-go#Algorithm) type and
   four commonly used AEAD algorithms (AES-128-GCM, AES-256-GCM, ChaCha20-Poly1305 and
   XChaCha20-Poly1305).  
   The [`Algorithm.Stream`](https://godoc.org/github.com/secure-io/sio-go#Algorithm.Stream)
   method provides a way to create a `Stream` directly from a secret key instead of first 
   creating an AEAD instance explicitly.
 - The [`NativeAES`](https://godoc.org/github.com/secure-io/sio-go/sioutil#NativeAES) function
   in the `sioutil`. It allows users to determine whether an optimized (and constant time)
   implementation is available for their CPU.
 - Two helper functions (`Random` and `MustRandom`) for generating random bytes in `sioutil`.

## [0.2.0] - 2019-10-13
### Added
 - An (experimental) `sioutil` package with a similar scope like the std library `io/ioutil`
   package.
 - Fuzzing using [go-fuzz](https://github.com/dvyukov/go-fuzz) and [fuzzbuzz.io][https://fuzzbuzz.io].
       
### Changed
 - All exported errors are now actual constants instead of effectively constant variables. 
   ([27e1114](https://github.com/secure-io/sio-go/commit/27e11147b5ddc0a6cbc69d4f79c2273a70ce36eb))
   Also the `ErrAuth` has been renamed to `NotAuthentic`.
   This is a major breaking change since:
     1. An exported symbol has changed.
     2. The type of exported symbols has changed.
 - The [go documentation](https://godoc.org/github.com/secure-io/sio-go) has changed to
   be more explicit about how to use the API and what guarantees are provided.

## [0.1.0] - 2019-05-24
### Added
 - A first stream-based secure channel implementation for wrapping [`io.Reader`](https://golang.org/pkg/io/#Reader) 
  and [`io.Writer`](https://golang.org/pkg/io/#Writer)
 - A work-in-progress README.md
 - Issue and PR templates
