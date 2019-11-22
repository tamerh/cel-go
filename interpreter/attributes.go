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
	"errors"
	"fmt"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// Attribute values are a variable or value with an optional set of qualifiers, such as field, key,
// or index accesses.
type Attribute interface {
	Qualifier

	// Qualify adds an additional qualifier on the Attribute or error if the qualification is not
	// a supported proto map key type.
	AddQualifier(id int64, v interface{}) (Attribute, error)
}

// AbsoluteAttribute refers to a variable value and an optional qualifier path.
//
// The namespaceNames represent the names the variable could have based on namespace
// resolution rules.
func AbsoluteAttribute(adapter ref.TypeAdapter,
	provider ref.TypeProvider,
	id int64,
	namespacedNames []string) Attribute {
	return &absoluteAttribute{
		id:             id,
		namespaceNames: namespacedNames,
		qualifiers:     []Qualifier{},
		adapter:        adapter,
		provider:       provider,
	}
}

type absoluteAttribute struct {
	id             int64
	namespaceNames []string
	qualifiers     []Qualifier
	adapter        ref.TypeAdapter
	provider       ref.TypeProvider
}

// ID implements the Attribute interface method.
func (a *absoluteAttribute) ID() int64 {
	return a.id
}

// Qualify implements the Attribute interface method.
func (a *absoluteAttribute) AddQualifier(id int64, v interface{}) (Attribute, error) {
	qual, err := newQualifier(a.adapter, id, v)
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
func (a *absoluteAttribute) Qualify(vars Activation, obj interface{}) (interface{}, error) {
	for _, nm := range a.namespaceNames {
		op, found := vars.ResolveName(nm)
		_, isUnk := op.(types.Unknown)
		if found && !isUnk {
			var err error
			for _, qual := range a.qualifiers {
				op, err = qual.Qualify(vars, op)
				if err != nil {
					return nil, err
				}
			}
			return op, nil
		}
		// Attempt to resolve the qualified type name if the name is not a variable identifier.
		typ, found := a.provider.FindIdent(nm)
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
func RelativeAttribute(adapter ref.TypeAdapter,
	id int64, operand Interpretable) Attribute {
	return &relativeAttribute{
		id:         id,
		operand:    operand,
		qualifiers: []Qualifier{},
		adapter:    adapter,
	}
}

type relativeAttribute struct {
	id         int64
	operand    Interpretable
	qualifiers []Qualifier
	adapter    ref.TypeAdapter
}

// ID is an implementation of the Attribute interface method.
func (a *relativeAttribute) ID() int64 {
	return a.id
}

// Qualify implements the Attribute interface method.
func (a *relativeAttribute) AddQualifier(id int64, v interface{}) (Attribute, error) {
	qual, err := newQualifier(a.adapter, id, v)
	if err != nil {
		return nil, err
	}
	a.qualifiers = append(a.qualifiers, qual)
	return a, nil
}

// Resolve expression value and qualifier relative to the expression result.
func (a *relativeAttribute) Qualify(vars Activation, obj interface{}) (interface{}, error) {
	v := a.operand.Eval(vars)
	if types.IsError(v) {
		return nil, v.Value().(error)
	}
	if types.IsUnknown(v) {
		return v, nil
	}
	var err error
	obj = v
	for _, qual := range a.qualifiers {
		obj, err = qual.Qualify(vars, obj)
		if err != nil {
			return nil, err
		}
	}
	return obj, nil
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
func (a *conditionalAttribute) AddQualifier(id int64, v interface{}) (Attribute, error) {
	_, err := a.truthy.AddQualifier(id, v)
	if err != nil {
		return nil, err
	}
	_, err = a.falsy.AddQualifier(id, v)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Resolve evaluates the condition, and then resolves the truthy or falsy branch accordingly.
func (a *conditionalAttribute) Qualify(vars Activation, obj interface{}) (interface{}, error) {
	val := a.expr.Eval(vars)
	if types.IsError(val) {
		return nil, val.Value().(error)
	}
	if val == types.True {
		return a.truthy.Qualify(vars, obj)
	}
	if val == types.False {
		return a.falsy.Qualify(vars, obj)
	}
	if types.IsUnknown(val) {
		return val, nil
	}
	return nil, types.ValOrErr(val, "no such overload").Value().(error)
}

// OneofAttribute collects variants of unchecked AbsoluteAttribute values which could either be
// direct variable accesses or some combination of variable access with qualification.
func OneofAttribute(adapter ref.TypeAdapter,
	provider ref.TypeProvider,
	id int64,
	namespacedNames []string) Attribute {
	return &oneofAttribute{
		id: id,
		attrs: []*absoluteAttribute{
			&absoluteAttribute{
				id:             id,
				namespaceNames: namespacedNames,
				qualifiers:     []Qualifier{},
				provider:       provider,
				adapter:        adapter,
			},
		},
		adapter:  adapter,
		provider: provider,
	}
}

type oneofAttribute struct {
	id       int64
	attrs    []*absoluteAttribute
	adapter  ref.TypeAdapter
	provider ref.TypeProvider
}

// ID is an implementation of the Attribute interface method.
func (a *oneofAttribute) ID() int64 {
	return a.id
}

// Qualify adds a qualifier to each possible attribute variant in the oneof, and also creates a new
// namespaced variable from the qualified value.
func (a *oneofAttribute) AddQualifier(id int64, v interface{}) (Attribute, error) {
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
		attr.AddQualifier(id, v)
	}
	a.attrs = append([]*absoluteAttribute{
		&absoluteAttribute{
			id:             id,
			namespaceNames: augmentedNames,
			qualifiers:     []Qualifier{},
			adapter:        a.adapter,
			provider:       a.provider,
		},
	}, a.attrs...)
	return a, nil
}

// Resolve follows the variable resolution
func (a *oneofAttribute) Qualify(vars Activation, obj interface{}) (interface{}, error) {
	for _, attr := range a.attrs {
		for _, nm := range attr.namespaceNames {
			op, found := vars.ResolveName(nm)
			_, isUnk := op.(types.Unknown)
			if found && !isUnk {
				var err error
				for _, qual := range attr.qualifiers {
					op, err = qual.Qualify(vars, op)
					if err != nil {
						return nil, err
					}
				}
				return op, nil
			}
			typ, found := a.provider.FindIdent(nm)
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

func newQualifier(adapter ref.TypeAdapter, id int64, v interface{}) (Qualifier, error) {
	var qual Qualifier
	switch val := v.(type) {
	case Attribute:
		return val, nil
	case Qualifier:
		return val, nil
	case string:
		qual = &stringQualifier{id: id, Value: val, CelValue: types.String(val), adapter: adapter}
	case int64:
		qual = &intQualifier{id: id, Value: val, CelValue: types.Int(val), adapter: adapter}
	case uint64:
		qual = &uintQualifier{id: id, Value: val, CelValue: types.Uint(val), adapter: adapter}
	case bool:
		qual = &boolQualifier{id: id, Value: val, CelValue: types.Bool(val), adapter: adapter}
	case types.String:
		qual = &stringQualifier{id: id, Value: string(val), CelValue: val, adapter: adapter}
	case types.Int:
		qual = &intQualifier{id: id, Value: int64(val), CelValue: val, adapter: adapter}
	case types.Uint:
		qual = &uintQualifier{id: id, Value: uint64(val), CelValue: val, adapter: adapter}
	case types.Bool:
		qual = &boolQualifier{id: id, Value: bool(val), CelValue: val, adapter: adapter}
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

	Qualify(vars Activation, obj interface{}) (interface{}, error)
}

type stringQualifier struct {
	id       int64
	Value    string
	CelValue ref.Val
	adapter  ref.TypeAdapter
}

// ID is an implementation of the Qualifier interface method.
func (q *stringQualifier) ID() int64 {
	return q.id
}

func (q *stringQualifier) Qualify(vars Activation, obj interface{}) (interface{}, error) {
	s := q.Value
	isMap := false
	isKey := false
	switch o := obj.(type) {
	case map[string]interface{}:
		isMap = true
		obj, isKey = o[s]
	case map[string]string:
		isMap = true
		obj, isKey = o[s]
	case map[string]int:
		isMap = true
		obj, isKey = o[s]
	case map[string]int32:
		isMap = true
		obj, isKey = o[s]
	case map[string]int64:
		isMap = true
		obj, isKey = o[s]
	case map[string]uint:
		isMap = true
		obj, isKey = o[s]
	case map[string]uint32:
		isMap = true
		obj, isKey = o[s]
	case map[string]uint64:
		isMap = true
		obj, isKey = o[s]
	case map[string]float32:
		isMap = true
		obj, isKey = o[s]
	case map[string]float64:
		isMap = true
		obj, isKey = o[s]
	case map[string]bool:
		isMap = true
		obj, isKey = o[s]
	case types.Unknown:
		return o, nil
	default:
		elem, err := refResolve(q.adapter, q.CelValue, obj)
		if err != nil {
			return nil, err
		}
		if types.IsUnknown(elem) {
			return fmtUnknown(elem, q), nil
		}
		return elem, nil
	}
	if isMap && !isKey {
		return nil, fmt.Errorf("no such key: %v", s)
	}
	return obj, nil
}

type intQualifier struct {
	id       int64
	Value    int64
	CelValue ref.Val
	adapter  ref.TypeAdapter
}

// ID is an implementation of the Qualifier interface method.
func (q *intQualifier) ID() int64 {
	return q.id
}

func (q *intQualifier) Qualify(vars Activation, obj interface{}) (interface{}, error) {
	i := q.Value
	isMap := false
	isKey := false
	isIndex := false
	switch o := obj.(type) {
	case map[int]interface{}:
		isMap = true
		obj, isKey = o[int(i)]
	case map[int32]interface{}:
		isMap = true
		obj, isKey = o[int32(i)]
	case map[int64]interface{}:
		isMap = true
		obj, isKey = o[i]
	case []interface{}:
		isIndex = i >= 0 && i < int64(len(o))
		if isIndex {
			obj = o[i]
		}
	case []string:
		isIndex = i >= 0 && i < int64(len(o))
		if isIndex {
			obj = o[i]
		}
	case []int:
		isIndex = i >= 0 && i < int64(len(o))
		if isIndex {
			obj = o[i]
		}
	case []int32:
		isIndex = i >= 0 && i < int64(len(o))
		if isIndex {
			obj = o[i]
		}
	case []int64:
		isIndex = i >= 0 && i < int64(len(o))
		if isIndex {
			obj = o[i]
		}
	case []uint:
		isIndex = i >= 0 && i < int64(len(o))
		if isIndex {
			obj = o[i]
		}
	case []uint32:
		isIndex = i >= 0 && i < int64(len(o))
		if isIndex {
			obj = o[i]
		}
	case []uint64:
		isIndex = i >= 0 && i < int64(len(o))
		if isIndex {
			obj = o[i]
		}
	case []float32:
		isIndex = i >= 0 && i < int64(len(o))
		if isIndex {
			obj = o[i]
		}
	case []float64:
		isIndex = i >= 0 && i < int64(len(o))
		if isIndex {
			obj = o[i]
		}
	case []bool:
		isIndex = i >= 0 && i < int64(len(o))
		if isIndex {
			obj = o[i]
		}
	case types.Unknown:
		return o, nil
	default:
		elem, err := refResolve(q.adapter, q.CelValue, obj)
		if err != nil {
			return nil, err
		}
		if types.IsUnknown(elem) {
			return fmtUnknown(elem, q), nil
		}
		return elem, nil
	}
	if isMap && !isKey {
		return nil, fmt.Errorf("no such key: %v", i)
	}
	if !isMap && !isIndex {
		return nil, fmt.Errorf("index out of bounds: %v", i)
	}
	return obj, nil
}

type uintQualifier struct {
	id       int64
	Value    uint64
	CelValue ref.Val
	adapter  ref.TypeAdapter
}

// ID is an implementation of the Qualifier interface method.
func (q *uintQualifier) ID() int64 {
	return q.id
}

func (q *uintQualifier) Qualify(vars Activation, obj interface{}) (interface{}, error) {
	u := q.Value
	isMap := false
	isKey := false
	switch o := obj.(type) {
	case map[uint]interface{}:
		isMap = true
		obj, isKey = o[uint(u)]
	case map[uint32]interface{}:
		isMap = true
		obj, isKey = o[uint32(u)]
	case map[uint64]interface{}:
		isMap = true
		obj, isKey = o[u]
	case types.Unknown:
		return o, nil
	default:
		elem, err := refResolve(q.adapter, q.CelValue, obj)
		if err != nil {
			return nil, err
		}
		if types.IsUnknown(elem) {
			return fmtUnknown(elem, q), nil
		}
		return elem, nil
	}
	if isMap && !isKey {
		return nil, fmt.Errorf("no such key: %v", u)
	}
	return obj, nil
}

type boolQualifier struct {
	id       int64
	Value    bool
	CelValue ref.Val
	adapter  ref.TypeAdapter
}

// ID is an implementation of the Qualifier interface method.
func (q *boolQualifier) ID() int64 {
	return q.id
}

func (q *boolQualifier) Qualify(vars Activation, obj interface{}) (interface{}, error) {
	b := q.Value
	isKey := false
	switch o := obj.(type) {
	case map[bool]interface{}:
		obj, isKey = o[b]
	case map[bool]string:
		obj, isKey = o[b]
	case map[bool]int:
		obj, isKey = o[b]
	case map[bool]int32:
		obj, isKey = o[b]
	case map[bool]int64:
		obj, isKey = o[b]
	case map[bool]uint:
		obj, isKey = o[b]
	case map[bool]uint32:
		obj, isKey = o[b]
	case map[bool]uint64:
		obj, isKey = o[b]
	case map[bool]float32:
		obj, isKey = o[b]
	case map[bool]float64:
		obj, isKey = o[b]
	case types.Unknown:
		return o, nil
	default:
		elem, err := refResolve(q.adapter, q.CelValue, obj)
		if err != nil {
			return nil, err
		}
		if types.IsUnknown(elem) {
			return fmtUnknown(elem, q), nil
		}
		return elem, nil
	}
	if !isKey {
		return nil, fmt.Errorf("no such key: %v", b)
	}
	return obj, nil
}

// FieldQualifier indicates that the qualification is a well-defined field with a known
// field type. When the field type is known this can be used to improve the speed and
// efficiency of field resolution.
func FieldQualifier(adapter ref.TypeAdapter,
	id int64, name string, fieldType *ref.FieldType) Qualifier {
	return &fieldQualifier{
		id:        id,
		Name:      name,
		FieldType: fieldType,
		adapter:   adapter,
	}
}

type fieldQualifier struct {
	id        int64
	Name      string
	FieldType *ref.FieldType
	adapter   ref.TypeAdapter
}

// ID is an implementation of the Qualifier interface method.
func (q *fieldQualifier) ID() int64 {
	return q.id
}

func (q *fieldQualifier) Qualify(vars Activation, obj interface{}) (interface{}, error) {
	if rv, ok := obj.(ref.Val); ok {
		obj = rv.Value()
	}
	v, err := q.FieldType.GetFrom(obj)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func fmtUnknown(elem ref.Val, qual Qualifier) ref.Val {
	unk := elem.(types.Unknown)
	if len(unk) == 0 {
		return types.Unknown{qual.ID()}
	}
	return unk
}

func refResolve(adapter ref.TypeAdapter, idx ref.Val, obj interface{}) (ref.Val, error) {
	celVal := adapter.NativeToValue(obj)
	mapper, isMapper := celVal.(traits.Mapper)
	if isMapper {
		elem, found := mapper.Find(idx)
		if !found {
			return nil, fmt.Errorf("no such key: %v", idx)
		}
		if types.IsError(elem) {
			return nil, elem.Value().(error)
		}
		return elem, nil
	}
	indexer, isIndexer := celVal.(traits.Indexer)
	if isIndexer {
		elem := indexer.Get(idx)
		if types.IsError(elem) {
			return nil, elem.Value().(error)
		}
		return elem, nil
	}
	return nil, errors.New("no such overload")
}
