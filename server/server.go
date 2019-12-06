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

// Package server defines the gRPC conformance test server for CEL Go.
package server

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	test2pb "github.com/google/cel-spec/proto/test/v1/proto2/test_all_types"
	test3pb "github.com/google/cel-spec/proto/test/v1/proto3/test_all_types"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	rpc "google.golang.org/genproto/googleapis/rpc/status"
)

// ConformanceServer contains the server state.
type ConformanceServer struct{}

// Parse implements ConformanceService.Parse.
func (s *ConformanceServer) Parse(ctx context.Context, in *exprpb.ParseRequest) (*exprpb.ParseResponse, error) {
	if in.CelSource == "" {
		st := status.New(codes.InvalidArgument, "No source code.")
		return nil, st.Err()
	}
	// NOTE: syntax_version isn't currently used
	var parseOptions []cel.EnvOption
	if in.DisableMacros {
		parseOptions = append(parseOptions, cel.ClearMacros())
	}
	env, _ := cel.NewEnv(parseOptions...)
	past, iss := env.Parse(in.CelSource)
	resp := exprpb.ParseResponse{}
	if iss == nil || iss.Err() == nil {
		// Success
		resp.ParsedExpr, _ = cel.AstToParsedExpr(past)
	} else {
		// Failure
		appendErrors(iss.Errors(), &resp.Issues)
	}
	return &resp, nil
}

// Check implements ConformanceService.Check.
func (s *ConformanceServer) Check(ctx context.Context, in *exprpb.CheckRequest) (*exprpb.CheckResponse, error) {
	if in.ParsedExpr == nil {
		st := status.New(codes.InvalidArgument, "No parsed expression.")
		return nil, st.Err()
	}
	if in.ParsedExpr.SourceInfo == nil {
		st := status.New(codes.InvalidArgument, "No source info.")
		return nil, st.Err()
	}
	// Build the environment.
	var checkOptions []cel.EnvOption
	if in.NoStdEnv {
		checkOptions = append(checkOptions, cel.ClearBuiltIns())
	}
	checkOptions = append(checkOptions, cel.Container(in.Container))
	checkOptions = append(checkOptions, cel.Declarations(in.TypeEnv...))
	checkOptions = append(checkOptions, cel.Types(&test2pb.TestAllTypes{}))
	checkOptions = append(checkOptions, cel.Types(&test3pb.TestAllTypes{}))
	env, _ := cel.NewEnv(checkOptions...)

	// Check the expression.
	cast, iss := env.Check(cel.ParsedExprToAst(in.ParsedExpr))
	resp := exprpb.CheckResponse{}
	if iss == nil || iss.Err() == nil {
		// Success
		resp.CheckedExpr, _ = cel.AstToCheckedExpr(cast)
	} else {
		// Failure
		appendErrors(iss.Errors(), &resp.Issues)
	}
	return &resp, nil
}

// Eval implements ConformanceService.Eval.
func (s *ConformanceServer) Eval(ctx context.Context, in *exprpb.EvalRequest) (*exprpb.EvalResponse, error) {
	env, _ := cel.NewEnv(cel.Container(in.Container),
		cel.Types(&test2pb.TestAllTypes{}, &test3pb.TestAllTypes{}))
	var prg cel.Program
	var err error
	switch in.ExprKind.(type) {
	case *exprpb.EvalRequest_ParsedExpr:
		ast := cel.ParsedExprToAst(in.GetParsedExpr())
		prg, err = env.Program(ast)
		if err != nil {
			return nil, err
		}
	case *exprpb.EvalRequest_CheckedExpr:
		ast := cel.CheckedExprToAst(in.GetCheckedExpr())
		prg, err = env.Program(ast)
		if err != nil {
			return nil, err
		}
	default:
		st := status.New(codes.InvalidArgument, "No expression.")
		return nil, st.Err()
	}
	args := make(map[string]interface{})
	for name, exprValue := range in.Bindings {
		refVal, err := ExprValueToRefValue(env.TypeAdapter(), exprValue)
		if err != nil {
			return nil, fmt.Errorf("can't convert binding %s: %s", name, err)
		}
		args[name] = refVal
	}
	// NOTE: the EvalState is currently discarded
	res, _, err := prg.Eval(args)
	resultExprVal, err := RefValueToExprValue(res, err)
	if err != nil {
		return nil, fmt.Errorf("con't convert result: %s", err)
	}
	return &exprpb.EvalResponse{Result: resultExprVal}, nil
}

// appendErrors converts the errors from errs to Status messages
// and appends them to the list of issues.
func appendErrors(errs []common.Error, issues *[]*rpc.Status) {
	for _, e := range errs {
		status := ErrToStatus(e, exprpb.IssueDetails_ERROR)
		*issues = append(*issues, status)
	}
}

// ErrToStatus converts an Error to a Status message with the given severity.
func ErrToStatus(e common.Error, severity exprpb.IssueDetails_Severity) *rpc.Status {
	detail := exprpb.IssueDetails{
		Severity: severity,
		Position: &exprpb.SourcePosition{
			Line:   int32(e.Location.Line()),
			Column: int32(e.Location.Column()),
		},
	}
	s := status.New(codes.InvalidArgument, e.Message)
	sd, err := s.WithDetails(&detail)
	if err == nil {
		return sd.Proto()
	}
	return s.Proto()
}

// TODO(jimlarson): The following conversion code should be moved to
// common/types/provider.go and consolidated/refactored as appropriate.
// In particular, make judicious use of types.NativeToValue().

// RefValueToExprValue converts between ref.Val and exprpb.ExprValue.
func RefValueToExprValue(res ref.Val, err error) (*exprpb.ExprValue, error) {
	if err != nil {
		s := status.Convert(err).Proto()
		return &exprpb.ExprValue{
			Kind: &exprpb.ExprValue_Error{
				Error: &exprpb.ErrorSet{
					Errors: []*rpc.Status{s},
				},
			},
		}, nil
	}
	if types.IsUnknown(res) {
		return &exprpb.ExprValue{
			Kind: &exprpb.ExprValue_Unknown{
				Unknown: &exprpb.UnknownSet{
					Exprs: res.Value().([]int64),
				},
			}}, nil
	}
	v, err := RefValueToValue(res)
	if err != nil {
		return nil, err
	}
	return &exprpb.ExprValue{
		Kind: &exprpb.ExprValue_Value{Value: v}}, nil
}

var (
	typeNameToTypeValue = map[string]*types.TypeValue{
		"bool":      types.BoolType,
		"bytes":     types.BytesType,
		"double":    types.DoubleType,
		"null_type": types.NullType,
		"int":       types.IntType,
		"list":      types.ListType,
		"map":       types.MapType,
		"string":    types.StringType,
		"type":      types.TypeType,
		"uint":      types.UintType,
	}
)

// RefValueToValue converts between ref.Val and Value.
// The ref.Val must not be error or unknown.
func RefValueToValue(res ref.Val) (*exprpb.Value, error) {
	switch res.Type() {
	case types.BoolType:
		return &exprpb.Value{
			Kind: &exprpb.Value_BoolValue{BoolValue: res.Value().(bool)}}, nil
	case types.BytesType:
		return &exprpb.Value{
			Kind: &exprpb.Value_BytesValue{BytesValue: res.Value().([]byte)}}, nil
	case types.DoubleType:
		return &exprpb.Value{
			Kind: &exprpb.Value_DoubleValue{DoubleValue: res.Value().(float64)}}, nil
	case types.IntType:
		return &exprpb.Value{
			Kind: &exprpb.Value_Int64Value{Int64Value: res.Value().(int64)}}, nil
	case types.ListType:
		l := res.(traits.Lister)
		sz := l.Size().(types.Int)
		elts := make([]*exprpb.Value, 0, int64(sz))
		for i := types.Int(0); i < sz; i++ {
			v, err := RefValueToValue(l.Get(i))
			if err != nil {
				return nil, err
			}
			elts = append(elts, v)
		}
		return &exprpb.Value{
			Kind: &exprpb.Value_ListValue{
				ListValue: &exprpb.ListValue{Values: elts}}}, nil
	case types.MapType:
		mapper := res.(traits.Mapper)
		sz := mapper.Size().(types.Int)
		entries := make([]*exprpb.MapValue_Entry, 0, int64(sz))
		for it := mapper.Iterator(); it.HasNext().(types.Bool); {
			k := it.Next()
			v := mapper.Get(k)
			kv, err := RefValueToValue(k)
			if err != nil {
				return nil, err
			}
			vv, err := RefValueToValue(v)
			if err != nil {
				return nil, err
			}
			entries = append(entries, &exprpb.MapValue_Entry{Key: kv, Value: vv})
		}
		return &exprpb.Value{
			Kind: &exprpb.Value_MapValue{
				MapValue: &exprpb.MapValue{Entries: entries}}}, nil
	case types.NullType:
		return &exprpb.Value{
			Kind: &exprpb.Value_NullValue{}}, nil
	case types.StringType:
		return &exprpb.Value{
			Kind: &exprpb.Value_StringValue{StringValue: res.Value().(string)}}, nil
	case types.TypeType:
		typeName := res.(ref.Type).TypeName()
		return &exprpb.Value{Kind: &exprpb.Value_TypeValue{TypeValue: typeName}}, nil
	case types.UintType:
		return &exprpb.Value{
			Kind: &exprpb.Value_Uint64Value{Uint64Value: res.Value().(uint64)}}, nil
	default:
		// Object type
		pb, ok := res.Value().(proto.Message)
		if !ok {
			return nil, status.New(codes.InvalidArgument, "Expected proto message").Err()
		}
		any, err := ptypes.MarshalAny(pb)
		if err != nil {
			return nil, err
		}
		return &exprpb.Value{
			Kind: &exprpb.Value_ObjectValue{ObjectValue: any}}, nil
	}
}

// ExprValueToRefValue converts between exprpb.ExprValue and ref.Val.
func ExprValueToRefValue(adapter ref.TypeAdapter, ev *exprpb.ExprValue) (ref.Val, error) {
	switch ev.Kind.(type) {
	case *exprpb.ExprValue_Value:
		return ValueToRefValue(adapter, ev.GetValue())
	case *exprpb.ExprValue_Error:
		// An error ExprValue is a repeated set of rpc.Status
		// messages, with no convention for the status details.
		// To convert this to a types.Err, we need to convert
		// these Status messages to a single string, and be
		// able to decompose that string on output so we can
		// round-trip arbitrary ExprValue messages.
		// TODO(jimlarson) make a convention for this.
		return types.NewErr("XXX add details later"), nil
	case *exprpb.ExprValue_Unknown:
		return types.Unknown(ev.GetUnknown().Exprs), nil
	}
	return nil, status.New(codes.InvalidArgument, "unknown ExprValue kind").Err()
}

// ValueToRefValue converts between exprpb.Value and ref.Val.
func ValueToRefValue(adapter ref.TypeAdapter, v *exprpb.Value) (ref.Val, error) {
	switch v.Kind.(type) {
	case *exprpb.Value_NullValue:
		return types.NullValue, nil
	case *exprpb.Value_BoolValue:
		return types.Bool(v.GetBoolValue()), nil
	case *exprpb.Value_Int64Value:
		return types.Int(v.GetInt64Value()), nil
	case *exprpb.Value_Uint64Value:
		return types.Uint(v.GetUint64Value()), nil
	case *exprpb.Value_DoubleValue:
		return types.Double(v.GetDoubleValue()), nil
	case *exprpb.Value_StringValue:
		return types.String(v.GetStringValue()), nil
	case *exprpb.Value_BytesValue:
		return types.Bytes(v.GetBytesValue()), nil
	case *exprpb.Value_ObjectValue:
		any := v.GetObjectValue()
		var msg ptypes.DynamicAny
		if err := ptypes.UnmarshalAny(any, &msg); err != nil {
			return nil, err
		}
		return adapter.NativeToValue(msg.Message), nil
	case *exprpb.Value_MapValue:
		m := v.GetMapValue()
		entries := make(map[ref.Val]ref.Val)
		for _, entry := range m.Entries {
			key, err := ValueToRefValue(adapter, entry.Key)
			if err != nil {
				return nil, err
			}
			pb, err := ValueToRefValue(adapter, entry.Value)
			if err != nil {
				return nil, err
			}
			entries[key] = pb
		}
		return adapter.NativeToValue(entries), nil
	case *exprpb.Value_ListValue:
		l := v.GetListValue()
		elts := make([]ref.Val, len(l.Values))
		for i, e := range l.Values {
			rv, err := ValueToRefValue(adapter, e)
			if err != nil {
				return nil, err
			}
			elts[i] = rv
		}
		return adapter.NativeToValue(elts), nil
	case *exprpb.Value_TypeValue:
		typeName := v.GetTypeValue()
		tv, ok := typeNameToTypeValue[typeName]
		if ok {
			return tv, nil
		}
		return types.NewObjectTypeValue(typeName), nil
	}
	return nil, status.New(codes.InvalidArgument, "unknown value").Err()
}
