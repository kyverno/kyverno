/*
Copyright 2019 The logr Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testing

import "github.com/go-logr/logr"

// NullLogger is a logr.Logger that does nothing.
type NullLogger struct{}

var _ logr.Logger = NullLogger{}

func (_ NullLogger) Info(_ string, _ ...interface{}) {
	// Do nothing.
}

func (_ NullLogger) Enabled() bool {
	return false
}

func (_ NullLogger) Error(_ error, _ string, _ ...interface{}) {
	// Do nothing.
}

func (log NullLogger) V(_ int) logr.Logger {
	return log
}

func (log NullLogger) WithName(_ string) logr.Logger {
	return log
}

func (log NullLogger) WithValues(_ ...interface{}) logr.Logger {
	return log
}
