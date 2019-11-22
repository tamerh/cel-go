// Copyright 2019 Google LLC
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

package interpreter

import (
	"github.com/google/cel-go/common/types/ref"
)

// Resolver provides methods for finding values by name and resolving qualified attributes from
// them.
type Resolver interface {
	// ResolveQualifiers returns the value at the qualified field path or error if the one or more
	// of the qualifier paths could not be found.
	ResolveQualifiers(Activation, interface{}, []Qualifier) (interface{}, error)
}

// NewResolver returns a default Resolver which is cabable of resolving types by simple names, and
// can resolve qualifiers on CEL values using the supported qualifier types: bool, int, string,
// and uint.
func NewResolver(a ref.TypeAdapter) Resolver {
	return &resolver{adapter: a}
}

type resolver struct {
	adapter ref.TypeAdapter
}

// ResolveQualifiers resolves static and dynamic qualifiers on the input object.
//
// Resolution of qualifiers on Go simple and aggregate types does not require marshalling of
// intermediate results to CEL ref.Val instances; however, proto message types and Go structs
// will be marshalled to CEL ref.Val's which can result in slower resolution time. Custom
// Resolvers may be used to improve performance.
func (res *resolver) ResolveQualifiers(vars Activation,
	obj interface{},
	quals []Qualifier) (interface{}, error) {
	return obj, nil
}
