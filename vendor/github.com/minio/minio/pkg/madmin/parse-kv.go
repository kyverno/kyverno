/*
 * MinIO Cloud Storage, (C) 2019 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package madmin

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"unicode"
)

// KV - is a shorthand of each key value.
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// KVS - is a shorthand for some wrapper functions
// to operate on list of key values.
type KVS []KV

// Empty - return if kv is empty
func (kvs KVS) Empty() bool {
	return len(kvs) == 0
}

<<<<<<< HEAD
// Set sets a value, if not sets a default value.
func (kvs *KVS) Set(key, value string) {
	for i, kv := range *kvs {
		if kv.Key == key {
			(*kvs)[i] = KV{
				Key:   key,
				Value: value,
			}
			return
		}
	}
	*kvs = append(*kvs, KV{
		Key:   key,
		Value: value,
	})
}

=======
>>>>>>> 524_bug
// Get - returns the value of a key, if not found returns empty.
func (kvs KVS) Get(key string) string {
	v, ok := kvs.Lookup(key)
	if ok {
		return v
	}
	return ""
}

// Lookup - lookup a key in a list of KVS
func (kvs KVS) Lookup(key string) (string, bool) {
	for _, kv := range kvs {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return "", false
}
<<<<<<< HEAD

// Target signifies an individual target
type Target struct {
	SubSystem string `json:"subSys"`
	KVS       KVS    `json:"kvs"`
}
=======
>>>>>>> 524_bug

// Targets sub-system targets
type Targets []Target

// Standard config keys and values.
const (
<<<<<<< HEAD
	EnableKey  = "enable"
	CommentKey = "comment"

	// Enable values
	EnableOn  = "on"
	EnableOff = "off"
=======
	stateKey   = "state"
	commentKey = "comment"
>>>>>>> 524_bug
)

func (kvs KVS) String() string {
	var s strings.Builder
	for _, kv := range kvs {
<<<<<<< HEAD
		// Do not need to print state which is on.
		if kv.Key == EnableKey && kv.Value == EnableOn {
			continue
		}
		if kv.Key == CommentKey && kv.Value == "" {
=======
		// Do not need to print state
		if kv.Key == stateKey {
			continue
		}
		if kv.Key == commentKey && kv.Value == "" {
>>>>>>> 524_bug
			continue
		}
		s.WriteString(kv.Key)
		s.WriteString(KvSeparator)
<<<<<<< HEAD
		spc := HasSpace(kv.Value)
=======
		spc := hasSpace(kv.Value)
>>>>>>> 524_bug
		if spc {
			s.WriteString(KvDoubleQuote)
		}
		s.WriteString(kv.Value)
		if spc {
			s.WriteString(KvDoubleQuote)
		}
		s.WriteString(KvSpaceSeparator)
	}
	return s.String()
}

// Count - returns total numbers of target
func (t Targets) Count() int {
	return len(t)
}

// HasSpace - returns if given string has space.
func HasSpace(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

func hasSpace(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

func (t Targets) String() string {
	var s strings.Builder
	count := t.Count()
<<<<<<< HEAD
	// Print all "on" states entries
	for _, targetKV := range t {
		kv := targetKV.KVS
		count--
		s.WriteString(targetKV.SubSystem)
		s.WriteString(KvSpaceSeparator)
		s.WriteString(kv.String())
		if len(t) > 1 && count > 0 {
			s.WriteString(KvNewline)
=======
	for subSys, targetKV := range t {
		for target, kv := range targetKV {
			count--
			s.WriteString(subSys)
			if target != Default {
				s.WriteString(SubSystemSeparator)
				s.WriteString(target)
			}
			s.WriteString(KvSpaceSeparator)
			s.WriteString(kv.String())
			if (len(t) > 1 || len(targetKV) > 1) && count > 0 {
				s.WriteString(KvNewline)
			}
>>>>>>> 524_bug
		}
	}
	return s.String()
}

// Constant separators
const (
	SubSystemSeparator = `:`
	KvSeparator        = `=`
<<<<<<< HEAD
	KvComment          = `#`
=======
>>>>>>> 524_bug
	KvSpaceSeparator   = ` `
	KvNewline          = "\n"
	KvDoubleQuote      = `"`
	KvSingleQuote      = `'`

	Default = `_`
)

// SanitizeValue - this function is needed, to trim off single or double quotes, creeping into the values.
func SanitizeValue(v string) string {
	v = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(v), KvDoubleQuote), KvDoubleQuote)
	return strings.TrimSuffix(strings.TrimPrefix(v, KvSingleQuote), KvSingleQuote)
}

// AddTarget - adds new targets, by parsing the input string s.
func (t *Targets) AddTarget(s string) error {
	inputs := strings.SplitN(s, KvSpaceSeparator, 2)
	if len(inputs) <= 1 {
		return fmt.Errorf("invalid number of arguments '%s'", s)
	}

	subSystemValue := strings.SplitN(inputs[0], SubSystemSeparator, 2)
	if len(subSystemValue) == 0 {
		return fmt.Errorf("invalid number of arguments %s", s)
	}

	var kvs = KVS{}
	var prevK string
	for _, v := range strings.Fields(inputs[1]) {
		kv := strings.SplitN(v, KvSeparator, 2)
		if len(kv) == 0 {
			continue
		}
		if len(kv) == 1 && prevK != "" {
<<<<<<< HEAD
			value := strings.Join([]string{
				kvs.Get(prevK),
				SanitizeValue(kv[0]),
			}, KvSpaceSeparator)
			kvs.Set(prevK, value)
=======
			kvs = append(kvs, KV{
				Key:   prevK,
				Value: strings.Join([]string{kvs.Get(prevK), sanitizeValue(kv[0])}, KvSpaceSeparator),
			})
>>>>>>> 524_bug
			continue
		}
		if len(kv) == 2 {
			prevK = kv[0]
			kvs.Set(prevK, SanitizeValue(kv[1]))
			continue
		}
<<<<<<< HEAD
		return fmt.Errorf("value for key '%s' cannot be empty", kv[0])
=======
		prevK = kv[0]
		kvs = append(kvs, KV{
			Key:   kv[0],
			Value: sanitizeValue(kv[1]),
		})
>>>>>>> 524_bug
	}

	for i := range *t {
		if (*t)[i].SubSystem == inputs[0] {
			(*t)[i] = Target{
				SubSystem: inputs[0],
				KVS:       kvs,
			}
			return nil
		}
	}
	*t = append(*t, Target{
		SubSystem: inputs[0],
		KVS:       kvs,
	})
	return nil
}

// ParseSubSysTarget - parse sub-system target
func ParseSubSysTarget(buf []byte) (Targets, error) {
	var targets Targets
	bio := bufio.NewScanner(bytes.NewReader(buf))
	for bio.Scan() {
		if err := targets.AddTarget(bio.Text()); err != nil {
			return nil, err
		}
	}
	if err := bio.Err(); err != nil {
		return nil, err
	}
	return targets, nil
}
