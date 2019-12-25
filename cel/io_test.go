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

package cel

import (
	"testing"

	"github.com/golang/protobuf/proto"

	"github.com/tamerh/cel-go/checker/decls"
)

func TestIO_AstToProto(t *testing.T) {
	stdEnv, _ := NewEnv(Declarations(
		decls.NewIdent("a", decls.Dyn, nil),
		decls.NewIdent("b", decls.Dyn, nil),
	))
	ast, _ := stdEnv.Parse("a + b")
	parsed, err := AstToParsedExpr(ast)
	if err != nil {
		t.Fatal(err)
	}
	ast2 := ParsedExprToAst(parsed)
	if !proto.Equal(ast2.Expr(), ast.Expr()) {
		t.Errorf("Got %v, wanted %v", ast2, ast)
	}

	_, err = AstToCheckedExpr(ast)
	if err == nil {
		t.Fatal("expected error converting unchecked ast")
	}
	ast, iss := stdEnv.Check(ast)
	if iss != nil && iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	checked, err := AstToCheckedExpr(ast)
	if err != nil {
		t.Fatal(err)
	}
	ast3 := CheckedExprToAst(checked)
	if !proto.Equal(ast3.Expr(), ast.Expr()) {
		t.Fatalf("Got %v, wanted %v", ast3, ast)
	}
}

func TestIO_AstToString(t *testing.T) {
	stdEnv, _ := NewEnv()
	in := "a + b - (c ? (-d + 4) : e)"
	ast, _ := stdEnv.Parse(in)
	expr, err := AstToString(ast)
	if err != nil {
		t.Fatal(err)
	}
	if expr != in {
		t.Errorf("Got %v, wanted %v", expr, in)
	}
}
