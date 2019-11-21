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
	"fmt"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// Attribute values are a variable or value with an optional set of qualifiers, such as field, key,
// or index accesses.
type Attribute interface {
	// ID is the expression identifier where the attribute first appears.
	ID() int64

	// Qualify adds an additional qualifier on the Attribute or error if the qualification is not
	// a supported proto map key type.
	Qualify(id int64, v interface{}) (Attribute, error)

	// Resolve returns the qualified Attribute value from the current Activation and Resolver, or
	// error if the qualification is not defined.
	Resolve(Activation, Resolver) (interface{}, error)
}

// AbsoluteAttribute refers to a variable value and an optional qualifier path.
//
// The namespaceNames represent the names the variable could have based on namespace
// resolution rules.
func AbsoluteAttribute(id int64, namespacedNames []string) Attribute {
	return &absoluteAttribute{
		id:             id,
		namespaceNames: namespacedNames,
		qualifiers:     []Qualifier{},
	}
}

type absoluteAttribute struct {
	id             int64
	namespaceNames []string
	qualifiers     []Qualifier
}

// ID implements the Attribute interface method.
func (a *absoluteAttribute) ID() int64 {
	return a.id
}

// Qualify implements the Attribute interface method.
func (a *absoluteAttribute) Qualify(id int64, v interface{}) (Attribute, error) {
	qual, err := newQualifier(id, v)
	if err != nil {
		return nil, err
	}
	a.qualifiers = append(a.qualifiers, qual)
	return a, nil
}

// Resolve iterates through the namespaced variable names until one is found in the Activation,
// and the the standard qualifier resolution logic is applied.
//
// If the variable name is not found an error is returned.
func (a *absoluteAttribute) Resolve(vars Activation, res Resolver) (interface{}, error) {
	for _, nm := range a.namespaceNames {
		op, found := vars.ResolveName(nm)
		_, isUnk := op.(types.Unknown)
		if found && !isUnk {
			if len(a.qualifiers) == 0 {
				return op, nil
			}
			return res.ResolveQualifiers(vars, op, a.qualifiers)
		}
		// Attempt to resolve the qualified type name if the name is not a variable identifier.
		typ, found := res.ResolveName(nm)
		if found {
			if len(a.qualifiers) == 0 {
				return typ, nil
			}
			return nil, fmt.Errorf("no such attribute: %v", typ)
		}
		if isUnk {
			return types.Unknown{a.ID()}, nil
		}
	}
	return nil, fmt.Errorf("no such attribute: %v", a)
}

// RelativeAttribute refers to an expression and an optional qualifier path.
func RelativeAttribute(id int64, operand Interpretable) Attribute {
	return &relativeAttribute{
		id:         id,
		operand:    operand,
		qualifiers: []Qualifier{},
	}
}

type relativeAttribute struct {
	id         int64
	operand    Interpretable
	qualifiers []Qualifier
}

// ID is an implementation of the Attribute interface method.
func (a *relativeAttribute) ID() int64 {
	return a.id
}

// Qualify implements the Attribute interface method.
func (a *relativeAttribute) Qualify(id int64, v interface{}) (Attribute, error) {
	qual, err := newQualifier(id, v)
	if err != nil {
		return nil, err
	}
	a.qualifiers = append(a.qualifiers, qual)
	return a, nil
}

// Resolve expression value and qualifier relative to the expression result.
func (a *relativeAttribute) Resolve(vars Activation, res Resolver) (interface{}, error) {
	v := a.operand.Eval(vars)
	if types.IsError(v) {
		return nil, v.Value().(error)
	}
	return res.ResolveQualifiers(vars, v, a.qualifiers)
}

// ConditionalAttribute supports the case where an attribute selection may occur on a conditional
// expression, e.g. (cond ? a : b).c
func ConditionalAttribute(id int64, expr Interpretable, t, f Attribute) Attribute {
	return &conditionalAttribute{
		id:     id,
		expr:   expr,
		truthy: t,
		falsy:  f,
	}
}

type conditionalAttribute struct {
	id     int64
	expr   Interpretable
	truthy Attribute
	falsy  Attribute
}

// ID is an implementation of the Attribute interface method.
func (a *conditionalAttribute) ID() int64 {
	return a.id
}

// Qualify appends the same qualifier to both sides of the conditional, in effect managing the
// qualification of alternate attributes.
func (a *conditionalAttribute) Qualify(id int64, v interface{}) (Attribute, error) {
	_, err := a.truthy.Qualify(id, v)
	if err != nil {
		return nil, err
	}
	_, err = a.falsy.Qualify(id, v)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Resolve evaluates the condition, and then resolves the truthy or falsy branch accordingly.
func (a *conditionalAttribute) Resolve(vars Activation, res Resolver) (interface{}, error) {
	val := a.expr.Eval(vars)
	if types.IsError(val) {
		return nil, val.Value().(error)
	}
	if val == types.True {
		return a.truthy.Resolve(vars, res)
	}
	if val == types.False {
		return a.falsy.Resolve(vars, res)
	}
	if types.IsUnknown(val) {
		return val, nil
	}
	return nil, types.ValOrErr(val, "no such overload").Value().(error)
}

// OneofAttribute collects variants of unchecked AbsoluteAttribute values which could either be
// direct variable accesses or some combination of variable access with qualification.
func OneofAttribute(id int64, namespacedNames []string) Attribute {
	return &oneofAttribute{
		id: id,
		attrs: []*absoluteAttribute{
			&absoluteAttribute{
				id:             id,
				namespaceNames: namespacedNames,
			},
		},
	}
}

type oneofAttribute struct {
	id    int64
	attrs []*absoluteAttribute
}

// ID is an implementation of the Attribute interface method.
func (a *oneofAttribute) ID() int64 {
	return a.id
}

// Qualify adds a qualifier to each possible attribute variant in the oneof, and also creates a new
// namespaced variable from the qualified value.
func (a *oneofAttribute) Qualify(id int64, v interface{}) (Attribute, error) {
	str, isStr := v.(string)
	var augmentedNames []string
	for _, attr := range a.attrs {
		if isStr && len(attr.qualifiers) == 0 {
			augmentedNames = make([]string,
				len(attr.namespaceNames),
				len(attr.namespaceNames))
			for i, name := range attr.namespaceNames {
				augmentedNames[i] = fmt.Sprintf("%s.%s", name, str)
			}
		}
		attr.Qualify(id, v)
	}
	a.attrs = append([]*absoluteAttribute{
		&absoluteAttribute{
			id:             id,
			namespaceNames: augmentedNames,
			qualifiers:     []Qualifier{},
		},
	}, a.attrs...)
	return a, nil
}

// Resolve follows the variable resolution
func (a *oneofAttribute) Resolve(vars Activation, res Resolver) (interface{}, error) {
	for _, attr := range a.attrs {
		for _, nm := range attr.namespaceNames {
			varVal, found := vars.ResolveName(nm)
			_, isUnk := varVal.(types.Unknown)
			if found && !isUnk {
				return res.ResolveQualifiers(vars, varVal, attr.qualifiers)
			}
			typ, found := res.ResolveName(nm)
			if found {
				if len(attr.qualifiers) == 0 {
					return typ, nil
				}
				return nil, fmt.Errorf("no such attribute: %v", typ)
			}
			if isUnk {
				return types.Unknown{a.ID()}, nil
			}
		}
	}
	return nil, fmt.Errorf("no such attribute: %v", a)
}

func newQualifier(id int64, v interface{}) (Qualifier, error) {
	var qual Qualifier
	switch val := v.(type) {
	case Attribute:
		return val, nil
	case Qualifier:
		return val, nil
	case string:
		qual = &stringQualifier{id: id, Value: val, CelValue: types.String(val)}
	case int64:
		qual = &intQualifier{id: id, Value: val, CelValue: types.Int(val)}
	case uint64:
		qual = &uintQualifier{id: id, Value: val, CelValue: types.Uint(val)}
	case bool:
		qual = &boolQualifier{id: id, Value: val, CelValue: types.Bool(val)}
	case types.String:
		qual = &stringQualifier{id: id, Value: string(val), CelValue: val}
	case types.Int:
		qual = &intQualifier{id: id, Value: int64(val), CelValue: val}
	case types.Uint:
		qual = &uintQualifier{id: id, Value: uint64(val), CelValue: val}
	case types.Bool:
		qual = &boolQualifier{id: id, Value: bool(val), CelValue: val}
	default:
		return nil, fmt.Errorf("invalid qualifier type: %T", v)
	}
	return qual, nil
}

// Qualifier marker interface for designating different qualifier values and where they appear
// within expressions.
type Qualifier interface {
	// ID where the qualifier appears within an expression.
	ID() int64
}

type stringQualifier struct {
	id       int64
	Value    string
	CelValue ref.Val
}

// ID is an implementation of the Qualifier interface method.
func (q *stringQualifier) ID() int64 {
	return q.id
}

type intQualifier struct {
	id       int64
	Value    int64
	CelValue ref.Val
}

// ID is an implementation of the Qualifier interface method.
func (q *intQualifier) ID() int64 {
	return q.id
}

type uintQualifier struct {
	id       int64
	Value    uint64
	CelValue ref.Val
}

// ID is an implementation of the Qualifier interface method.
func (q *uintQualifier) ID() int64 {
	return q.id
}

type boolQualifier struct {
	id       int64
	Value    bool
	CelValue ref.Val
}

// ID is an implementation of the Qualifier interface method.
func (q *boolQualifier) ID() int64 {
	return q.id
}

// FieldQualifier indicates that the qualification is a well-defined field with a known
// field type. When the field type is known this can be used to improve the speed and
// efficiency of field resolution.
func FieldQualifier(id int64, name string, fieldType *ref.FieldType) Qualifier {
	return &fieldQualifier{
		id:        id,
		Name:      name,
		FieldType: fieldType,
	}
}

type fieldQualifier struct {
	id        int64
	Name      string
	FieldType *ref.FieldType
}

// ID is an implementation of the Qualifier interface method.
func (q *fieldQualifier) ID() int64 {
	return q.id
}
