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
	vars, _ := NewActivation(map[string]interface{}{
		"acme.a": map[string]interface{}{
			"b": map[uint]interface{}{
				4: map[bool]string{
					false: "success",
				},
			},
		},
	})

	pkgr := packages.NewPackage("acme.ns")
	attr := AbsoluteAttribute(reg, reg, 1, pkgr.ResolveCandidateNames("a"))
	attr.AddQualifier(2, "b")
	attr.AddQualifier(3, uint64(4))
	attr.AddQualifier(4, false)
	out, err := attr.Qualify(vars, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "success" {
		t.Errorf("Got %v, wanted success", out)
	}
}

func TestAttributes_AbsoluteAttr_Type(t *testing.T) {
	reg := types.NewRegistry()
	attr := AbsoluteAttribute(reg, reg, 1, []string{"int"})
	out, err := attr.Qualify(EmptyActivation(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.IntType {
		t.Errorf("Got %v, wanted success", out)
	}
}

func TestAttributes_RelativeAttr(t *testing.T) {
	reg := types.NewRegistry()
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"b": 1,
	}
	vars, _ := NewActivation(data)

	op := &evalConst{
		id:  1,
		val: reg.NativeToValue(data),
	}
	attr := RelativeAttribute(reg, 1, op)
	attr.AddQualifier(2, "a")
	attr.AddQualifier(3, int64(-1))
	attr.AddQualifier(4, AbsoluteAttribute(reg, reg, 4, []string{"b"}))
	out, err := attr.Qualify(vars, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Int(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributes_OneofAttr(t *testing.T) {
	reg := types.NewRegistry()
	pkgr := packages.NewPackage("acme.ns")
	attr := OneofAttribute(reg, reg, 1, pkgr.ResolveCandidateNames("a"))
	attr.AddQualifier(2, "b")

	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": []int32{2, 42},
		},
		"acme.a.b":    1,
		"acme.ns.a.b": "found",
	}
	vars, _ := NewActivation(data)
	out, err := attr.Qualify(vars, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "found" {
		t.Errorf("Got %v, wanted 'found'", out)
	}
}

func TestAttributes_ConditionalAttr_TrueBranch(t *testing.T) {
	reg := types.NewRegistry()
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
	tv := AbsoluteAttribute(reg, reg, 2, []string{"a"})
	fv := OneofAttribute(reg, reg, 3, []string{"b"})
	fv.AddQualifier(4, "c")
	cond := ConditionalAttribute(1, &evalConst{val: types.True}, tv, fv)
	cond.AddQualifier(5, int64(-1))
	cond.AddQualifier(6, int64(1))
	out, err := cond.Qualify(vars, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != int32(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributes_ConditionalAttr_FalseBranch(t *testing.T) {
	reg := types.NewRegistry()
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
	tv := AbsoluteAttribute(reg, reg, 2, []string{"a"})
	fv := OneofAttribute(reg, reg, 3, []string{"b"})
	fv.AddQualifier(4, "c")
	cond := ConditionalAttribute(1, &evalConst{val: types.False}, tv, fv)
	cond.AddQualifier(5, int64(-1))
	cond.AddQualifier(6, int64(1))
	out, err := cond.Qualify(vars, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != uint(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributes_ConditionalAttr_ErrorUnknown(t *testing.T) {
	reg := types.NewRegistry()
	tv := AbsoluteAttribute(reg, reg, 2, []string{"a"})
	fv := OneofAttribute(reg, reg, 3, []string{"b"})
	cond := ConditionalAttribute(1, &evalConst{val: types.NewErr("test error")}, tv, fv)
	out, err := cond.Qualify(EmptyActivation(), nil)
	if err == nil {
		t.Errorf("Got %v, wanted error", out)
	}

	condUnk := ConditionalAttribute(1, &evalConst{val: types.Unknown{1}}, tv, fv)
	out, err = condUnk.Qualify(EmptyActivation(), nil)
	if err != nil {
		t.Fatal(err)
	}
	unk, ok := out.(types.Unknown)
	if !ok || !types.IsUnknown(unk) {
		t.Errorf("Got %v, wanted unknown", out)
	}
}
