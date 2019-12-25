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
	"fmt"
	"io/ioutil"
	"log"
	"sync"
	"testing"

	"github.com/tamerh/cel-go/common"
	"github.com/tamerh/cel-go/common/operators"
	"github.com/tamerh/cel-go/common/overloads"
	"github.com/tamerh/cel-go/parser"

	"github.com/golang/protobuf/proto"
	"github.com/tamerh/cel-go/checker/decls"
	"github.com/tamerh/cel-go/common/types"
	"github.com/tamerh/cel-go/common/types/ref"
	"github.com/tamerh/cel-go/common/types/traits"
	"github.com/tamerh/cel-go/interpreter/functions"

	descpb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func Example() {
	// Create the CEL environment with declarations for the input attributes and
	// the desired extension functions. In many cases the desired functionality will
	// be present in a built-in function.
	decls := Declarations(
		// Identifiers used within this expression.
		decls.NewIdent("i", decls.String, nil),
		decls.NewIdent("you", decls.String, nil),
		// Function to generate a greeting from one person to another.
		//    i.greet(you)
		decls.NewFunction("greet",
			decls.NewInstanceOverload("string_greet_string",
				[]*exprpb.Type{decls.String, decls.String},
				decls.String)))
	e, err := NewEnv(decls)
	if err != nil {
		log.Fatalf("environment creation error: %s\n", err)
	}

	// Parse and check the expression.
	p, iss := e.Parse("i.greet(you)")
	if iss != nil && iss.Err() != nil {
		log.Fatalln(iss.Err())
	}
	c, iss := e.Check(p)
	if iss != nil && iss.Err() != nil {
		log.Fatalln(iss.Err())
	}

	// Create the program.
	funcs := Functions(
		&functions.Overload{
			Operator: "string_greet_string",
			Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
				return types.String(
					fmt.Sprintf("Hello %s! Nice to meet you, I'm %s.\n", rhs, lhs))
			}})
	prg, err := e.Program(c, funcs)
	if err != nil {
		log.Fatalf("program creation error: %s\n", err)
	}

	// Evaluate the program against some inputs. Note: the details return is not used.
	out, _, err := prg.Eval(map[string]interface{}{
		// Native values are converted to CEL values under the covers.
		"i": "CEL",
		// Values may also be lazily supplied.
		"you": func() ref.Val { return types.String("world") },
	})
	if err != nil {
		log.Fatalf("runtime error: %s\n", err)
	}

	fmt.Println(out)
	// Output:Hello world! Nice to meet you, I'm CEL.
}

// ExampleGlobalOverload demonstrates how to define global overload function.
func Example_globalOverload() {
	// Create the CEL environment with declarations for the input attributes and
	// the desired extension functions. In many cases the desired functionality will
	// be present in a built-in function.
	decls := Declarations(
		// Identifiers used within this expression.
		decls.NewIdent("i", decls.String, nil),
		decls.NewIdent("you", decls.String, nil),
		// Function to generate shake_hands between two people.
		//    shake_hands(i,you)
		decls.NewFunction("shake_hands",
			decls.NewOverload("shake_hands_string_string",
				[]*exprpb.Type{decls.String, decls.String},
				decls.String)))
	e, err := NewEnv(decls)
	if err != nil {
		log.Fatalf("environment creation error: %s\n", err)
	}

	// Parse and check the expression.
	p, iss := e.Parse(`shake_hands(i,you)`)
	if iss != nil && iss.Err() != nil {
		log.Fatalln(iss.Err())
	}
	c, iss := e.Check(p)
	if iss != nil && iss.Err() != nil {
		log.Fatalln(iss.Err())
	}

	// Create the program.
	funcs := Functions(
		&functions.Overload{
			Operator: "shake_hands_string_string",
			Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
				s1, ok := lhs.(types.String)
				if !ok {
					return types.ValOrErr(lhs, "unexpected type '%v' passed to shake_hands", lhs.Type())
				}
				s2, ok := rhs.(types.String)
				if !ok {
					return types.ValOrErr(rhs, "unexpected type '%v' passed to shake_hands", rhs.Type())
				}
				return types.String(
					fmt.Sprintf("%s and %s are shaking hands.\n", s1, s2))
			}})
	prg, err := e.Program(c, funcs)
	if err != nil {
		log.Fatalf("program creation error: %s\n", err)
	}

	// Evaluate the program against some inputs. Note: the details return is not used.
	out, _, err := prg.Eval(map[string]interface{}{
		"i":   "CEL",
		"you": func() ref.Val { return types.String("world") },
	})
	if err != nil {
		log.Fatalf("runtime error: %s\n", err)
	}

	fmt.Println(out)
	// Output:CEL and world are shaking hands.
}

func Test_ExampleWithBuiltins(t *testing.T) {
	// Variables used within this expression environment.
	decls := Declarations(
		decls.NewIdent("i", decls.String, nil),
		decls.NewIdent("you", decls.String, nil))
	env, err := NewEnv(decls)
	if err != nil {
		t.Fatalf("environment creation error: %s\n", err)
	}

	// Parse and type-check the expression.
	p, iss := env.Parse(`"Hello " + you + "! I'm " + i + "."`)
	if iss != nil && iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	c, iss := env.Check(p)
	if iss != nil && iss.Err() != nil {
		t.Fatal(iss.Err())
	}

	// Create the program, and evaluate it against some input.
	prg, err := env.Program(c)
	if err != nil {
		t.Fatalf("program creation error: %s\n", err)
	}

	// If the Eval() call were provided with cel.EvalOptions(OptTrackState) the details response
	// (2nd return) would be non-nil.
	out, _, err := prg.Eval(map[string]interface{}{
		"i":   "CEL",
		"you": "world"})
	if err != nil {
		t.Fatalf("runtime error: %s\n", err)
	}

	// Hello world! I'm CEL.
	if out.Equal(types.String("Hello world! I'm CEL.")) != types.True {
		t.Errorf(`Got '%v', wanted "Hello world! I'm CEL."`, out.Value())
	}
}

func Test_DisableStandardEnv(t *testing.T) {
	e, _ := NewEnv(
		ClearMacros(),
		ClearBuiltIns(),
		Declarations(decls.NewIdent("a.b.c", decls.Bool, nil)))

	t.Run("err", func(t *testing.T) {
		p, _ := e.Parse("a.b.c == true")
		_, iss := e.Check(p)
		if iss == nil || iss.Err() == nil {
			t.Error("Got successful check, expected error for missing operator '_==_'")
		}
	})

	t.Run("ok", func(t *testing.T) {
		p, _ := e.Parse("a.b.c")
		c, _ := e.Check(p)
		prg, _ := e.Program(c)
		out, _, _ := prg.Eval(map[string]interface{}{"a.b.c": true})
		if out != types.True {
			t.Errorf("Got '%v', wanted 'true'", out.Value())
		}
	})
}

func Test_HomogeneousAggregateLiterals(t *testing.T) {
	e, _ := NewEnv(
		ClearBuiltIns(),
		Declarations(
			decls.NewIdent("name", decls.String, nil),
			decls.NewFunction(
				operators.In,
				decls.NewOverload(overloads.InList, []*exprpb.Type{
					decls.String, decls.NewListType(decls.String),
				}, decls.Bool),
				decls.NewOverload(overloads.InMap, []*exprpb.Type{
					decls.String, decls.NewMapType(decls.String, decls.Bool),
				}, decls.Bool))),
		HomogeneousAggregateLiterals())

	t.Run("err_list", func(t *testing.T) {
		p, _ := e.Parse("name in ['hello', 0]")
		_, iss := e.Check(p)
		if iss == nil || iss.Err() == nil {
			t.Error("Got successful check, expected error for mixed list entry types.")
		}
	})
	t.Run("err_map_key", func(t *testing.T) {
		p, _ := e.Parse("name in {'hello':'world', 1:'!'}")
		_, iss := e.Check(p)
		if iss == nil || iss.Err() == nil {
			t.Error("Got successful check, expected error for mixed map key types.")
		}
	})
	t.Run("err_map_val", func(t *testing.T) {
		p, _ := e.Parse("name in {'hello':'world', 'goodbye':true}")
		_, iss := e.Check(p)
		if iss == nil || iss.Err() == nil {
			t.Error("Got successful check, expected error for mixed map value types.")
		}
	})
	funcs := Functions(&functions.Overload{
		Operator: operators.In,
		Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
			if rhs.Type().HasTrait(traits.ContainerType) {
				return rhs.(traits.Container).Contains(lhs)
			}
			return types.ValOrErr(rhs, "no such overload")
		},
	})
	t.Run("ok_list", func(t *testing.T) {
		p, _ := e.Parse("name in ['hello', 'world']")
		c, iss := e.Check(p)
		if iss != nil && iss.Err() != nil {
			t.Fatalf("Got issue: %v, expected successful check.", iss.Err())
		}
		prg, _ := e.Program(c, funcs)
		out, _, err := prg.Eval(map[string]interface{}{"name": "world"})
		if err != nil {
			t.Fatalf("Got err: %v, wanted result", err)
		}
		if out != types.True {
			t.Errorf("Got '%v', wanted 'true'", out)
		}
	})
	t.Run("ok_map", func(t *testing.T) {
		p, _ := e.Parse("name in {'hello': false, 'world': true}")
		c, iss := e.Check(p)
		if iss != nil && iss.Err() != nil {
			t.Fatalf("Got issue: %v, expected successful check.", iss.Err())
		}
		prg, _ := e.Program(c, funcs)
		out, _, err := prg.Eval(map[string]interface{}{"name": "world"})
		if err != nil {
			t.Fatalf("Got err: %v, wanted result", err)
		}
		if out != types.True {
			t.Errorf("Got '%v', wanted 'true'", out)
		}
	})
}

func Test_CustomTypes(t *testing.T) {
	exprType := decls.NewObjectType("google.api.expr.v1alpha1.Expr")
	reg := types.NewEmptyRegistry()
	e, _ := NewEnv(
		CustomTypeAdapter(reg),
		CustomTypeProvider(reg),
		Container("google.api.expr.v1alpha1"),
		Types(
			&exprpb.Expr{},
			types.BoolType,
			types.IntType,
			types.StringType),
		Declarations(
			decls.NewIdent("expr", exprType, nil)))

	p, _ := e.Parse(`
		expr == Expr{id: 2,
			call_expr: Expr.Call{
				function: "_==_",
				args: [
					Expr{id: 1, ident_expr: Expr.Ident{ name: "a" }},
					Expr{id: 3, ident_expr: Expr.Ident{ name: "b" }}]
			}}`)
	c, _ := e.Check(p)
	if !proto.Equal(c.ResultType(), decls.Bool) {
		t.Fatalf("Got %v, wanted type bool", c.ResultType())
	}
	prg, _ := e.Program(c)
	vars := map[string]interface{}{"expr": &exprpb.Expr{
		Id: 2,
		ExprKind: &exprpb.Expr_CallExpr{
			CallExpr: &exprpb.Expr_Call{
				Function: "_==_",
				Args: []*exprpb.Expr{
					{
						Id: 1,
						ExprKind: &exprpb.Expr_IdentExpr{
							IdentExpr: &exprpb.Expr_Ident{Name: "a"},
						},
					},
					{
						Id: 3,
						ExprKind: &exprpb.Expr_IdentExpr{
							IdentExpr: &exprpb.Expr_Ident{Name: "b"},
						},
					},
				},
			},
		},
	}}
	out, _, _ := prg.Eval(vars)
	if out != types.True {
		t.Errorf("Got '%v', wanted 'true'", out.Value())
	}
}

func Test_TypeIsolation(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/team.fds")
	if err != nil {
		t.Fatal("Can't read fds file: ", err)
	}
	var fds descpb.FileDescriptorSet
	if err = proto.Unmarshal(b, &fds); err != nil {
		t.Fatal("Can't unmarshal descriptor data: ", err)
	}

	e, err := NewEnv(
		TypeDescs(&fds),
		Declarations(
			decls.NewIdent("myteam",
				decls.NewObjectType("cel.testdata.Team"),
				nil)))
	if err != nil {
		t.Fatal("Can't create env: ", err)
	}

	src := "myteam.members[0].name == 'Cyclops'"
	p, _ := e.Parse(src)
	_, iss := e.Check(p)
	if iss != nil && iss.Err() != nil {
		t.Error(iss.Err())
	}

	// Ensure that isolated types don't leak through.
	e2, _ := NewEnv(
		Declarations(
			decls.NewIdent("myteam",
				decls.NewObjectType("cel.testdata.Team"),
				nil)))
	p2, _ := e2.Parse(src)
	_, iss = e2.Check(p2)
	if iss == nil || iss.Err() == nil {
		t.Errorf("Wanted check failure for unknown message.")
	}
}

func Test_GlobalVars(t *testing.T) {
	mapStrDyn := decls.NewMapType(decls.String, decls.Dyn)
	e, _ := NewEnv(
		Declarations(
			decls.NewIdent("attrs", mapStrDyn, nil),
			decls.NewIdent("default", decls.Dyn, nil),
			decls.NewFunction(
				"get",
				decls.NewInstanceOverload(
					"get_map",
					[]*exprpb.Type{mapStrDyn, decls.String, decls.Dyn},
					decls.Dyn))))
	p, _ := e.Parse(`attrs.get("first", attrs.get("second", default))`)
	c, _ := e.Check(p)

	// Create the program.
	funcs := Functions(
		&functions.Overload{
			Operator: "get",
			Function: func(args ...ref.Val) ref.Val {
				if len(args) != 3 {
					return types.NewErr("invalid arguments to 'get'")
				}
				attrs, ok := args[0].(traits.Mapper)
				if !ok {
					return types.NewErr(
						"invalid operand of type '%v' to obj.get(key, def)",
						args[0].Type())
				}
				key, ok := args[1].(types.String)
				if !ok {
					return types.NewErr(
						"invalid key of type '%v' to obj.get(key, def)",
						args[1].Type())
				}
				defVal := args[2]
				if attrs.Contains(key) == types.True {
					return attrs.Get(key)
				}
				return defVal
			}})

	// Global variables can be configured as a ProgramOption and optionally overridden on Eval.
	prg, _ := e.Program(c, funcs, Globals(map[string]interface{}{
		"default": "third",
	}))

	t.Run("global_default", func(t *testing.T) {
		vars := map[string]interface{}{
			"attrs": map[string]interface{}{}}
		out, _, _ := prg.Eval(vars)
		if out.Equal(types.String("third")) != types.True {
			t.Errorf("Got '%v', expected 'third'.", out.Value())
		}
	})

	t.Run("attrs_alt", func(t *testing.T) {
		vars := map[string]interface{}{
			"attrs": map[string]interface{}{"second": "yep"}}
		out, _, _ := prg.Eval(vars)
		if out.Equal(types.String("yep")) != types.True {
			t.Errorf("Got '%v', expected 'yep'.", out.Value())
		}
	})

	t.Run("local_default", func(t *testing.T) {
		vars := map[string]interface{}{
			"attrs":   map[string]interface{}{},
			"default": "fourth"}
		out, _, _ := prg.Eval(vars)
		if out.Equal(types.String("fourth")) != types.True {
			t.Errorf("Got '%v', expected 'fourth'.", out.Value())
		}
	})
}

func Test_CustomMacro(t *testing.T) {
	joinMacro := parser.NewReceiverMacro("join", 1,
		func(eh parser.ExprHelper,
			target *exprpb.Expr,
			args []*exprpb.Expr) (*exprpb.Expr, *common.Error) {
			delim := args[0]
			iterIdent := eh.Ident("__iter__")
			accuIdent := eh.Ident("__result__")
			init := eh.LiteralString("")
			condition := eh.LiteralBool(true)
			step := eh.GlobalCall(
				operators.Conditional,
				eh.GlobalCall(operators.Greater,
					eh.ReceiverCall("size", accuIdent),
					eh.LiteralInt(0)),
				eh.GlobalCall(
					operators.Add,
					eh.GlobalCall(
						operators.Add,
						accuIdent,
						delim),
					iterIdent),
				iterIdent)
			return eh.Fold(
				"__iter__",
				target,
				"__result__",
				init,
				condition,
				step,
				accuIdent), nil
		})
	e, _ := NewEnv(
		Macros(joinMacro),
	)
	p, iss := e.Parse(`['hello', 'cel', 'friend'].join(',')`)
	if iss != nil && iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	c, iss := e.Check(p)
	if iss != nil && iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	prg, err := e.Program(c, EvalOptions(OptExhaustiveEval))
	if err != nil {
		t.Fatalf("program creation error: %s\n", err)
	}
	out, _, err := prg.Eval(NoVars())
	if err != nil {
		t.Fatal(err)
	}
	if out.Equal(types.String("hello,cel,friend")) != types.True {
		t.Errorf("Got %v, wanted 'hello,cel,friend'", out)
	}
}

func Test_EvalOptions(t *testing.T) {
	e, _ := NewEnv(
		Declarations(
			decls.NewIdent("k", decls.String, nil),
			decls.NewIdent("v", decls.Bool, nil)))
	p, _ := e.Parse(`{k: true}[k] || v != false`)
	c, _ := e.Check(p)

	prg, err := e.Program(c, EvalOptions(OptExhaustiveEval))
	if err != nil {
		t.Fatalf("program creation error: %s\n", err)
	}
	out, details, err := prg.Eval(
		map[string]interface{}{
			"k": "key",
			"v": true})
	if err != nil {
		t.Fatalf("runtime error: %s\n", err)
	}
	if out != types.True {
		t.Errorf("Got '%v', expected 'true'", out.Value())
	}

	// Test to see whether 'v != false' was resolved to a value.
	// With short-circuiting it normally wouldn't be.
	s := details.State()
	lhsVal, found := s.Value(p.Expr().GetCallExpr().GetArgs()[0].Id)
	if !found {
		t.Error("Got not found, wanted evaluation of left hand side expression.")
		return
	}
	if lhsVal != types.True {
		t.Errorf("Got '%v', expected 'true'", lhsVal)
	}
	rhsVal, found := s.Value(p.Expr().GetCallExpr().GetArgs()[1].Id)
	if !found {
		t.Error("Got not found, wanted evaluation of right hand side expression.")
		return
	}
	if rhsVal != types.True {
		t.Errorf("Got '%v', expected 'true'", rhsVal)
	}
}

func Test_EvalRecover(t *testing.T) {
	e, _ := NewEnv(
		Declarations(
			decls.NewFunction("panic",
				decls.NewOverload("panic", []*exprpb.Type{}, decls.Bool)),
		))
	funcs := Functions(&functions.Overload{
		Operator: "panic",
		Function: func(args ...ref.Val) ref.Val {
			panic("watch me recover")
		},
	})
	// Test standard evaluation.
	pAst, _ := e.Parse("panic()")
	prgm, _ := e.Program(pAst, funcs)
	_, _, err := prgm.Eval(map[string]interface{}{})
	if err.Error() != "internal error: watch me recover" {
		t.Errorf("Got '%v', wanted 'internal error: watch me recover'", err)
	}
	// Test the factory-based evaluation.
	prgm, _ = e.Program(pAst, funcs, EvalOptions(OptTrackState))
	_, _, err = prgm.Eval(map[string]interface{}{})
	if err.Error() != "internal error: watch me recover" {
		t.Errorf("Got '%v', wanted 'internal error: watch me recover'", err)
	}
}

func Benchmark_EvalOptions(b *testing.B) {
	e, _ := NewEnv(
		Declarations(
			decls.NewIdent("ai", decls.Int, nil),
			decls.NewIdent("ar", decls.NewMapType(decls.String, decls.String), nil),
		),
	)
	pAst, _ := e.Parse("ai == 20 || ar['foo'] == 'bar'")
	cAst, _ := e.Check(pAst)
	vars := map[string]interface{}{
		"ai": 2,
		"ar": map[string]string{
			"foo": "bar",
		},
	}

	opts := map[string]EvalOption{
		"track-state":     OptTrackState,
		"exhaustive-eval": OptExhaustiveEval,
		"optimize":        OptOptimize,
	}
	for k, opt := range opts {
		b.Run(k, func(bb *testing.B) {
			prg, _ := e.Program(cAst, EvalOptions(opt))
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < bb.N; i++ {
				prg.Eval(vars)
			}
		})
	}
}

func Test_EnvExtension(t *testing.T) {
	e, _ := NewEnv(
		Container("google.api.expr.v1alpha1"),
		Types(&exprpb.Expr{}),
		Declarations(
			decls.NewIdent("expr",
				decls.NewObjectType("google.api.expr.v1alpha1.Expr"), nil),
		),
	)
	e2, _ := e.Extend()
	if e == e2 {
		t.Error("Got object equality, wanted separate objects")
	}
}

func Test_ParseAndCheckConcurrently(t *testing.T) {
	e, _ := NewEnv(
		Container("google.api.expr.v1alpha1"),
		Types(&exprpb.Expr{}),
		Declarations(
			decls.NewIdent("expr",
				decls.NewObjectType("google.api.expr.v1alpha1.Expr"), nil),
		),
	)

	parseAndCheck := func(expr string) {
		p, iss := e.Parse(expr)
		if iss != nil && iss.Err() != nil {
			t.Fatalf("Failed to parse '%s': %v", expr, iss.Err())
		}
		_, iss = e.Check(p)
		if iss != nil && iss.Err() != nil {
			t.Fatalf("Failed to check '%s': %v", expr, iss.Err())
		}
	}

	const concurrency = 10
	wgDone := sync.WaitGroup{}
	wgDone.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(i int) {
			defer wgDone.Done()
			parseAndCheck(fmt.Sprintf("expr.id + %d", i))
		}(i)
	}
	wgDone.Wait()
}
