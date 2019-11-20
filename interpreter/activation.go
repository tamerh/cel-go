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

package interpreter

import (
	"errors"
	"fmt"
	"sync"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// Activation used to resolve identifiers by name and references by id.
//
// An Activation is the primary mechanism by which a caller supplies input into a CEL program.
type Activation interface {
	// ResolveName returns a value from the activation by qualified name, or false if the name
	// could not be found.
	ResolveName(name string) (interface{}, bool)

	// Parent returns the parent of the current activation, may be nil.
	// If non-nil, the parent will be searched during resolve calls.
	Parent() Activation
}

// EmptyActivation returns a variable free activation.
func EmptyActivation() Activation {
	// This call cannot fail.
	a, _ := NewActivation(map[string]interface{}{})
	return a
}

// NewActivation returns an activation based on a map-based binding where the map keys are
// expected to be qualified names used with ResolveName calls.
//
// The input `bindings` may either be of type `Activation` or `map[string]interface{}`.
//
// When the bindings are a `map` form whose values are not of `ref.Val` type, the values will be
// converted to CEL values (if possible) using the `types.DefaultTypeAdapter`.
func NewActivation(bindings interface{}) (Activation, error) {
	if bindings == nil {
		return nil, errors.New("bindings must be non-nil")
	}
	a, isActivation := bindings.(Activation)
	if isActivation {
		return a, nil
	}
	m, isMap := bindings.(map[string]interface{})
	if !isMap {
		return nil, fmt.Errorf(
			"activation input must be an activation or map[string]interface: got %T",
			bindings)
	}
	return &mapActivation{bindings: m}, nil
}

// mapActivation which implements Activation and maps of named values.
//
// Named bindings may lazily supply values by providing a function which accepts no arguments and
// produces an interface value.
type mapActivation struct {
	adapter  ref.TypeAdapter
	bindings map[string]interface{}
}

// Parent implements the Activation interface method.
func (a *mapActivation) Parent() Activation {
	return nil
}

// ResolveName implements the Activation interface method.
func (a *mapActivation) ResolveName(name string) (interface{}, bool) {
	obj, found := a.bindings[name]
	if !found {
		return nil, false
	}
	fn, isLazy := obj.(func() ref.Val)
	if isLazy {
		obj = fn()
		a.bindings[name] = obj
	}
	return obj, found
}

// hierarchicalActivation which implements Activation and contains a parent and
// child activation.
type hierarchicalActivation struct {
	parent Activation
	child  Activation
}

// Parent implements the Activation interface method.
func (a *hierarchicalActivation) Parent() Activation {
	return a.parent
}

// ResolveName implements the Activation interface method.
func (a *hierarchicalActivation) ResolveName(name string) (interface{}, bool) {
	if object, found := a.child.ResolveName(name); found {
		return object, found
	}
	return a.parent.ResolveName(name)
}

// NewHierarchicalActivation takes two activations and produces a new one which prioritizes
// resolution in the child first and parent(s) second.
func NewHierarchicalActivation(parent Activation, child Activation) Activation {
	return &hierarchicalActivation{parent, child}
}

// UnknownActivation returns an Activation that returns a 'types.Unknown' value for all requests
// to ResolveName.
func UnknownActivation() Activation {
	a, _ := PartialActivation(EmptyActivation())
	return a
}

// PartialActivation returns an Activation that will resolve identifier names if present, otherwise
// will return 'types.Unknown'.
func PartialActivation(bindings interface{}) (Activation, error) {
	a, err := NewActivation(bindings)
	if err != nil {
		return nil, err
	}
	return &partActivation{known: a}, nil
}

type partActivation struct {
	known Activation
}

// ResolveName implements the Activation interface method.
func (a *partActivation) ResolveName(name string) (interface{}, bool) {
	obj, found := a.known.ResolveName(name)
	if found {
		return obj, true
	}
	return types.Unknown{}, true
}

// Parent implements the Activation interface method.
func (a *partActivation) Parent() Activation {
	return a.known.Parent()
}

// newVarActivation returns a new varActivation instance.
func newVarActivation(parent Activation, name string) *varActivation {
	return &varActivation{
		parent: parent,
		name:   name,
	}
}

// varActivation represents a single mutable variable binding.
//
// This activation type should only be used within folds as the fold loop controls the object
// life-cycle.
type varActivation struct {
	parent Activation
	name   string
	val    ref.Val
}

// Parent implements the Activation interface method.
func (v *varActivation) Parent() Activation {
	return v.parent
}

// ResolveName implements the Activation interface method.
func (v *varActivation) ResolveName(name string) (interface{}, bool) {
	if name == v.name {
		return v.val, true
	}
	return v.parent.ResolveName(name)
}

var (
	// pool of var activations to reduce allocations during folds.
	varActivationPool = &sync.Pool{
		New: func() interface{} {
			return &varActivation{}
		},
	}
)
