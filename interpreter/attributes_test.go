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
	res := NewResolver(reg, reg)
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
	attr := AbsoluteAttribute(1, pkgr.ResolveCandidateNames("a"))
	attr.Qualify(2, "b")
	attr.Qualify(3, uint64(4))
	attr.Qualify(4, false)
	out, err := attr.Resolve(vars, res)
	if err != nil {
		t.Fatal(err)
	}
	if out != "success" {
		t.Errorf("Got %v, wanted success", out)
	}
}

func TestAttributes_AbsoluteAttr_Type(t *testing.T) {
	reg := types.NewRegistry()
	res := NewResolver(reg, reg)
	attr := AbsoluteAttribute(1, []string{"int"})
	out, err := attr.Resolve(EmptyActivation(), res)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.IntType {
		t.Errorf("Got %v, wanted success", out)
	}
}

func TestAttributes_RelativeAttr(t *testing.T) {
	reg := types.NewRegistry()
	res := NewResolver(reg, reg)
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
	attr := RelativeAttribute(1, op)
	attr.Qualify(2, "a")
	attr.Qualify(3, int64(-1))
	attr.Qualify(4, AbsoluteAttribute(4, []string{"b"}))
	out, err := attr.Resolve(vars, res)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Int(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributes_OneofAttr(t *testing.T) {
	pkgr := packages.NewPackage("acme.ns")
	attr := OneofAttribute(1, pkgr.ResolveCandidateNames("a"))
	attr.Qualify(2, "b")

	reg := types.NewRegistry()
	res := NewResolver(reg, reg)
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": []int32{2, 42},
		},
		"acme.a.b":    1,
		"acme.ns.a.b": "found",
	}
	vars, _ := NewActivation(data)
	out, err := attr.Resolve(vars, res)
	if err != nil {
		t.Fatal(err)
	}
	if out != "found" {
		t.Errorf("Got %v, wanted 'found'", out)
	}
}

func TestAttributes_ConditionalAttr_TrueBranch(t *testing.T) {
	reg := types.NewRegistry()
	res := NewResolver(reg, reg)
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
	tv := AbsoluteAttribute(2, []string{"a"})
	fv := OneofAttribute(3, []string{"b"})
	fv.Qualify(4, "c")
	cond := ConditionalAttribute(1, &evalConst{val: types.True}, tv, fv)
	cond.Qualify(5, int64(-1))
	cond.Qualify(6, int64(1))
	out, err := cond.Resolve(vars, res)
	if err != nil {
		t.Fatal(err)
	}
	if out != int32(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributes_ConditionalAttr_FalseBranch(t *testing.T) {
	reg := types.NewRegistry()
	res := NewResolver(reg, reg)
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
	tv := AbsoluteAttribute(2, []string{"a"})
	fv := OneofAttribute(3, []string{"b"})
	fv.Qualify(4, "c")
	cond := ConditionalAttribute(1, &evalConst{val: types.False}, tv, fv)
	cond.Qualify(5, int64(-1))
	cond.Qualify(6, int64(1))
	out, err := cond.Resolve(vars, res)
	if err != nil {
		t.Fatal(err)
	}
	if out != uint(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
}

func TestAttributes_ConditionalAttr_ErrorUnknown(t *testing.T) {
	reg := types.NewRegistry()
	res := NewResolver(reg, reg)
	tv := AbsoluteAttribute(2, []string{"a"})
	fv := OneofAttribute(3, []string{"b"})
	cond := ConditionalAttribute(1, &evalConst{val: types.NewErr("test error")}, tv, fv)
	out, err := cond.Resolve(EmptyActivation(), res)
	if err == nil {
		t.Errorf("Got %v, wanted error", out)
	}

	condUnk := ConditionalAttribute(1, &evalConst{val: types.Unknown{1}}, tv, fv)
	out, err = condUnk.Resolve(EmptyActivation(), res)
	if err != nil {
		t.Fatal(err)
	}
	unk, ok := out.(types.Unknown)
	if !ok || !types.IsUnknown(unk) {
		t.Errorf("Got %v, wanted unknown", out)
	}
}

func BenchmarkAttributes_ResolveAttr(b *testing.B) {
	tc := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": []interface{}{"d", "e"},
			},
		},
	}
	vars, _ := NewActivation(map[string]interface{}{
		"tc": tc,
	})
	res := &resolver{}
	attr := AbsoluteAttribute(1, []string{"tc"})
	attr.Qualify(2, "a")
	attr.Qualify(3, "b")
	attr.Qualify(4, "c")
	attr.Qualify(5, int64(1))
	for n := 0; n < b.N; n++ {
		attr.Resolve(vars, res)
	}
}
