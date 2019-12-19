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
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"

	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/packages"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/interpreter/functions"
	"github.com/google/cel-go/parser"

	structpb "github.com/golang/protobuf/ptypes/struct"
	tpb "github.com/golang/protobuf/ptypes/timestamp"
	wrapperspb "github.com/golang/protobuf/ptypes/wrappers"

	proto2pb "github.com/google/cel-go/test/proto2pb"
	proto3pb "github.com/google/cel-go/test/proto3pb"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

type testCase struct {
	name      string
	expr      string
	pkg       string
	env       []*exprpb.Decl
	types     []proto.Message
	funcs     []*functions.Overload
	attrs     AttributeFactory
	unchecked bool

	in  map[string]interface{}
	out interface{}
	err string
}

var (
	testData = []testCase{
		{
			name: "and_false_1st",
			expr: `false && true`,
			out:  types.False,
		},
		{
			name: "and_false_2nd",
			expr: `true && false`,
			out:  types.False,
		},
		{
			name: "and_error_1st_false",
			expr: `1/0 != 0 && false`,
			out:  types.False,
		},
		{
			name: "and_error_2nd_false",
			expr: `false && 1/0 != 0`,
			out:  types.False,
		},
		{
			name: "and_error_1st_error",
			expr: `1/0 != 0 && true`,
			err:  "divide by zero",
		},
		{
			name: "and_error_2nd_error",
			expr: `true && 1/0 != 0`,
			err:  "divide by zero",
		},
		{
			name:      "call_no_args",
			expr:      `zero()`,
			unchecked: true,
			funcs: []*functions.Overload{
				{
					Operator: "zero",
					Function: func(args ...ref.Val) ref.Val {
						return types.IntZero
					},
				},
			},
			out: types.IntZero,
		},
		{
			name:      "call_one_arg",
			expr:      `neg(1)`,
			unchecked: true,
			funcs: []*functions.Overload{
				{
					Operator:     "neg",
					OperandTrait: traits.NegatorType,
					Unary: func(arg ref.Val) ref.Val {
						return arg.(traits.Negater).Negate()
					},
				},
			},
			out: types.IntNegOne,
		},
		{
			name:      "call_two_arg",
			expr:      `b'abc'.concat(b'def')`,
			unchecked: true,
			funcs: []*functions.Overload{
				{
					Operator:     "concat",
					OperandTrait: traits.AdderType,
					Binary: func(lhs, rhs ref.Val) ref.Val {
						return lhs.(traits.Adder).Add(rhs)
					},
				},
			},
			out: []byte{'a', 'b', 'c', 'd', 'e', 'f'},
		},
		{
			name:      "call_varargs",
			expr:      `addall(a, b, c, d) == 10`,
			unchecked: true,
			funcs: []*functions.Overload{
				{
					Operator:     "addall",
					OperandTrait: traits.AdderType,
					Function: func(args ...ref.Val) ref.Val {
						val := types.Int(0)
						for _, arg := range args {
							val += arg.(types.Int)
						}
						return val
					},
				},
			},
			in: map[string]interface{}{
				"a": 1, "b": 2, "c": 3, "d": 4,
			},
		},
		{
			name: "complex",
			expr: `
			!(headers.ip in ["10.0.1.4", "10.0.1.5"]) &&
				((headers.path.startsWith("v1") && headers.token in ["v1", "v2", "admin"]) ||
				(headers.path.startsWith("v2") && headers.token in ["v2", "admin"]) ||
				(headers.path.startsWith("/admin") && headers.token == "admin" && headers.ip in ["10.0.1.2", "10.0.1.2", "10.0.1.2"]))
			`,
			env: []*exprpb.Decl{
				decls.NewIdent("headers", decls.NewMapType(decls.String, decls.String), nil),
			},
			in: map[string]interface{}{
				"headers": map[string]interface{}{
					"ip":    "10.0.1.2",
					"path":  "/admin/edit",
					"token": "admin",
				},
			},
		},
		{
			name: "complex_qual_vars",
			expr: `
			!(headers.ip in ["10.0.1.4", "10.0.1.5"]) &&
				((headers.path.startsWith("v1") && headers.token in ["v1", "v2", "admin"]) ||
				(headers.path.startsWith("v2") && headers.token in ["v2", "admin"]) ||
				(headers.path.startsWith("/admin") && headers.token == "admin" && headers.ip in ["10.0.1.2", "10.0.1.2", "10.0.1.2"]))
			`,
			env: []*exprpb.Decl{
				decls.NewIdent("headers.ip", decls.String, nil),
				decls.NewIdent("headers.path", decls.String, nil),
				decls.NewIdent("headers.token", decls.String, nil),
			},
			in: map[string]interface{}{
				"headers.ip":    "10.0.1.2",
				"headers.path":  "/admin/edit",
				"headers.token": "admin",
			},
		},
		{
			name: "cond",
			expr: `a ? b < 1.2 : c == ['hello']`,
			env: []*exprpb.Decl{
				decls.NewIdent("a", decls.Bool, nil),
				decls.NewIdent("b", decls.Double, nil),
				decls.NewIdent("c", decls.NewListType(decls.String), nil),
			},
			in: map[string]interface{}{
				"a": true,
				"b": 2.0,
				"c": []string{"hello"},
			},
			out: types.False,
		},
		{
			name: "in_list",
			expr: `6 in [2, 12, 6]`,
		},
		{
			name: "in_map",
			expr: `'other-key' in {'key': null, 'other-key': 42}`,
		},
		{
			name: "index",
			expr: `m['key'][1] == 42u && m['null'] == null`,
			env: []*exprpb.Decl{
				decls.NewIdent("m", decls.NewMapType(decls.String, decls.Dyn), nil),
			},
			in: map[string]interface{}{
				"m": map[string]interface{}{
					"key":  []uint{21, 42},
					"null": nil,
				},
			},
		},
		{
			name: "index_relative",
			expr: `([[[1]], [[2]], [[3]]][0][0] + [2, 3, {'four': {'five': 'six'}}])[3].four.five == 'six'`,
		},
		{
			name: "literal_bool_false",
			expr: `false`,
			out:  types.False,
		},
		{
			name: "literal_bool_true",
			expr: `true`,
		},
		{
			name: "literal_null",
			expr: `null`,
			out:  types.NullValue,
		},
		{
			name: "literal_list",
			expr: `[1, 2, 3]`,
			out:  []int64{1, 2, 3},
		},
		{
			name: "literal_map",
			expr: `{'hi': 21, 'world': 42u}`,
			out: map[string]interface{}{
				"hi":    21,
				"world": uint(42),
			},
		},
		{
			name: "literal_equiv_string_bytes",
			expr: `string(bytes("\303\277")) == '''\303\277'''`,
		},
		{
			name: "literal_not_equiv_string_bytes",
			expr: `string(b"\303\277") != '''\303\277'''`,
		},
		{
			name: "literal_equiv_bytes_string",
			expr: `string(b"\303\277") == 'ÿ'`,
		},
		{
			name: "literal_bytes_string",
			expr: `string(b'aaa"bbb')`,
			out:  `aaa"bbb`,
		},
		{
			name: "literal_bytes_string2",
			expr: `string(b"""Kim\t""")`,
			out: `Kim	`,
		},
		{
			name:  "literal_pb3_msg",
			pkg:   "google.api.expr",
			types: []proto.Message{&exprpb.Expr{}},
			expr: `v1alpha1.Expr{
				id: 1,
				const_expr: v1alpha1.Constant{
					string_value: "oneof_test"
				}
			}`,
			out: &exprpb.Expr{Id: 1,
				ExprKind: &exprpb.Expr_ConstExpr{
					ConstExpr: &exprpb.Constant{
						ConstantKind: &exprpb.Constant_StringValue{
							StringValue: "oneof_test"}}}},
		},
		{
			name:  "literal_pb_enum",
			pkg:   "google.expr.proto3.test",
			types: []proto.Message{&proto3pb.TestAllTypes{}},
			expr: `TestAllTypes{
				repeated_nested_enum: [
					0,
					TestAllTypes.NestedEnum.BAZ,
					TestAllTypes.NestedEnum.BAR],
				repeated_int32: [
					TestAllTypes.NestedEnum.FOO,
					TestAllTypes.NestedEnum.BAZ]}`,
			out: &proto3pb.TestAllTypes{
				RepeatedNestedEnum: []proto3pb.TestAllTypes_NestedEnum{
					proto3pb.TestAllTypes_FOO,
					proto3pb.TestAllTypes_BAZ,
					proto3pb.TestAllTypes_BAR,
				},
				RepeatedInt32: []int32{0, 2},
			},
		},
		{
			name: "string_to_timestamp",
			expr: `timestamp('1986-04-26T01:23:40Z')`,
			out:  &tpb.Timestamp{Seconds: 514862620},
		},
		{
			name: "macro_all_non_strict",
			expr: `![0, 2, 4].all(x, 4/x != 2 && 4/(4-x) != 2)`,
		},
		{
			name: "macro_all_non_strict_var",
			expr: `code == "111" && ["a", "b"].all(x, x in tags)
				|| code == "222" && ["a", "b"].all(x, x in tags)`,
			env: []*exprpb.Decl{
				decls.NewIdent("code", decls.String, nil),
				decls.NewIdent("tags", decls.NewListType(decls.String), nil),
			},
			in: map[string]interface{}{
				"code": "222",
				"tags": []string{"a", "b"},
			},
		},
		{
			name: "macro_exists_lit",
			expr: `[1, 2, 3, 4, 5u, 1.0].exists(e, type(e) == uint)`,
		},
		{
			name: "macro_exists_nonstrict",
			expr: `[0, 2, 4].exists(x, 4/x == 2 && 4/(4-x) == 2)`,
		},
		{
			name: "macro_exists_var",
			expr: `elems.exists(e, type(e) == uint)`,
			env: []*exprpb.Decl{
				decls.NewIdent("elems", decls.NewListType(decls.Dyn), nil),
			},
			in: map[string]interface{}{
				"elems": []interface{}{0, 1, 2, 3, 4, uint(5), 6},
			},
		},
		{
			name: "macro_exists_one",
			expr: `[1, 2, 3].exists_one(x, (x % 2) == 0)`,
		},
		{
			name: "macro_filter",
			expr: `[1, 2, 3].filter(x, x > 2) == [3]`,
		},
		{
			name: "macro_has_map_key",
			expr: `has({'a':1}.a) && !has({}.a)`,
		},
		{
			name:  "macro_has_pb3_msg_field",
			types: []proto.Message{&exprpb.ParsedExpr{}},
			pkg:   "google.api.expr.v1alpha1",
			expr: `has(v1alpha1.ParsedExpr{expr:Expr{id: 1}}.expr)
				&& !has(expr.v1alpha1.ParsedExpr{expr:Expr{id: 1}}.source_info)`,
		},
		{
			name: "macro_map",
			expr: `[1, 2, 3].map(x, x * 2) == [2, 4, 6]`,
		},
		{
			name: "matches",
			expr: `input.matches('k.*')
				&& !'foo'.matches('k.*')
				&& !'bar'.matches('k.*')
				&& 'kilimanjaro'.matches('.*ro')`,
			env: []*exprpb.Decl{
				decls.NewIdent("input", decls.String, nil),
			},
			in: map[string]interface{}{
				"input": "kathmandu",
			},
		},
		{
			name: "or_true_1st",
			expr: `ai == 20 || ar["foo"] == "bar"`,
			env: []*exprpb.Decl{
				decls.NewIdent("ai", decls.Int, nil),
				decls.NewIdent("ar", decls.NewMapType(decls.String, decls.String), nil),
			},
			in: map[string]interface{}{
				"ai": 20,
				"ar": map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			name: "or_true_2nd",
			expr: `ai == 20 || ar["foo"] == "bar"`,
			env: []*exprpb.Decl{
				decls.NewIdent("ai", decls.Int, nil),
				decls.NewIdent("ar", decls.NewMapType(decls.String, decls.String), nil),
			},
			in: map[string]interface{}{
				"ai": 2,
				"ar": map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			name: "or_false",
			expr: `ai == 20 || ar["foo"] == "bar"`,
			env: []*exprpb.Decl{
				decls.NewIdent("ai", decls.Int, nil),
				decls.NewIdent("ar", decls.NewMapType(decls.String, decls.String), nil),
			},
			in: map[string]interface{}{
				"ai": 2,
				"ar": map[string]string{
					"foo": "baz",
				},
			},
			out: types.False,
		},
		{
			name: "or_error_1st_error",
			expr: `1/0 != 0 || false`,
			err:  "divide by zero",
		},
		{
			name: "or_error_2nd_error",
			expr: `false || 1/0 != 0`,
			err:  "divide by zero",
		},
		{
			name: "or_error_1st_true",
			expr: `1/0 != 0 || true`,
			out:  types.True,
		},
		{
			name: "or_error_2nd_true",
			expr: `true || 1/0 != 0`,
			out:  types.True,
		},
		{
			name: "pkg_qualified_id",
			expr: `b.c.d != 10`,
			pkg:  "a.b",
			env: []*exprpb.Decl{
				decls.NewIdent("a.b.c.d", decls.Int, nil),
			},
			in: map[string]interface{}{
				"a.b.c.d": 9,
			},
		},
		{
			name:      "pkg_qualified_id_unchecked",
			expr:      `c.d != 10`,
			unchecked: true,
			pkg:       "a.b",
			in: map[string]interface{}{
				"a.c.d": 9,
			},
		},
		{
			name:      "pkg_qualified_index_unchecked",
			expr:      `b.c['d'] == 10`,
			unchecked: true,
			pkg:       "a.b",
			in: map[string]interface{}{
				"a.b.c": map[string]int{
					"d": 10,
				},
			},
		},
		{
			name: "select_key",
			expr: `m.strMap['val'] == 'string'
				&& m.floatMap['val'] == 1.5
				&& m.doubleMap['val'] == -2.0
				&& m.intMap['val'] == -3
				&& m.int32Map['val'] == 4
				&& m.int64Map['val'] == -5
				&& m.uintMap['val'] == 6u
				&& m.uint32Map['val'] == 7u
				&& m.uint64Map['val'] == 8u
				&& m.boolMap['val'] == true
				&& m.boolMap['val'] != false`,
			env: []*exprpb.Decl{
				decls.NewIdent("m", decls.NewMapType(decls.String, decls.Dyn), nil),
			},
			in: map[string]interface{}{
				"m": map[string]interface{}{
					"strMap":    map[string]string{"val": "string"},
					"floatMap":  map[string]float32{"val": 1.5},
					"doubleMap": map[string]float64{"val": -2.0},
					"intMap":    map[string]int{"val": -3},
					"int32Map":  map[string]int32{"val": 4},
					"int64Map":  map[string]int64{"val": -5},
					"uintMap":   map[string]uint{"val": 6},
					"uint32Map": map[string]uint32{"val": 7},
					"uint64Map": map[string]uint64{"val": 8},
					"boolMap":   map[string]bool{"val": true},
				},
			},
		},
		{
			name: "select_bool_key",
			expr: `m.boolStr[true] == 'string'
				&& m.boolFloat32[true] == 1.5
				&& m.boolFloat64[false] == -2.1
				&& m.boolInt[false] == -3
				&& m.boolInt32[false] == 0
				&& m.boolInt64[true] == 4
				&& m.boolUint[true] == 5u
				&& m.boolUint32[true] == 6u
				&& m.boolUint64[false] == 7u
				&& m.boolBool[true]
				&& m.boolIface[false] == true`,
			env: []*exprpb.Decl{
				decls.NewIdent("m", decls.NewMapType(decls.String, decls.Dyn), nil),
			},
			in: map[string]interface{}{
				"m": map[string]interface{}{
					"boolStr":     map[bool]string{true: "string"},
					"boolFloat32": map[bool]float32{true: 1.5},
					"boolFloat64": map[bool]float64{false: -2.1},
					"boolInt":     map[bool]int{false: -3},
					"boolInt32":   map[bool]int32{false: 0},
					"boolInt64":   map[bool]int64{true: 4},
					"boolUint":    map[bool]uint{true: 5},
					"boolUint32":  map[bool]uint32{true: 6},
					"boolUint64":  map[bool]uint64{false: 7},
					"boolBool":    map[bool]bool{true: true},
					"boolIface":   map[bool]interface{}{false: true},
				},
			},
		},
		{
			name: "select_uint_key",
			expr: `m.uintIface[1u] == 'string'
				&& m.uint32Iface[2u] == 1.5
				&& m.uint64Iface[3u] == -2.1
				&& m.uint64String[4u] == 'three'`,
			env: []*exprpb.Decl{
				decls.NewIdent("m", decls.NewMapType(decls.String, decls.Dyn), nil),
			},
			in: map[string]interface{}{
				"m": map[string]interface{}{
					"uintIface":    map[uint]interface{}{1: "string"},
					"uint32Iface":  map[uint32]interface{}{2: 1.5},
					"uint64Iface":  map[uint64]interface{}{3: -2.1},
					"uint64String": map[uint64]string{4: "three"},
				},
			},
		},
		{
			name: "select_index",
			expr: `m.strList[0] == 'string'
				&& m.floatList[0] == 1.5
				&& m.doubleList[0] == -2.0
				&& m.intList[0] == -3
				&& m.int32List[0] == 4
				&& m.int64List[0] == -5
				&& m.uintList[0] == 6u
				&& m.uint32List[0] == 7u
				&& m.uint64List[0] == 8u
				&& m.boolList[0] == true
				&& m.boolList[1] != true
				&& m.ifaceList[0] == {}`,
			env: []*exprpb.Decl{
				decls.NewIdent("m", decls.NewMapType(decls.String, decls.Dyn), nil),
			},
			in: map[string]interface{}{
				"m": map[string]interface{}{
					"strList":    []string{"string"},
					"floatList":  []float32{1.5},
					"doubleList": []float64{-2.0},
					"intList":    []int{-3},
					"int32List":  []int32{4},
					"int64List":  []int64{-5},
					"uintList":   []uint{6},
					"uint32List": []uint32{7},
					"uint64List": []uint64{8},
					"boolList":   []bool{true, false},
					"ifaceList":  []interface{}{map[string]string{}},
				},
			},
		},
		{
			name: "select_field",
			expr: `a.b.c
				&& pb3.repeated_nested_enum[0] == test.TestAllTypes.NestedEnum.BAR
				&& json.list[0] == 'world'`,
			pkg:   "google.expr.proto3",
			types: []proto.Message{&proto3pb.TestAllTypes{}},
			env: []*exprpb.Decl{
				decls.NewIdent("a.b", decls.NewMapType(decls.String, decls.Bool), nil),
				decls.NewIdent("pb3", decls.NewObjectType("google.expr.proto3.test.TestAllTypes"), nil),
				decls.NewIdent("json", decls.NewMapType(decls.String, decls.Dyn), nil),
			},
			in: map[string]interface{}{
				"a.b": map[string]bool{
					"c": true,
				},
				"pb3": &proto3pb.TestAllTypes{
					RepeatedNestedEnum: []proto3pb.TestAllTypes_NestedEnum{proto3pb.TestAllTypes_BAR},
				},
				"json": &structpb.Value{
					Kind: &structpb.Value_StructValue{
						StructValue: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"list": {Kind: &structpb.Value_ListValue{
									ListValue: &structpb.ListValue{
										Values: []*structpb.Value{
											{Kind: &structpb.Value_StringValue{
												StringValue: "world",
											}},
										},
									},
								}},
							},
						},
					},
				},
			},
		},
		// pb2 primitive fields may have default values set.
		{
			name: "select_pb2_primitive_fields",
			expr: `!has(a.single_int32)
			&& a.single_int32 == -32
			&& a.single_int64 == -64
			&& a.single_uint32 == 32u
			&& a.single_uint64 == 64u
			&& a.single_float == 3.0
			&& a.single_double == 6.4
			&& a.single_bool
			&& "empty" == a.single_string`,
			types: []proto.Message{&proto2pb.TestAllTypes{}},
			in: map[string]interface{}{
				"a": &proto2pb.TestAllTypes{},
			},
			env: []*exprpb.Decl{
				decls.NewIdent("a", decls.NewObjectType("google.expr.proto2.test.TestAllTypes"), nil),
			},
		},
		// Wrapper type nil or value test.
		{
			name: "select_pb3_wrapper_fields",
			expr: `!has(a.single_int32_wrapper) && a.single_int32_wrapper == null
				&& has(a.single_int64_wrapper) && a.single_int64_wrapper == 0
				&& has(a.single_string_wrapper) && a.single_string_wrapper == "hello"
				&& a.single_int64_wrapper == google.protobuf.Int32Value{value: 0}`,
			types: []proto.Message{&proto3pb.TestAllTypes{}},
			env: []*exprpb.Decl{
				decls.NewIdent("a", decls.NewObjectType("google.expr.proto3.test.TestAllTypes"), nil),
			},
			in: map[string]interface{}{
				"a": &proto3pb.TestAllTypes{
					SingleInt64Wrapper:  &wrapperspb.Int64Value{},
					SingleStringWrapper: &wrapperspb.StringValue{Value: "hello"},
				},
			},
		},
		{
			name:  "select_pb3_compare",
			expr:  `a.single_uint64 > 3u`,
			pkg:   "google.expr.proto3.test",
			types: []proto.Message{&proto3pb.TestAllTypes{}},
			env: []*exprpb.Decl{
				decls.NewIdent("a", decls.NewObjectType("google.expr.proto3.test.TestAllTypes"), nil),
			},
			in: map[string]interface{}{
				"a": &proto3pb.TestAllTypes{
					SingleUint64: 10,
				},
			},
			out: types.True,
		},
		{
			name:  "select_custom_pb3_compare",
			expr:  `a.bb > 100`,
			pkg:   "google.expr.proto3.test",
			types: []proto.Message{&proto3pb.TestAllTypes_NestedMessage{}},
			env: []*exprpb.Decl{
				decls.NewIdent("a",
					decls.NewObjectType("google.expr.proto3.test.TestAllTypes.NestedMessage"), nil),
			},
			attrs: &custAttrFactory{
				AttributeFactory: NewAttributeFactory(
					packages.NewPackage("google.expr.proto3.test"),
					types.NewRegistry(),
					types.NewRegistry(),
				),
			},
			in: map[string]interface{}{
				"a": &proto3pb.TestAllTypes_NestedMessage{
					Bb: 101,
				},
			},
			out: types.True,
		},
		{
			name: "select_relative",
			expr: `json('{"hi":"world"}').hi == 'world'`,
			env: []*exprpb.Decl{
				decls.NewFunction("json",
					decls.NewOverload("string_to_json",
						[]*exprpb.Type{decls.String}, decls.Dyn)),
			},
			funcs: []*functions.Overload{
				{
					Operator: "json",
					Unary: func(val ref.Val) ref.Val {
						str, ok := val.(types.String)
						if !ok {
							return types.ValOrErr(val, "no such overload")
						}
						m := make(map[string]interface{})
						err := json.Unmarshal([]byte(str), &m)
						if err != nil {
							return types.NewErr("invalid json: %v", err)
						}
						return types.DefaultTypeAdapter.NativeToValue(m)
					},
				},
			},
		},
		{
			name: "select_subsumed_field",
			expr: `a.b.c`,
			env: []*exprpb.Decl{
				decls.NewIdent("a.b.c", decls.Int, nil),
				decls.NewIdent("a.b", decls.NewMapType(decls.String, decls.String), nil),
			},
			in: map[string]interface{}{
				"a.b.c": 10,
				"a.b": map[string]string{
					"c": "ten",
				},
			},
			out: types.Int(10),
		},
	}
)

func BenchmarkInterpreter(b *testing.B) {
	for _, tst := range testData {
		prg, vars, err := program(&tst, Optimize())
		if err != nil {
			b.Fatal(err)
		}
		// Benchmark the eval.
		b.Run(tst.name, func(bb *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < bb.N; i++ {
				prg.Eval(vars)
			}
		})
	}
}

func BenchmarkInterpreter_Parallel(b *testing.B) {
	for _, tst := range testData {
		prg, vars, err := program(&tst, Optimize())
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		b.Run(tst.name,
			func(bb *testing.B) {
				bb.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						prg.Eval(vars)
					}
				})
			})
	}
}

func TestInterpreter(t *testing.T) {
	for _, tst := range testData {
		tc := tst
		prg, vars, err := program(&tc)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		t.Run(tc.name, func(tt *testing.T) {
			var want ref.Val = types.True
			if tc.out != nil {
				want = tc.out.(ref.Val)
			}
			got := prg.Eval(vars)
			_, expectUnk := want.(types.Unknown)
			if expectUnk {
				if !reflect.DeepEqual(got, want) {
					tt.Errorf("Got %v, wanted %v", got, want)
					return
				}
			} else if tc.err != "" {
				if !types.IsError(got) || got.(*types.Err).String() != tc.err {
					tt.Errorf("Got %v (%T), wanted error: %s", got, got, tc.err)
					return
				}
			} else if got.Equal(want) != types.True {
				tt.Errorf("Got %v, wanted %v", got, want)
				return
			}
			state := NewEvalState()
			opts := map[string]InterpretableDecorator{
				"optimize":   Optimize(),
				"exhaustive": ExhaustiveEval(state),
				"track":      TrackState(state),
			}
			for mode, opt := range opts {
				prg, vars, err = program(&tc, opt)
				if err != nil {
					tt.Fatal(err)
				}
				tt.Run(mode, func(ttt *testing.T) {
					got := prg.Eval(vars)
					_, expectUnk := want.(types.Unknown)
					if expectUnk {
						if !reflect.DeepEqual(got, want) {
							ttt.Errorf("Got %v, wanted %v", got, want)
						}
					} else if tc.err != "" {
						if !types.IsError(got) || got.(*types.Err).String() != tc.err {
							ttt.Errorf("Got %v (%T), wanted error: %s", got, got, tc.err)
						}
					} else if got.Equal(want) != types.True {
						ttt.Errorf("Got %v, wanted %v", got, want)
					}
					state.Reset()
				})
			}
		})
	}
}

func TestInterpreter_LogicalAndMissingType(t *testing.T) {
	src := common.NewTextSource(`a && TestProto{c: true}.c`)
	parsed, errors := parser.Parse(src)
	if len(errors.GetErrors()) != 0 {
		t.Errorf(errors.ToDisplayString())
	}

	reg := types.NewRegistry()
	pkg := packages.DefaultPackage
	attrs := NewAttributeFactory(pkg, reg, reg)
	intr := NewStandardInterpreter(pkg, reg, reg, attrs)
	i, err := intr.NewUncheckedInterpretable(parsed.GetExpr())
	if err == nil {
		t.Errorf("Got '%v', wanted error", i)
	}
}

func TestInterpreter_ExhaustiveConditionalExpr(t *testing.T) {
	src := common.NewTextSource(`a ? b < 1.0 : c == ['hello']`)
	parsed, errors := parser.Parse(src)
	if len(errors.GetErrors()) != 0 {
		t.Errorf(errors.ToDisplayString())
	}

	state := NewEvalState()
	pkg := packages.DefaultPackage
	reg := types.NewRegistry(&exprpb.ParsedExpr{})
	attrs := NewAttributeFactory(pkg, reg, reg)
	intr := NewStandardInterpreter(pkg, reg, reg, attrs)
	interpretable, _ := intr.NewUncheckedInterpretable(
		parsed.GetExpr(),
		ExhaustiveEval(state))
	vars, _ := NewActivation(map[string]interface{}{
		"a": types.True,
		"b": types.Double(0.999),
		"c": types.NewStringList(reg, []string{"hello"})})
	result := interpretable.Eval(vars)
	// Operator "_==_" is at Expr 7, should be evaluated in exhaustive mode
	// even though "a" is true
	ev, _ := state.Value(7)
	// "==" should be evaluated in exhaustive mode though unnecessary
	if ev != types.True {
		t.Errorf("Else expression expected to be true, got: %v", ev)
	}
	if result != types.True {
		t.Errorf("Expected true, got: %v", result)
	}
}

func TestInterpreter_ExhaustiveLogicalOrEquals(t *testing.T) {
	// a || b == "b"
	// Operator "==" is at Expr 4, should be evaluated though "a" is true
	src := common.NewTextSource(`a || b == "b"`)
	parsed, errors := parser.Parse(src)
	if len(errors.GetErrors()) != 0 {
		t.Errorf(errors.ToDisplayString())
	}

	state := NewEvalState()
	reg := types.NewRegistry(&exprpb.Expr{})
	pkg := packages.NewPackage("test")
	attrs := NewAttributeFactory(pkg, reg, reg)
	interp := NewStandardInterpreter(pkg, reg, reg, attrs)
	i, _ := interp.NewUncheckedInterpretable(
		parsed.GetExpr(),
		ExhaustiveEval(state))
	vars, _ := NewActivation(map[string]interface{}{
		"a": true,
		"b": "b",
	})
	result := i.Eval(vars)
	rhv, _ := state.Value(3)
	// "==" should be evaluated in exhaustive mode though unnecessary
	if rhv != types.True {
		t.Errorf("Right hand side expression expected to be true, got: %v", rhv)
	}
	if result != types.True {
		t.Errorf("Expected true, got: %v", result)
	}
}

func TestInterpreter_SetProto2PrimitiveFields(t *testing.T) {
	// Test the use of proto2 primitives within object construction.
	src := common.NewTextSource(
		`input == TestAllTypes{
			single_int32: 1,
			single_int64: 2,
			single_uint32: 3u,
			single_uint64: 4u,
			single_float: -3.3,
			single_double: -2.2,
			single_string: "hello world",
			single_bool: true
		}`)
	parsed, errors := parser.Parse(src)
	if len(errors.GetErrors()) != 0 {
		t.Errorf(errors.ToDisplayString())
	}

	pkg := packages.NewPackage("google.expr.proto2.test")
	reg := types.NewRegistry(&proto2pb.TestAllTypes{})
	env := checker.NewStandardEnv(pkg, reg)
	env.Add(decls.NewIdent("input", decls.NewObjectType("google.expr.proto2.test.TestAllTypes"), nil))
	checked, errors := checker.Check(parsed, src, env)
	if len(errors.GetErrors()) != 0 {
		t.Errorf(errors.ToDisplayString())
	}

	attrs := NewAttributeFactory(pkg, reg, reg)
	i := NewStandardInterpreter(pkg, reg, reg, attrs)
	eval, _ := i.NewInterpretable(checked)
	one := int32(1)
	two := int64(2)
	three := uint32(3)
	four := uint64(4)
	five := float32(-3.3)
	six := float64(-2.2)
	str := "hello world"
	truth := true
	input := &proto2pb.TestAllTypes{
		SingleInt32:  &one,
		SingleInt64:  &two,
		SingleUint32: &three,
		SingleUint64: &four,
		SingleFloat:  &five,
		SingleDouble: &six,
		SingleString: &str,
		SingleBool:   &truth,
	}
	vars, _ := NewActivation(map[string]interface{}{
		"input": reg.NativeToValue(input),
	})
	result := eval.Eval(vars)
	got, ok := result.(ref.Val).Value().(bool)
	if !ok {
		t.Fatalf("Got '%v', wanted 'true'.", result)
	}
	expected := true
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Could not build object properly. Got '%v', wanted '%v'",
			result.(ref.Val).Value(),
			expected)
	}
}

func TestInterpreter_MissingIdentInSelect(t *testing.T) {
	src := common.NewTextSource(`a.b.c`)
	parsed, errors := parser.Parse(src)
	if len(errors.GetErrors()) != 0 {
		t.Fatalf(errors.ToDisplayString())
	}

	pkg := packages.NewPackage("test")
	reg := types.NewRegistry()
	env := checker.NewStandardEnv(pkg, reg)
	env.Add(decls.NewIdent("a.b", decls.Dyn, nil))
	checked, errors := checker.Check(parsed, src, env)
	if len(errors.GetErrors()) != 0 {
		t.Fatalf(errors.ToDisplayString())
	}

	attrs := NewAttributeFactory(pkg, reg, reg)
	interp := NewStandardInterpreter(pkg, reg, reg, attrs)
	i, _ := interp.NewInterpretable(checked)
	vars := UnknownActivation()
	result := i.Eval(vars)
	if !types.IsUnknown(result) {
		t.Errorf("Got %v, wanted unknown", result)
	}

	vars = EmptyActivation()
	result = i.Eval(vars)
	if !types.IsError(result) {
		t.Errorf("Got %v, wanted error", result)
	}
}

func program(tst *testCase, opts ...InterpretableDecorator) (Interpretable, Activation, error) {
	// Configure the package.
	pkg := packages.DefaultPackage
	if tst.pkg != "" {
		pkg = packages.NewPackage(tst.pkg)
	}
	reg := types.NewRegistry()
	if tst.types != nil {
		reg = types.NewRegistry(tst.types...)
	}
	attrs := NewAttributeFactory(pkg, reg, reg)
	if tst.attrs != nil {
		attrs = tst.attrs
	}

	// Configure the environment.
	env := checker.NewStandardEnv(pkg, reg)
	if tst.env != nil {
		env.Add(tst.env...)
	}
	// Configure the program input.
	vars := EmptyActivation()
	if tst.in != nil {
		vars, _ = NewActivation(tst.in)
	}
	// Adapt the test output, if needed.
	if tst.out != nil {
		tst.out = reg.NativeToValue(tst.out)
	}

	disp := NewDispatcher()
	disp.Add(functions.StandardOverloads()...)
	if tst.funcs != nil {
		disp.Add(tst.funcs...)
	}
	interp := NewInterpreter(disp, pkg, reg, reg, attrs)

	// Parse the expression.
	s := common.NewTextSource(tst.expr)
	parsed, errs := parser.Parse(s)
	if len(errs.GetErrors()) != 0 {
		return nil, nil, errors.New(errs.ToDisplayString())
	}
	if tst.unchecked {
		// Build the program plan.
		prg, err := interp.NewUncheckedInterpretable(
			parsed.GetExpr(), opts...)
		if err != nil {
			return nil, nil, err
		}
		return prg, vars, nil
	}
	// Check the expression.
	checked, errs := checker.Check(parsed, s, env)
	if len(errs.GetErrors()) != 0 {
		return nil, nil, errors.New(errs.ToDisplayString())
	}
	// Build the program plan.
	prg, err := interp.NewInterpretable(checked, opts...)
	if err != nil {
		return nil, nil, err
	}
	return prg, vars, nil
}
