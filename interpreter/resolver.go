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

type Resolver interface {
	FindName(name string) (interface{}, bool)

	ResolveQualifiers(Activation, interface{}, []Qualifier) (interface{}, error)
}

type resolver struct {
	adapter  ref.TypeAdapter
	provider ref.TypeProvider
}

func (res *resolver) FindName(name string) (interface{}, bool) {
	return res.provider.FindIdent(name)
}

func (res *resolver) ResolveQualifiers(vars Activation,
	obj interface{},
	quals []Qualifier) (interface{}, error) {
	var s string
	var i int64
	var u uint64
	var b bool
	for idx := 0; idx < len(quals); idx++ {
		isMap := false
		isKey := false
		isIndex := false
		switch qual := quals[idx].(type) {
		case *stringQualifier:
			s = qual.Value
			goto QualString
		case *intQualifier:
			i = qual.Value
			goto QualInt
		case *uintQualifier:
			u = qual.Value
			goto QualUint
		case *boolQualifier:
			b = qual.Value
			goto QualBool
		case Attribute:
			v, err := qual.Resolve(vars, res)
			if err != nil {
				return nil, err
			}
			// TODO: add ref.Val support
			switch q := v.(type) {
			case string:
				s = q
				goto QualString
			case int64:
				i = q
				goto QualInt
			case uint64:
				u = q
				goto QualUint
			case bool:
				b = q
				goto QualBool
			default:
				return nil, fmt.Errorf("unsupported attribute type: %T", q)
			}
		}

	QualString:
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
		case map[string]bool:
			isMap = true
			obj, isKey = o[s]
		default:
			elem, err := res.refResolve(types.String(s), obj)
			if err != nil {
				return nil, err
			}
			if types.IsUnknown(elem) {
				return elem, nil
			}
			obj = elem
			isMap = true
			isKey = true
		}
		if isMap && !isKey {
			return nil, fmt.Errorf("no such key: %v", s)
		}
		continue

	QualInt:
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
		case []uint:
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
		default:
			elem, err := res.refResolve(types.Int(i), obj)
			if err != nil {
				return nil, err
			}
			if types.IsUnknown(elem) {
				return elem, nil
			}
			isMap = true
			isKey = true
			obj = elem
		}
		if isMap && !isKey {
			return nil, fmt.Errorf("no such key: %v", i)
		}
		if !isMap && !isIndex {
			return nil, fmt.Errorf("index out of bounds: %v", i)
		}
		continue

	QualUint:
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
		case []interface{}:
			isIndex = u >= 0 && u < uint64(len(o))
			if isIndex {
				obj = o[u]
			}
		case []string:
			isIndex = u >= 0 && u < uint64(len(o))
			if isIndex {
				obj = o[u]
			}
		case []int:
			isIndex = u >= 0 && u < uint64(len(o))
			if isIndex {
				obj = o[u]
			}
		case []uint:
			isIndex = u >= 0 && u < uint64(len(o))
			if isIndex {
				obj = o[u]
			}
		case []float32:
			isIndex = u >= 0 && u < uint64(len(o))
			if isIndex {
				obj = o[u]
			}
		case []float64:
			isIndex = u >= 0 && u < uint64(len(o))
			if isIndex {
				obj = o[u]
			}
		case []bool:
			isIndex = u >= 0 && u < uint64(len(o))
			if isIndex {
				obj = o[u]
			}
		default:
			elem, err := res.refResolve(types.Uint(u), obj)
			if err != nil {
				return nil, err
			}
			if types.IsUnknown(elem) {
				return elem, nil
			}
			isMap = true
			isKey = true
			obj = elem
		}
		if isMap && !isKey {
			return nil, fmt.Errorf("no such key: %v", i)
		}
		if !isMap && !isIndex {
			return nil, fmt.Errorf("index out of bounds: %v", i)
		}
		continue

	QualBool:
		switch o := obj.(type) {
		case map[bool]interface{}:
			obj, isKey = o[b]
		case map[bool]string:
			obj, isKey = o[b]
		case map[bool]int:
			obj, isKey = o[b]
		default:
			elem, err := res.refResolve(types.Bool(b), obj)
			if err != nil {
				return nil, err
			}
			if types.IsUnknown(elem) {
				return elem, nil
			}
			isMap = true
			isKey = true
			obj = elem
		}
		if !isKey {
			return nil, fmt.Errorf("no such key: %v", b)
		}
		continue
	}
	return obj, nil
}

func (res *resolver) refResolve(idx ref.Val, obj interface{}) (ref.Val, error) {
	celVal := res.adapter.NativeToValue(obj)
	indexer, isIndexer := celVal.(traits.Indexer)
	if !isIndexer {
		return nil, errors.New("no such overload")
	}
	elem := indexer.Get(idx)
	if types.IsError(elem) {
		return nil, elem.Value().(error)
	}
	return elem, nil
}
