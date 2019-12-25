// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package traits

import "github.com/tamerh/cel-go/common/types/ref"

// Mapper interface which aggregates the traits of a maps.
type Mapper interface {
	ref.Val
	Container
	Indexer
	Iterable
	Sizer

	// Find a value by key if it exists.
	//
	// When the value exists, the result will be non-nil, true. When the value does not exist,
	// the result will be nil, false. When an error occurs, the value will be non-nil and the
	// found result false.
	Find(key ref.Val) (ref.Val, bool)
}
