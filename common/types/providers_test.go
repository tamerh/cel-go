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

package types

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"

	"github.com/tamerh/cel-go/common/types/ref"
	"github.com/tamerh/cel-go/common/types/traits"

	structpb "github.com/golang/protobuf/ptypes/struct"
	wrapperspb "github.com/golang/protobuf/ptypes/wrappers"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestTypeRegistry_NewValue(t *testing.T) {
	reg := NewRegistry(&exprpb.ParsedExpr{})
	sourceInfo := reg.NewValue(
		"google.api.expr.v1alpha1.SourceInfo",
		map[string]ref.Val{
			"location":     String("TestTypeRegistry_NewValue"),
			"line_offsets": NewDynamicList(reg, []int64{0, 2}),
			"positions":    NewDynamicMap(reg, map[int64]int64{1: 2, 2: 4}),
		})
	if IsError(sourceInfo) {
		t.Error(sourceInfo)
	} else {
		info := sourceInfo.Value().(*exprpb.SourceInfo)
		if info.Location != "TestTypeRegistry_NewValue" ||
			!reflect.DeepEqual(info.LineOffsets, []int32{0, 2}) ||
			!reflect.DeepEqual(info.Positions, map[int64]int32{1: 2, 2: 4}) {
			t.Errorf("Source info not properly configured: %v", info)
		}
	}
}

func TestTypeRegistry_NewValue_OneofFields(t *testing.T) {
	reg := NewRegistry(&exprpb.ParsedExpr{})
	if exp := reg.NewValue(
		"google.api.expr.v1alpha1.Expr",
		map[string]ref.Val{
			"const_expr": reg.NativeToValue(
				&exprpb.Constant{
					ConstantKind: &exprpb.Constant_StringValue{
						StringValue: "oneof"}}),
		}); IsError(exp) {
		t.Error(exp)
	} else {
		e := exp.Value().(*exprpb.Expr)
		if e.GetConstExpr().GetStringValue() != "oneof" {
			t.Errorf("Expr with oneof could not be created: %v", e)
		}
	}
}

func TestTypeRegistry_Getters(t *testing.T) {
	reg := NewRegistry(&exprpb.ParsedExpr{})
	if sourceInfo := reg.NewValue(
		"google.api.expr.v1alpha1.SourceInfo",
		map[string]ref.Val{
			"location":     String("TestTypeRegistry_GetFieldValue"),
			"line_offsets": NewDynamicList(reg, []int64{0, 2}),
			"positions":    NewDynamicMap(reg, map[int64]int64{1: 2, 2: 4}),
		}); IsError(sourceInfo) {
		t.Error(sourceInfo)
	} else {
		si := sourceInfo.(traits.Indexer)
		if loc := si.Get(String("location")); IsError(loc) {
			t.Error(loc)
		} else if loc.(String) != "TestTypeRegistry_GetFieldValue" {
			t.Errorf("Expected %s, got %s",
				"TestTypeRegistry_GetFieldValue",
				loc)
		}
		if pos := si.Get(String("positions")); IsError(pos) {
			t.Error(pos)
		} else if pos.Equal(NewDynamicMap(reg, map[int64]int32{1: 2, 2: 4})) != True {
			t.Errorf("Expected map[int64]int32, got %v", pos)
		} else if posKeyVal := pos.(traits.Indexer).Get(Int(1)); IsError(posKeyVal) {
			t.Error(posKeyVal)
		} else if posKeyVal.(Int) != 2 {
			t.Error("Expected value to be int64, not int32")
		}
		if offsets := si.Get(String("line_offsets")); IsError(offsets) {
			t.Error(offsets)
		} else if offset1 := offsets.(traits.Lister).Get(Int(1)); IsError(offset1) {
			t.Error(offset1)
		} else if offset1.(Int) != 2 {
			t.Errorf("Expected index 1 to be value 2, was %v", offset1)
		}
	}
}

func TestValue_ConvertToNative(t *testing.T) {
	reg := NewRegistry(&exprpb.ParsedExpr{})

	// Core type conversion tests.
	expectValueToNative(t, True, true)
	expectValueToNative(t, Int(-1), int32(-1))
	expectValueToNative(t, Int(2), int64(2))
	expectValueToNative(t, Uint(3), uint32(3))
	expectValueToNative(t, Uint(4), uint64(4))
	expectValueToNative(t, Double(5.5), float32(5.5))
	expectValueToNative(t, Double(-5.5), float64(-5.5))
	expectValueToNative(t, String("hello"), "hello")
	expectValueToNative(t, Bytes("world"), []byte("world"))
	expectValueToNative(t, NewDynamicList(reg,
		[]int64{1, 2, 3}), []int32{1, 2, 3})
	expectValueToNative(t, NewDynamicMap(reg,
		map[int64]int64{1: 1, 2: 1, 3: 1}),
		map[int32]int32{1: 1, 2: 1, 3: 1})

	// Null conversion tests.
	expectValueToNative(t, Null(structpb.NullValue_NULL_VALUE), structpb.NullValue_NULL_VALUE)

	// Proto conversion tests.
	parsedExpr := &exprpb.ParsedExpr{}
	expectValueToNative(t, reg.NativeToValue(parsedExpr), parsedExpr)
}

func TestNativeToValue_Any(t *testing.T) {
	reg := NewRegistry(&exprpb.ParsedExpr{})
	// NullValue
	anyValue, err := NullValue.ConvertToNative(anyValueType)
	if err != nil {
		t.Error(err)
	}
	expectNativeToValue(t, anyValue, NullValue)

	// Json Struct
	anyValue, err = ptypes.MarshalAny(&structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"a": {Kind: &structpb.Value_StringValue{StringValue: "world"}},
					"b": {Kind: &structpb.Value_StringValue{StringValue: "five!"}}}}}})
	if err != nil {
		t.Error(err)
	}
	expected := NewJSONStruct(reg, &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"a": {Kind: &structpb.Value_StringValue{StringValue: "world"}},
			"b": {Kind: &structpb.Value_StringValue{StringValue: "five!"}}}})
	expectNativeToValue(t, anyValue, expected)

	//Json List
	anyValue, err = ptypes.MarshalAny(&structpb.Value{
		Kind: &structpb.Value_ListValue{
			ListValue: &structpb.ListValue{
				Values: []*structpb.Value{
					{Kind: &structpb.Value_StringValue{StringValue: "world"}},
					{Kind: &structpb.Value_StringValue{StringValue: "five!"}}}}}})
	if err != nil {
		t.Error(err)
	}
	expectedList := NewJSONList(reg, &structpb.ListValue{
		Values: []*structpb.Value{
			{Kind: &structpb.Value_StringValue{StringValue: "world"}},
			{Kind: &structpb.Value_StringValue{StringValue: "five!"}}}})
	expectNativeToValue(t, anyValue, expectedList)

	// Object
	pbMessage := exprpb.ParsedExpr{
		SourceInfo: &exprpb.SourceInfo{
			LineOffsets: []int32{1, 2, 3}}}
	anyValue, err = ptypes.MarshalAny(&pbMessage)
	if err != nil {
		t.Error(err)
	}
	expectNativeToValue(t, anyValue, reg.NativeToValue(&pbMessage))
}

func TestNativeToValue_Json(t *testing.T) {
	reg := NewRegistry(&exprpb.ParsedExpr{})
	// Json primitive conversion test.
	expectNativeToValue(t,
		&structpb.Value{Kind: &structpb.Value_BoolValue{}},
		False)
	expectNativeToValue(t,
		&structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: 1.1}},
		Double(1.1))
	expectNativeToValue(t,
		&structpb.Value{Kind: &structpb.Value_NullValue{
			NullValue: structpb.NullValue_NULL_VALUE}},
		Null(structpb.NullValue_NULL_VALUE))
	expectNativeToValue(t,
		&structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "hello"}},
		String("hello"))

	// Json list conversion.
	expectNativeToValue(t,
		&structpb.Value{
			Kind: &structpb.Value_ListValue{
				ListValue: &structpb.ListValue{
					Values: []*structpb.Value{
						{Kind: &structpb.Value_StringValue{StringValue: "world"}},
						{Kind: &structpb.Value_StringValue{StringValue: "five!"}}}}}},
		NewJSONList(reg, &structpb.ListValue{
			Values: []*structpb.Value{
				{Kind: &structpb.Value_StringValue{StringValue: "world"}},
				{Kind: &structpb.Value_StringValue{StringValue: "five!"}}}}))

	// Json struct conversion.
	expectNativeToValue(t,
		&structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"a": {Kind: &structpb.Value_StringValue{StringValue: "world"}},
						"b": {Kind: &structpb.Value_StringValue{StringValue: "five!"}}}}}},
		NewJSONStruct(reg, &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"a": {Kind: &structpb.Value_StringValue{StringValue: "world"}},
				"b": {Kind: &structpb.Value_StringValue{StringValue: "five!"}}}}))

	// Proto conversion test.
	parsedExpr := &exprpb.ParsedExpr{}
	expectNativeToValue(t, parsedExpr, reg.NativeToValue(parsedExpr))
}

func TestNativeToValue_Wrappers(t *testing.T) {
	// Wrapper conversion test.
	expectNativeToValue(t, &wrapperspb.BoolValue{Value: true}, True)
	expectNativeToValue(t, &wrapperspb.BoolValue{}, False)
	expectNativeToValue(t, &wrapperspb.BytesValue{}, Bytes{})
	expectNativeToValue(t, &wrapperspb.BytesValue{Value: []byte("hi")}, Bytes("hi"))
	expectNativeToValue(t, &wrapperspb.DoubleValue{}, Double(0.0))
	expectNativeToValue(t, &wrapperspb.DoubleValue{Value: 6.4}, Double(6.4))
	expectNativeToValue(t, &wrapperspb.FloatValue{}, Double(0.0))
	expectNativeToValue(t, &wrapperspb.FloatValue{Value: 3.0}, Double(3.0))
	expectNativeToValue(t, &wrapperspb.Int32Value{}, IntZero)
	expectNativeToValue(t, &wrapperspb.Int32Value{Value: -32}, Int(-32))
	expectNativeToValue(t, &wrapperspb.Int64Value{}, IntZero)
	expectNativeToValue(t, &wrapperspb.Int64Value{Value: -64}, Int(-64))
	expectNativeToValue(t, &wrapperspb.StringValue{}, String(""))
	expectNativeToValue(t, &wrapperspb.StringValue{Value: "hello"}, String("hello"))
	expectNativeToValue(t, &wrapperspb.UInt32Value{}, Uint(0))
	expectNativeToValue(t, &wrapperspb.UInt32Value{Value: 32}, Uint(32))
	expectNativeToValue(t, &wrapperspb.UInt64Value{}, Uint(0))
	expectNativeToValue(t, &wrapperspb.UInt64Value{Value: 64}, Uint(64))
}

func TestNativeToValue_Primitive(t *testing.T) {
	reg := NewRegistry()

	// Core type conversions.
	expectNativeToValue(t, true, True)
	expectNativeToValue(t, int(-10), Int(-10))
	expectNativeToValue(t, int32(-1), Int(-1))
	expectNativeToValue(t, int64(2), Int(2))
	expectNativeToValue(t, uint(6), Uint(6))
	expectNativeToValue(t, uint32(3), Uint(3))
	expectNativeToValue(t, uint64(4), Uint(4))
	expectNativeToValue(t, float32(5.5), Double(5.5))
	expectNativeToValue(t, float64(-5.5), Double(-5.5))
	expectNativeToValue(t, "hello", String("hello"))
	expectNativeToValue(t, []byte("world"), Bytes("world"))
	expectNativeToValue(t, []int32{1, 2, 3}, NewDynamicList(reg, []int32{1, 2, 3}))
	expectNativeToValue(t, map[int32]int32{1: 1, 2: 1, 3: 1},
		NewDynamicMap(reg, map[int32]int32{1: 1, 2: 1, 3: 1}))

	// Pointers to core types.
	pBool := true
	expectNativeToValue(t, &pBool, True)
	pDub32 := float32(2.5)
	pDub64 := float64(-1000.2)
	expectNativeToValue(t, &pDub32, Double(2.5))
	expectNativeToValue(t, &pDub64, Double(-1000.2))
	pInt := int(1)
	pInt32 := int32(2)
	pInt64 := int64(-1000)
	expectNativeToValue(t, &pInt, Int(1))
	expectNativeToValue(t, &pInt32, Int(2))
	expectNativeToValue(t, &pInt64, Int(-1000))
	pStr := "hello"
	expectNativeToValue(t, &pStr, String("hello"))
	pUint := uint(1)
	pUint32 := uint32(2)
	pUint64 := uint64(1000)
	expectNativeToValue(t, &pUint, Uint(1))
	expectNativeToValue(t, &pUint32, Uint(2))
	expectNativeToValue(t, &pUint64, Uint(1000))

	// Pointers to ref.Val extensions of core types.
	rBool := True
	expectNativeToValue(t, &rBool, True)
	rDub := Double(32.1)
	expectNativeToValue(t, &rDub, rDub)
	rInt := Int(-12)
	expectNativeToValue(t, &rInt, rInt)
	rStr := String("hello")
	expectNativeToValue(t, &rStr, rStr)
	rUint := Uint(12405)
	expectNativeToValue(t, &rUint, rUint)
	rBytes := Bytes([]byte("hello"))
	expectNativeToValue(t, &rBytes, rBytes)

	// Extensions to core types.
	expectNativeToValue(t, testInt32(1), Int(1))
	expectNativeToValue(t, testInt64(-100), Int(-100))
	expectNativeToValue(t, testUint32(2), Uint(2))
	expectNativeToValue(t, testUint64(3), Uint(3))
	expectNativeToValue(t, testFloat32(4.5), Double(4.5))
	expectNativeToValue(t, testFloat64(-5.1), Double(-5.1))

	// Null conversion test.
	expectNativeToValue(t, nil, NullValue)
	expectNativeToValue(t, structpb.NullValue_NULL_VALUE, Null(structpb.NullValue_NULL_VALUE))
}

func TestUnsupportedConversion(t *testing.T) {
	reg := NewRegistry()
	if val := reg.NativeToValue(nonConvertible{}); !IsError(val) {
		t.Error("Expected error when converting non-proto struct to proto", val)
	}
}

func expectValueToNative(t *testing.T, in ref.Val, out interface{}) {
	t.Helper()
	if val, err := in.ConvertToNative(reflect.TypeOf(out)); err != nil {
		t.Error(err)
	} else {
		var equals bool
		switch val.(type) {
		case []byte:
			equals = bytes.Equal(val.([]byte), out.([]byte))
		case proto.Message:
			equals = proto.Equal(val.(proto.Message), out.(proto.Message))
		case bool, int32, int64, uint32, uint64, float32, float64, string:
			equals = val == out
		default:
			equals = reflect.DeepEqual(val, out)
		}
		if !equals {
			t.Errorf("Unexpected conversion from expr to proto.\n"+
				"expected: %T, actual: %T", val, out)
		}
	}
}

func expectNativeToValue(t *testing.T, in interface{}, out ref.Val) {
	t.Helper()
	reg := NewRegistry(&exprpb.ParsedExpr{})
	if val := reg.NativeToValue(in); IsError(val) {
		t.Error(val)
	} else {
		if val.Equal(out) != True {
			t.Errorf("Unexpected conversion from expr to proto.\n"+
				"expected: %T, actual: %T", val, out)
		}
	}
}

func BenchmarkTypeProvider_NewValue(b *testing.B) {
	reg := NewRegistry(&exprpb.ParsedExpr{})
	for i := 0; i < b.N; i++ {
		reg.NewValue(
			"google.api.expr.v1.SourceInfo",
			map[string]ref.Val{
				"Location":    String("BenchmarkTypeProvider_NewValue"),
				"LineOffsets": NewDynamicList(reg, []int64{0, 2}),
				"Positions":   NewDynamicMap(reg, map[int64]int64{1: 2, 2: 4}),
			})
	}
}

// Helper types useful for testing extensions of primitive types.
type nonConvertible struct {
	Field string
}
type testInt32 int32
type testInt64 int64
type testUint32 uint32
type testUint64 uint64
type testFloat32 float32
type testFloat64 float64
