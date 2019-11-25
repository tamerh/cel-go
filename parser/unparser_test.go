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

package parser

import (
	"testing"

	"github.com/golang/protobuf/proto"

	"github.com/google/cel-go/common"
)

func TestUnparse_Identical(t *testing.T) {
	tests := map[string]string{
		"call_add":            `a + b - c`,
		"call_and":            `a && b && c && d && e`,
		"call_and_or":         `a || b && (c || d) && e`,
		"call_cond":           `a ? b : c`,
		"call_index":          `a[1]["b"]`,
		"call_index_eq":       `x["a"].single_int32 == 23`,
		"call_mul":            `a * (b / c) % 0`,
		"call_mul_add":        `a + b * c`,
		"call_mul_add_nested": `(a + b) * c / (d - e)`,
		"call_mul_nested":     `a * b / c % 0`,
		"call_not":            `!true`,
		"call_neg":            `-num`,
		"call_or":             `a || b || c || d || e`,
		"call_neg_mult":       `-(1 * 2)`,
		"call_neg_add":        `-(1 + 2)`,
		"calc_distr_paren":    `(1 + 2) * 3`,
		"calc_distr_noparen":  `1 + 2 * 3`,
		"cond_tern_simple":    `(x > 5) ? (x - 5) : 0`,
		"func_global":         `size(a ? (b ? c : d) : e)`,
		"func_member":         `a.hello("world")`,
		"func_no_arg":         `zero()`,
		"func_one_arg":        `one("a")`,
		"func_two_args":       `and(d, 32u)`,
		"func_var_args":       `max(a, b, 100)`,
		"func_neq":            `x != "a"`,
		"func_in":             `a in b`,
		"list_empty":          `[]`,
		"list_one":            `[1]`,
		"list_many":           `["hello, world", "goodbye, world", "sure, why not?"]`,
		"lit_bytes":           `b"\303\203\302\277"`,
		"lit_double":          `-42.101`,
		"lit_false":           `false`,
		"lit_int":             `-405069`,
		"lit_null":            `null`,
		"lit_string":          `"hello:\t'world'"`,
		"lit_true":            `true`,
		"lit_uint":            `42u`,
		"ident":               `my_ident`,
		"macro_has":           `has(hello.world)`,
		"map_empty":           `{}`,
		"map_lit_key":         `{"a": a.b.c, b"\142": bytes(a.b.c)}`,
		"map_expr_key":        `{a: a, b: a.b, c: a.b.c, a ? b : c: false, a || b: true}`,
		"msg_empty":           `v1alpha1.Expr{}`,
		"msg_fields":          `v1alpha1.Expr{id: 1, call_expr: v1alpha1.Call_Expr{function: "name"}}`,
		"select":              `a.b.c`,
		"idx_idx_sel":         `a[b][c].name`,
		"sel_expr_target":     `(a + b).name`,
		"sel_cond_target":     `(a ? b : c).name`,
		"idx_cond_target":     `(a ? b : c)[0]`,
		"cond_conj":           `(a1 && a2) ? b : c`,
		"cond_disj_conj":      `a ? (b1 || b2) : (c1 && c2)`,
		"call_cond_target":    `(a ? b : c).method(d)`,
		"cond_flat":           `false && !true || false`,
		"cond_paren":          `false && (!true || false)`,
		"cond_cond":           `(false && !true || false) ? 2 : 3`,
		"cond_binop":          `(x < 5) ? x : 5`,
		"cond_binop_binop":    `(x > 5) ? (x - 5) : 0`,
		"cond_cond_binop":     `(x > 5) ? ((x > 10) ? (x - 10) : 5) : 0`,
		//"comp_all":            `[1, 2, 3].all(x, x > 0)`,
		//"comp_exists":         `[1, 2, 3].exists(x, x > 0)`,
		//"comp_map":            `[1, 2, 3].map(x, x >= 2, x * 4)`,
		//"comp_exists_one":     `[1, 2, 3].exists_one(x, x >= 2)`,
	}

	for name, in := range tests {
		t.Run(name, func(tt *testing.T) {
			p, iss := Parse(common.NewTextSource(in))
			if len(iss.GetErrors()) > 0 {
				tt.Fatal(iss.ToDisplayString())
			}
			out, err := Unparse(p.GetExpr(), p.GetSourceInfo())
			if err != nil {
				tt.Error(err)
			}
			if out != in {
				tt.Errorf("Got '%s', wanted '%s'", out, in)
			}
			p2, _ := Parse(common.NewTextSource(out))
			before := p.GetExpr()
			after := p2.GetExpr()
			if !proto.Equal(before, after) {
				tt.Errorf("Second parse differs from the first. Got '%v', wanted '%v'",
					before, after)
			}
		})
	}
}

func TestUnparse_Equivalent(t *testing.T) {
	tests := map[string][]string{
		"call_add":   {`a+b-c`, `a + b - c`},
		"call_cond":  {`a ? b          : c`, `a ? b :          c`},
		"call_index": {`a[  1  ]["b"]`, `a[  1]  ["b"]`},
		"call_or_and": {`(false && !true) || false`,
			` false && !true  || false`},
		"call_not_not":    {`!!true`, `  true`},
		"lit_quote_bytes": {`b'aaa"bbb'`, `b"\141\141\141\042\142\142\142"`},
		"select":          {`a . b . c`, `a .b  .c`},
	}

	for name, in := range tests {
		t.Run(name, func(tt *testing.T) {
			p, iss := Parse(common.NewTextSource(in[0]))
			if len(iss.GetErrors()) > 0 {
				tt.Fatal(iss.ToDisplayString())
			}
			out, err := Unparse(p.GetExpr(), p.GetSourceInfo())
			if err != nil {
				tt.Error(err)
			}
			if out != in[1] {
				tt.Errorf("Got '%s', wanted '%s'", out, in[1])
			}
			p2, _ := Parse(common.NewTextSource(out))
			before := p.GetExpr()
			after := p2.GetExpr()
			if !proto.Equal(before, after) {
				tt.Errorf("Second parse differs from the first. Got '%v', wanted '%v'",
					before, after)
			}
		})
	}
}
