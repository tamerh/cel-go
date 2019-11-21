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
	// TODO: make a specific case here.
}

func TestAttributes_ConditionalAttr(t *testing.T) {
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
	// Test cond == true first.
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

	// Test cond == false second.
	// Recreate the attributes, since they are mutable.
	tv = AbsoluteAttribute(2, []string{"a"})
	fv = OneofAttribute(3, []string{"b"})
	fv.Qualify(4, "c")
	cond = ConditionalAttribute(1, &evalConst{val: types.False}, tv, fv)
	cond.Qualify(5, int64(-1))
	cond.Qualify(6, int64(1))
	out, err = cond.Resolve(vars, res)
	if err != nil {
		t.Fatal(err)
	}
	if out != uint(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}

	// TODO: test the error and unknown cases.
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
