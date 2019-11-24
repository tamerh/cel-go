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
	"testing"

	"github.com/google/cel-go/common/packages"
	"github.com/google/cel-go/common/types"
)

func TestAttributes_AbsoluteAttr(t *testing.T) {
	reg := types.NewRegistry()
	pkg := packages.NewPackage("acme.ns")
	res := NewResolver(pkg, reg, reg)
	vars, _ := NewActivation(map[string]interface{}{
		"acme.a": map[string]interface{}{
			"b": map[uint]interface{}{
				4: map[bool]string{
					false: "success",
				},
			},
		},
	})

	// acme.a.b[4][false]
	attr := res.AbsoluteAttribute(1, "acme.a")
	qualB, _ := res.NewQualifier(nil, 2, "b")
	qual4, _ := res.NewQualifier(nil, 3, uint64(4))
	qualFalse, _ := res.NewQualifier(nil, 4, false)
	attr.AddQualifier(qualB)
	attr.AddQualifier(qual4)
	attr.AddQualifier(qualFalse)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != "success" {
		t.Errorf("Got %v, wanted success", out)
	}
}

func TestAttributes_AbsoluteAttr_Type(t *testing.T) {
	reg := types.NewRegistry()
	res := NewResolver(packages.DefaultPackage, reg, reg)

	// int
	attr := res.AbsoluteAttribute(1, "int")
	out, err := attr.Resolve(EmptyActivation())
	if err != nil {
		t.Fatal(err)
	}
	if out != types.IntType {
		t.Errorf("Got %v, wanted success", out)
	}
}

func TestAttributes_RelativeAttr(t *testing.T) {
	reg := types.NewRegistry()
	res := NewResolver(packages.DefaultPackage, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"b": 1,
	}
	vars, _ := NewActivation(data)

	// obj.a[-1][b]
	op := &evalConst{
		id:  1,
		val: reg.NativeToValue(data),
	}
	attr := res.RelativeAttribute(1, op)
	qualA, _ := res.NewQualifier(nil, 2, "a")
	qualNeg1, _ := res.NewQualifier(nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(res.AbsoluteAttribute(4, "b"))
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Int(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributes_RelativeAttr_OneOf(t *testing.T) {
	reg := types.NewRegistry()
	pkg := packages.NewPackage("acme.ns")
	res := NewResolver(pkg, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"acme.b": 1,
	}
	vars, _ := NewActivation(data)

	// obj.a[-1][b]
	op := &evalConst{
		id:  1,
		val: reg.NativeToValue(data),
	}
	attr := res.RelativeAttribute(1, op)
	qualA, _ := res.NewQualifier(nil, 2, "a")
	qualNeg1, _ := res.NewQualifier(nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(res.OneofAttribute(4, "b"))
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Int(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributes_RelativeAttr_Conditional(t *testing.T) {
	reg := types.NewRegistry()
	res := NewResolver(packages.DefaultPackage, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"b": []int{0, 1},
		"c": []interface{}{1, 0},
	}
	vars, _ := NewActivation(data)

	// obj[a][-1][(cond ? b : c)[0]]
	cond := &evalConst{
		id:  2,
		val: types.False,
	}
	condAttr := res.ConditionalAttribute(4, cond,
		res.AbsoluteAttribute(5, "b"),
		res.AbsoluteAttribute(6, "c"))
	qual0, _ := res.NewQualifier(nil, 7, 0)
	condAttr.AddQualifier(qual0)

	obj := &evalConst{
		id:  1,
		val: reg.NativeToValue(data),
	}
	attr := res.RelativeAttribute(1, obj)
	qualA, _ := res.NewQualifier(nil, 2, "a")
	qualNeg1, _ := res.NewQualifier(nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(condAttr)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Int(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributes_RelativeAttr_Relative(t *testing.T) {
	pkg := packages.NewPackage("acme.ns")
	reg := types.NewRegistry()
	res := NewResolver(pkg, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: map[string]interface{}{
				"first":  uint(1),
				"second": uint(2),
				"third":  uint(3),
			},
		},
		"b": uint32(2),
	}
	vars, _ := NewActivation(data)

	// a[-1][{1u: "first", 2u: "second", 3u: "third"}[b]]
	obj := &evalConst{
		id:  1,
		val: reg.NativeToValue(data),
	}
	mp := &evalConst{
		id: 1,
		val: reg.NativeToValue(map[uint32]interface{}{
			1: "first",
			2: "second",
			3: "third",
		}),
	}
	relAttr := res.RelativeAttribute(4, mp)
	qualB, _ := res.NewQualifier(nil, 5, res.AbsoluteAttribute(5, "b"))
	relAttr.AddQualifier(qualB)
	attr := res.RelativeAttribute(1, obj)
	qualA, _ := res.NewQualifier(nil, 2, "a")
	qualNeg1, _ := res.NewQualifier(nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(relAttr)

	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Uint(2) {
		t.Errorf("Got %v (%T), wanted 2", out, out)
	}
}

func TestAttributes_OneofAttr(t *testing.T) {
	reg := types.NewRegistry()
	pkg := packages.NewPackage("acme.ns")
	res := NewResolver(pkg, reg, reg)
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": []int32{2, 42},
		},
		"acme.a.b":    1,
		"acme.ns.a.b": "found",
	}
	vars, _ := NewActivation(data)

	// a.b -> should resolve to acme.ns.a.b per namespace resolution rules.
	attr := res.OneofAttribute(1, "a")
	qualB, _ := res.NewQualifier(nil, 2, "b")
	attr.AddQualifier(qualB)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != "found" {
		t.Errorf("Got %v, wanted 'found'", out)
	}
}

func TestAttributes_ConditionalAttr_TrueBranch(t *testing.T) {
	reg := types.NewRegistry()
	res := NewResolver(packages.DefaultPackage, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"b": map[string]interface{}{
			"c": map[int32]interface{}{
				-1: []uint{2, 42},
			},
		},
	}
	vars, _ := NewActivation(data)

	// (true ? a : b.c)[-1][1]
	tv := res.AbsoluteAttribute(2, "a")
	fv := res.OneofAttribute(3, "b")
	qualC, _ := res.NewQualifier(nil, 4, "c")
	fv.AddQualifier(qualC)
	cond := res.ConditionalAttribute(1, &evalConst{val: types.True}, tv, fv)
	qualNeg1, _ := res.NewQualifier(nil, 5, int64(-1))
	qual1, _ := res.NewQualifier(nil, 6, int64(1))
	cond.AddQualifier(qualNeg1)
	cond.AddQualifier(qual1)
	out, err := cond.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != int32(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributes_ConditionalAttr_FalseBranch(t *testing.T) {
	reg := types.NewRegistry()
	res := NewResolver(packages.DefaultPackage, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"b": map[string]interface{}{
			"c": map[int32]interface{}{
				-1: []uint{2, 42},
			},
		},
	}
	vars, _ := NewActivation(data)

	// (false ? a : b.c)[-1][1]
	tv := res.AbsoluteAttribute(2, "a")
	fv := res.OneofAttribute(3, "b")
	qualC, _ := res.NewQualifier(nil, 4, "c")
	fv.AddQualifier(qualC)
	cond := res.ConditionalAttribute(1, &evalConst{val: types.False}, tv, fv)
	qualNeg1, _ := res.NewQualifier(nil, 5, int64(-1))
	qual1, _ := res.NewQualifier(nil, 6, int64(1))
	cond.AddQualifier(qualNeg1)
	cond.AddQualifier(qual1)
	out, err := cond.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != uint(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributes_ConditionalAttr_ErrorUnknown(t *testing.T) {
	reg := types.NewRegistry()
	res := NewResolver(packages.DefaultPackage, reg, reg)

	// err ? a : b
	tv := res.AbsoluteAttribute(2, "a")
	fv := res.OneofAttribute(3, "b")
	cond := res.ConditionalAttribute(1, &evalConst{val: types.NewErr("test error")}, tv, fv)
	out, err := cond.Resolve(EmptyActivation())
	if err == nil {
		t.Errorf("Got %v, wanted error", out)
	}

	// unk ? a : b
	condUnk := res.ConditionalAttribute(1, &evalConst{val: types.Unknown{1}}, tv, fv)
	out, err = condUnk.Resolve(EmptyActivation())
	if err != nil {
		t.Fatal(err)
	}
	unk, ok := out.(types.Unknown)
	if !ok || !types.IsUnknown(unk) {
		t.Errorf("Got %v, wanted unknown", out)
	}
}
