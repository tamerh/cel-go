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

	proto3pb "github.com/google/cel-go/test/proto3pb"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestAttributes_AbsoluteAttr(t *testing.T) {
	reg := types.NewRegistry()
	pkg := packages.NewPackage("acme.ns")
	attrs := NewAttributeFactory(pkg, reg, reg)
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
	attr := attrs.AbsoluteAttribute(1, "acme.a")
	qualB, _ := attrs.NewQualifier(nil, 2, "b")
	qual4, _ := attrs.NewQualifier(nil, 3, uint64(4))
	qualFalse, _ := attrs.NewQualifier(nil, 4, false)
	attr.AddQualifier(qualB)
	attr.AddQualifier(qual4)
	attr.AddQualifier(qualFalse)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.String("success") {
		t.Errorf("Got %v (%T), wanted success", out, out)
	}
}

func TestAttributes_AbsoluteAttr_Type(t *testing.T) {
	reg := types.NewRegistry()
	attrs := NewAttributeFactory(packages.DefaultPackage, reg, reg)

	// int
	attr := attrs.AbsoluteAttribute(1, "int")
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
	attrs := NewAttributeFactory(packages.DefaultPackage, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"b": 1,
	}
	vars, _ := NewActivation(data)

	// The relative attribute under test is applied to a map literal:
	// {
	//   a: {-1: [2, 42], b: 1}
	//   b: 1
	// }
	//
	// The expression being evaluated is: <map-literal>.a[-1][b] -> 42
	op := &evalConst{
		id:  1,
		val: reg.NativeToValue(data),
	}
	attr := attrs.RelativeAttribute(1, op)
	qualA, _ := attrs.NewQualifier(nil, 2, "a")
	qualNeg1, _ := attrs.NewQualifier(nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(attrs.AbsoluteAttribute(4, "b"))
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
	attrs := NewAttributeFactory(pkg, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"acme.b": 1,
	}
	vars, _ := NewActivation(data)

	// The relative attribute under test is applied to a map literal:
	// {
	//   a: {-1: [2, 42], b: 1}
	//   b: 1
	// }
	//
	// The expression being evaluated is: <map-literal>.a[-1][b] -> 42
	//
	// However, since the test is validating what happens with maybe attributes
	// the attribute resolution must also consider the following variations:
	// - <map-literal>.a[-1][acme.ns.b]
	// - <map-literal>.a[-1][acme.b]
	//
	// The correct behavior should yield the value of the last alternative.
	op := &evalConst{
		id:  1,
		val: reg.NativeToValue(data),
	}
	attr := attrs.RelativeAttribute(1, op)
	qualA, _ := attrs.NewQualifier(nil, 2, "a")
	qualNeg1, _ := attrs.NewQualifier(nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(attrs.MaybeAttribute(4, "b"))
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
	attrs := NewAttributeFactory(packages.DefaultPackage, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"b": []int{0, 1},
		"c": []interface{}{1, 0},
	}
	vars, _ := NewActivation(data)

	// The relative attribute under test is applied to a map literal:
	// {
	//   a: {-1: [2, 42], b: 1}
	//   b: [0, 1],
	//   c: {1, 0},
	// }
	//
	// The expression being evaluated is:
	// <map-literal>.a[-1][(false ? b : c)[0]] -> 42
	//
	// Effectively the same as saying <map-literal>.a[-1][c[0]]
	cond := &evalConst{
		id:  2,
		val: types.False,
	}
	condAttr := attrs.ConditionalAttribute(4, cond,
		attrs.AbsoluteAttribute(5, "b"),
		attrs.AbsoluteAttribute(6, "c"))
	qual0, _ := attrs.NewQualifier(nil, 7, 0)
	condAttr.AddQualifier(qual0)

	obj := &evalConst{
		id:  1,
		val: reg.NativeToValue(data),
	}
	attr := attrs.RelativeAttribute(1, obj)
	qualA, _ := attrs.NewQualifier(nil, 2, "a")
	qualNeg1, _ := attrs.NewQualifier(nil, 3, int64(-1))
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
	attrs := NewAttributeFactory(pkg, reg, reg)
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

	// The environment declares the following variables:
	// {
	//   a: {
	//     -1: {
	//       "first": 1u,
	//       "second": 2u,
	//       "third": 3u,
	//   b: 2u,
	// }
	//
	// The map of input variables is also re-used as a map-literal <obj> in the expression.
	//
	// The relative object under test is the following map literal.
	// <mp> {
	//   1u: "first",
	//   2u: "second",
	//   3u: "third",
	// }
	//
	// The expression under test is:
	//   <obj>.a[-1][<mp>[b]]
	//
	// This is equivalent to:
	//   <obj>.a[-1]["second"] -> 2u
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
	relAttr := attrs.RelativeAttribute(4, mp)
	qualB, _ := attrs.NewQualifier(nil, 5, attrs.AbsoluteAttribute(5, "b"))
	relAttr.AddQualifier(qualB)
	attr := attrs.RelativeAttribute(1, obj)
	qualA, _ := attrs.NewQualifier(nil, 2, "a")
	qualNeg1, _ := attrs.NewQualifier(nil, 3, int64(-1))
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
	attrs := NewAttributeFactory(pkg, reg, reg)
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": []int32{2, 42},
		},
		"acme.a.b":    1,
		"acme.ns.a.b": "found",
	}
	vars, _ := NewActivation(data)

	// a.b -> should resolve to acme.ns.a.b per namespace resolution rules.
	attr := attrs.MaybeAttribute(1, "a")
	qualB, _ := attrs.NewQualifier(nil, 2, "b")
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
	attrs := NewAttributeFactory(packages.DefaultPackage, reg, reg)
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
	tv := attrs.AbsoluteAttribute(2, "a")
	fv := attrs.MaybeAttribute(3, "b")
	qualC, _ := attrs.NewQualifier(nil, 4, "c")
	fv.AddQualifier(qualC)
	cond := attrs.ConditionalAttribute(1, &evalConst{val: types.True}, tv, fv)
	qualNeg1, _ := attrs.NewQualifier(nil, 5, int64(-1))
	qual1, _ := attrs.NewQualifier(nil, 6, int64(1))
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
	attrs := NewAttributeFactory(packages.DefaultPackage, reg, reg)
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
	tv := attrs.AbsoluteAttribute(2, "a")
	fv := attrs.MaybeAttribute(3, "b")
	qualC, _ := attrs.NewQualifier(nil, 4, "c")
	fv.AddQualifier(qualC)
	cond := attrs.ConditionalAttribute(1, &evalConst{val: types.False}, tv, fv)
	qualNeg1, _ := attrs.NewQualifier(nil, 5, int64(-1))
	qual1, _ := attrs.NewQualifier(nil, 6, int64(1))
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
	attrs := NewAttributeFactory(packages.DefaultPackage, reg, reg)

	// err ? a : b
	tv := attrs.AbsoluteAttribute(2, "a")
	fv := attrs.MaybeAttribute(3, "b")
	cond := attrs.ConditionalAttribute(1, &evalConst{val: types.NewErr("test error")}, tv, fv)
	out, err := cond.Resolve(EmptyActivation())
	if err == nil {
		t.Errorf("Got %v, wanted error", out)
	}

	// unk ? a : b
	condUnk := attrs.ConditionalAttribute(1, &evalConst{val: types.Unknown{1}}, tv, fv)
	out, err = condUnk.Resolve(EmptyActivation())
	if err != nil {
		t.Fatal(err)
	}
	unk, ok := out.(types.Unknown)
	if !ok || !types.IsUnknown(unk) {
		t.Errorf("Got %v, wanted unknown", out)
	}
}

func TestResolver_CustomQualifier(t *testing.T) {
	reg := types.NewRegistry()
	attrs := &custAttrFactory{
		AttributeFactory: NewAttributeFactory(packages.DefaultPackage, reg, reg),
	}
	msg := &proto3pb.TestAllTypes_NestedMessage{
		Bb: 123,
	}
	vars, _ := NewActivation(map[string]interface{}{
		"msg": msg,
	})
	attr := attrs.AbsoluteAttribute(1, "msg")
	qualBB, _ := attrs.NewQualifier(&exprpb.Type{
		TypeKind: &exprpb.Type_MessageType{
			MessageType: "google.expr.proto3.test.TestAllTypes.NestedMessage",
		},
	}, 2, "bb")
	attr.AddQualifier(qualBB)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Error(err)
	}
	if out != int32(123) {
		t.Errorf("Got %v, wanted 123", out)
	}
}

func BenchmarkResolver_CustomQualifier(b *testing.B) {
	reg := types.NewRegistry()
	attrs := &custAttrFactory{
		AttributeFactory: NewAttributeFactory(packages.DefaultPackage, reg, reg),
	}
	msg := &proto3pb.TestAllTypes_NestedMessage{
		Bb: 123,
	}
	vars, _ := NewActivation(map[string]interface{}{
		"msg": msg,
	})
	attr := attrs.AbsoluteAttribute(1, "msg")
	qualBB, _ := attrs.NewQualifier(&exprpb.Type{
		TypeKind: &exprpb.Type_MessageType{
			MessageType: "google.expr.proto3.test.TestAllTypes.NestedMessage",
		},
	}, 2, "bb")
	attr.AddQualifier(qualBB)
	for i := 0; i < b.N; i++ {
		attr.Resolve(vars)
	}
}

type custAttrFactory struct {
	AttributeFactory
}

func (r *custAttrFactory) NewQualifier(objType *exprpb.Type,
	qualID int64, val interface{}) (Qualifier, error) {
	if objType.GetMessageType() == "google.expr.proto3.test.TestAllTypes.NestedMessage" {
		return &nestedMsgQualifier{id: qualID, field: val.(string)}, nil
	}
	return r.AttributeFactory.NewQualifier(objType, qualID, val)
}

type nestedMsgQualifier struct {
	id    int64
	field string
}

func (q *nestedMsgQualifier) ID() int64 {
	return q.id
}

func (q *nestedMsgQualifier) Qualify(vars Activation, obj interface{}) (interface{}, error) {
	pb := obj.(*proto3pb.TestAllTypes_NestedMessage)
	return pb.GetBb(), nil
}
