# Contributing to gotest.tools

Thank you for your interest in contributing to the project! Below are some
suggestions which may make the process easier.

## Pull requests

Pull requests for new features should generally be preceded by an issue
explaining the feature and why it is necessary.

Pull requests for bug fixes are always appreciated. They should almost always
include a test which fails without the bug fix.

## Dependencies

At this time both a `Gopkg.toml` for `dep` and `go.mod` for go modules exist in
the repo. The `Gopkg.toml` remains so that projects using earlier versions of Go
are able to find compatible versions of dependencies.

If you need to make a change to a dependency:

1. Update `Gopkg.toml`.
2. Run the following to sync the changes to `go.mod`.
   ```
   dep ensure
   rm go.mod go.sum
   go mod init
   gotestsum
   go mod tidy
   ```
