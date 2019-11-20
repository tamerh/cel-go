package interpreter

import (
	"testing"
)

func TestAttributes_AbsoluteAttr(t *testing.T) {

}

func TestAttributes_RelativeAttr(t *testing.T) {

}

func TestAttributes_OneofAttr(t *testing.T) {

}

func TestAttributes_ConditionalAttr(t *testing.T) {

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
