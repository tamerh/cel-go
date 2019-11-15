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

type Attribute interface {
	ID() int64
	Qualify(id int64, v interface{}) (Attribute, error)
	Resolve(Activation, Resolver) (interface{}, error)
}

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

func (a *absoluteAttribute) ID() int64 {
	return a.id
}

func (a *absoluteAttribute) Qualify(id int64, v interface{}) (Attribute, error) {
	qual, err := newQualifier(id, v)
	if err != nil {
		return nil, err
	}
	a.qualifiers = append(a.qualifiers, qual)
	return a, nil
}

func (a *absoluteAttribute) Resolve(vars Activation, res Resolver) (interface{}, error) {
	for _, nm := range a.namespaceNames {
		if op, found := vars.FindName(nm); found {
			if len(a.qualifiers) == 0 {
				return op, nil
			}
			return res.ResolveQualifiers(vars, op, a.qualifiers)
		}
		if typ, found := res.FindName(nm); found {
			if len(a.qualifiers) == 0 {
				return typ, nil
			}
			return nil, fmt.Errorf("no such attribute: %v", typ)
		}
	}
	return nil, fmt.Errorf("no such attribute: %v", a)
}

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

func (a *relativeAttribute) ID() int64 {
	return a.id
}

func (a *relativeAttribute) Qualify(id int64, v interface{}) (Attribute, error) {
	qual, err := newQualifier(id, v)
	if err != nil {
		return nil, err
	}
	a.qualifiers = append(a.qualifiers, qual)
	return a, nil
}

func (a *relativeAttribute) QualifyDyn(attr Attribute) (Attribute, error) {
	a.qualifiers = append(a.qualifiers, attr)
	return a, nil
}

func (a *relativeAttribute) Resolve(vars Activation, res Resolver) (interface{}, error) {
	v := a.operand.Eval(vars)
	if types.IsError(v) {
		return nil, v.Value().(error)
	}
	return res.ResolveQualifiers(vars, v, a.qualifiers)
}

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

func (a *conditionalAttribute) ID() int64 {
	return a.id
}

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

func (a *conditionalAttribute) Resolve(vars Activation, res Resolver) (interface{}, error) {
	var v interface{}
	var err error
	attr, isAttr := a.expr.(instAttr)
	if isAttr {
		v, err = attr.Attr().Resolve(vars, res)
		if err != nil {
			return nil, err
		}
	} else {
		val := a.expr.Eval(vars)
		if types.IsError(val) {
			return nil, val.Value().(error)
		}
		v = val
	}
	if v == types.True || v == true {
		return a.truthy.Resolve(vars, res)
	}
	if v == types.False || v == false {
		return a.falsy.Resolve(vars, res)
	}
	return v, nil
}

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

func (a *oneofAttribute) ID() int64 {
	return a.id
}

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

func (a *oneofAttribute) Resolve(vars Activation, res Resolver) (interface{}, error) {
	for _, attr := range a.attrs {
		for _, nm := range attr.namespaceNames {
			varVal, found := vars.FindName(nm)
			if found {
				return res.ResolveQualifiers(vars, varVal, attr.qualifiers)
			}
			if typ, found := res.FindName(nm); found {
				if len(attr.qualifiers) == 0 {
					return typ, nil
				}
				return nil, fmt.Errorf("no such attribute: %v", typ)
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

type Qualifier interface {
	ID() int64
}

type stringQualifier struct {
	id       int64
	Value    string
	CelValue ref.Val
}

func (q *stringQualifier) ID() int64 {
	return q.id
}

type intQualifier struct {
	id       int64
	Value    int64
	CelValue ref.Val
}

func (q *intQualifier) ID() int64 {
	return q.id
}

type uintQualifier struct {
	id       int64
	Value    uint64
	CelValue ref.Val
}

func (q *uintQualifier) ID() int64 {
	return q.id
}

type boolQualifier struct {
	id       int64
	Value    bool
	CelValue ref.Val
}

func (q *boolQualifier) ID() int64 {
	return q.id
}

type attributePattern struct {
	variable   types.String
	qualifiers []ref.Val
}
