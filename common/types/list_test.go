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
	"reflect"
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"

	"github.com/tamerh/cel-go/common/types/ref"
	"github.com/tamerh/cel-go/common/types/traits"

	dpb "github.com/golang/protobuf/ptypes/duration"
	structpb "github.com/golang/protobuf/ptypes/struct"
)

func TestBaseList_Add_Empty(t *testing.T) {
	reg := NewRegistry()
	list := NewDynamicList(reg, []bool{true})
	if list.Add(NewDynamicList(reg, []bool{})) != list {
		t.Error("Adding an empty list created new list.")
	}
	if NewDynamicList(reg, []string{}).Add(list) != list {
		t.Error("Adding list to empty created a new list.")
	}
}

func TestBaseList_Add_Error(t *testing.T) {
	if !IsError(NewDynamicList(NewRegistry(), []bool{}).Add(String("error"))) {
		t.Error("Addind a non-list value to a list unexpected succeeds.")
	}
}

func TestBaseList_Contains(t *testing.T) {
	list := NewDynamicList(NewRegistry(), []float32{1.0, 2.0, 3.0})
	if list.Contains(Double(5)) != False {
		t.Error("List contains did not return false")
	}
	if list.Contains(Double(3)) != True {
		t.Error("List contains did not succeed")
	}
	list = NewDynamicList(NewRegistry(), []interface{}{1.0, 2, 3.0})
	if list.Contains(Int(2)) != True {
		t.Error("List contains did not succeed")
	}
	if list.Contains(Double(3)) != True {
		t.Error("List contains did not succeed")
	}
}

func TestBaseList_Contains_NonBool(t *testing.T) {
	list := NewDynamicList(NewRegistry(), []interface{}{1.0, 2, 3.0})
	if !IsError(list.Contains(Int(3))) {
		t.Error("List contains succeeded with wrong type")
	}
	if !reflect.DeepEqual(list.Contains(Unknown{1}), Unknown{1}) {
		t.Error("List ")
	}
}

func TestBaseList_ConvertToNative(t *testing.T) {
	list := NewDynamicList(NewRegistry(), []float64{1.0, 2.0})
	if protoList, err := list.ConvertToNative(reflect.TypeOf([]float32{})); err != nil {
		t.Error(err)
	} else if !reflect.DeepEqual(protoList, []float32{1.0, 2.0}) {
		t.Errorf("Could not convert to []float32: %v", protoList)
	}
}

func TestBaseList_ConvertToNative_Any(t *testing.T) {
	list := NewDynamicList(NewRegistry(), []float64{1.0, 2.0})
	val, err := list.ConvertToNative(anyValueType)
	if err != nil {
		t.Error(err)
	}
	jsonVal := &structpb.ListValue{}
	if jsonpb.UnmarshalString("[1.0, 2.0]", jsonVal) != nil {
		t.Error("Unable to unmarshal json")
	}
	want, err := ptypes.MarshalAny(jsonVal)
	if err != nil {
		t.Error(err)
	}
	if !proto.Equal(val.(proto.Message), want) {
		t.Errorf("Got %v, wanted %v", val, want)
	}
}

func TestBaseList_ConvertToNative_Json(t *testing.T) {
	list := NewDynamicList(NewRegistry(), []float64{1.0, 2.0})
	val, err := list.ConvertToNative(jsonListValueType)
	if err != nil {
		t.Error(err)
	}
	want := &structpb.ListValue{}
	if jsonpb.UnmarshalString("[1.0, 2.0]", want) != nil {
		t.Error("Unable to unmarshal json")
	}
	if !proto.Equal(val.(proto.Message), want) {
		t.Errorf("Got %v, wanted %v", val, want)
	}
}

func TestBaseList_ConvertToType(t *testing.T) {
	list := NewDynamicList(NewRegistry(), []string{"h", "e", "l", "l", "o"})
	if list.ConvertToType(ListType) != list {
		t.Error("List was not convertible to itself.")
	}
	if list.ConvertToType(TypeType) != ListType {
		t.Error("Unable to obtain the proper type from the list.")
	}
	if !IsError(list.ConvertToType(MapType)) {
		t.Error("List was able to convert to unexpected type.")
	}
}

func TestBaseList_Equal(t *testing.T) {
	listA := NewDynamicList(NewRegistry(), []string{"h", "e", "l", "l", "o"})
	listB := NewDynamicList(NewRegistry(), []string{"h", "e", "l", "p", "!"})
	if listA.Equal(listB) != False {
		t.Error("Lists with different contents returned equal.")
	}
}

func TestBaseList_Get(t *testing.T) {
	validateList123(t, NewDynamicList(NewRegistry(), []int32{1, 2, 3}).(traits.Lister))
}

func TestValueList_Get(t *testing.T) {
	validateList123(t, NewValueList(NewRegistry(), []ref.Val{Int(1), Int(2), Int(3)}))
}

func TestBaseList_Iterator(t *testing.T) {
	validateIterator123(t, NewDynamicList(NewRegistry(), []int32{1, 2, 3}).(traits.Lister))
}

func TestValueListValue_Iterator(t *testing.T) {
	validateIterator123(t, NewValueList(NewRegistry(), []ref.Val{Int(1), Int(2), Int(3)}))
}

func TestBaseList_NestedList(t *testing.T) {
	reg := NewRegistry()
	listUint32 := []uint32{1, 2}
	nestedUint32 := NewDynamicList(reg, []interface{}{listUint32})
	listUint64 := []uint64{1, 2}
	nestedUint64 := NewDynamicList(reg, []interface{}{listUint64})
	if nestedUint32.Equal(nestedUint64) != True {
		t.Error("Could not find nested list")
	}
	if nestedUint32.Contains(NewDynamicList(reg, listUint64)) != True ||
		nestedUint64.Contains(NewDynamicList(reg, listUint32)) != True {
		t.Error("Could not find type compatible nested lists")
	}
}

func TestBaseList_Size(t *testing.T) {
	reg := NewRegistry()
	listUint32 := []uint32{1, 2}
	nestedUint32 := NewDynamicList(reg, []interface{}{listUint32})
	if nestedUint32.Size() != IntOne {
		t.Error("List indicates the incorrect size.")
	}
	if nestedUint32.Get(IntZero).(traits.Sizer).Size() != Int(2) {
		t.Error("Nested list indicates the incorrect size.")
	}
}

func TestConcatList_Add(t *testing.T) {
	reg := NewRegistry()
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewStringList(reg, []string{"3"})
	list := listA.Add(listB).(traits.Lister).Add(listA).
		Value().([]interface{})
	expected := []interface{}{
		float64(1.0),
		float64(2.0),
		string("3"),
		float64(1.0),
		float64(2.0)}
	if len(list) != len(expected) {
		t.Errorf("Got '%v', expected '%v'", list, expected)
	} else {
		for i := 0; i < len(list); i++ {
			if expected[i] != list[i] {
				t.Errorf("elem[%d] Got '%v', expected '%v'",
					i, list[i], expected[i])
			}
		}
	}
	// Zero length input list
	listConcat := listA.Add(listB).(traits.Lister)
	same := listConcat.Add(NewStringList(reg, []string{}))
	if !reflect.DeepEqual(listConcat, same) {
		t.Error("Adding an empty list to a concat list did not return the concat list")
	}
	// Zero length operand list
	same = NewDynamicList(reg, []bool{}).Add(listConcat)
	if !reflect.DeepEqual(listConcat, same) {
		t.Error("Adding a concat list to an empty list did not return the concat list")
	}
}

func TestConcatList_ConvertToNative_Json(t *testing.T) {
	reg := NewRegistry()
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []string{"3"})
	list := listA.Add(listB)
	json, err := list.ConvertToNative(jsonValueType)
	if err != nil {
		t.Errorf("Got '%v', expected '%v'", err, json)
	}
	jsonTxt, err := (&jsonpb.Marshaler{}).MarshalToString(json.(proto.Message))
	if err != nil {
		t.Error(err)
	}
	if jsonTxt != "[1,2,\"3\"]" {
		t.Errorf("Got '%v', expected [1,2,\"3\"]", jsonTxt)
	}
	// Test proto3 to JSON conversion.
	listC := NewDynamicList(reg, []*dpb.Duration{{Seconds: 100}})
	listConcat := listA.Add(listC)
	json, err = listConcat.ConvertToNative(jsonValueType)
	if err != nil {
		t.Error(err)
	}
	jsonTxt, err = (&jsonpb.Marshaler{}).MarshalToString(json.(proto.Message))
	if err != nil {
		t.Error(err)
	}
	if jsonTxt != "[1,2,\"100s\"]" {
		t.Errorf("Got '%v', expected [1,2,\"100s\"]", jsonTxt)
	}
}

func TestConcatList_ConvertToNative_ListInterface(t *testing.T) {
	reg := NewRegistry()
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewStringList(reg, []string{"3.0"})
	list := listA.Add(listB)
	iface, err := list.ConvertToNative(reflect.TypeOf([]interface{}{}))
	if err != nil {
		t.Errorf("Got '%v', expected '%v'", err, list)
	}
	if !reflect.DeepEqual(iface, []interface{}{1.0, 2.0, "3.0"}) {
		t.Errorf("Got '%v', expected '%v'", iface, []interface{}{1.0, 2.0, "3.0"})
	}
}

func TestConcatList_ConvertToType(t *testing.T) {
	reg := NewRegistry()
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []*dpb.Duration{{Seconds: 100}})
	list := listA.Add(listB)
	if list.ConvertToType(ListType) != list {
		t.Error("List conversion to list failed.")
	}
	if list.ConvertToType(TypeType) != ListType {
		t.Error("List conversion to type failed.")
	}
	if !IsError(list.ConvertToType(MapType)) {
		t.Error("List conversion to map unexpectedly succeeded.")
	}
}

func TestConcatList_Contains(t *testing.T) {
	reg := NewRegistry()
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []string{"3"})
	listConcat := listA.Add(listB).(traits.Lister)
	if listConcat.Contains(String("3")) != True {
		t.Error("Concatenated list did not contain value in 'next' list.")
	}
	if listConcat.Contains(Double(2.0)) != True {
		t.Error("Concatenated list did not contain value in 'prev' list.")
	}
	homogList := NewDynamicList(reg, []string{"3"}).Add(
		NewStringList(reg, []string{"2", "1"})).(traits.Lister)
	if homogList.Contains(String("4")) != False {
		t.Error("Concatenated homogeneous list did not return false.")
	}
}

func TestConcatList_Contains_NonBool(t *testing.T) {
	reg := NewRegistry()
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []string{"3"})
	listConcat := listA.Add(listB).(traits.Lister)
	if !IsError(listConcat.Contains(String("4"))) {
		t.Error("Contains did not error with list of mixed types an not found input.")
	}
}

func TestConcatListValue_Equal(t *testing.T) {
	reg := NewRegistry()
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []float64{3.0})
	list := listA.Add(listB)
	// Note the internal type of list raw and concat list are slightly different.
	listRaw := NewDynamicList(reg, []interface{}{
		float32(1.0), float64(2.0), float64(3.0)})
	if listRaw.Equal(list) != True ||
		list.Equal(listRaw) != True {
		t.Errorf("Concat list and raw list were not equal, got '%v', expected '%v'",
			list.Value(),
			listRaw.Value())
	}
	if list.Equal(listA) == True ||
		listRaw.Equal(listA) == True {
		t.Errorf("Lists of unequal length considered equal")
	}
}

func TestConcatListValue_Get(t *testing.T) {
	reg := NewRegistry()
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []float64{3.0})
	list := listA.Add(listB).(traits.Lister)
	if getElem(t, list, 0) != Double(1.0) ||
		getElem(t, list, 1) != Double(2.0) ||
		getElem(t, list, 2) != Double(3.0) {
		t.Errorf("List values by index did not match expectations")
	}
	if val := list.Get(Int(-1)); !IsError(val) {
		t.Errorf("Should not have been able to read a negative index")
	}
	if val := list.Get(Int(3)); !IsError(val) {
		t.Errorf("Should not have been able to read beyond end of list")
	}
}

func TestConcatListValue_Iterator(t *testing.T) {
	reg := NewRegistry()
	listA := NewDynamicList(reg, []float32{1.0, 2.0})
	listB := NewDynamicList(reg, []float64{3.0})
	list := listA.Add(listB).(traits.Lister)
	it := list.Iterator()
	var i = int64(0)
	for ; it.HasNext() == True; i++ {
		elem := it.Next()
		if getElem(t, list, Int(i)) != elem {
			t.Errorf(
				"List iterator returned incorrect value: list[%d]: %v", i, elem)
		}
	}
	if it.Next() != nil {
		t.Errorf("List iterator attempted to continue beyond list size")
	}
	if i != 3 {
		t.Errorf("Iterator did not iterate until last value")
	}
}

func TestStringList_Add_Empty(t *testing.T) {
	reg := NewRegistry()
	list := NewStringList(reg, []string{"hello"})
	if list.Add(NewStringList(reg, []string{})) != list {
		t.Error("Adding empty lists resulted in new list creation.")
	}
	if NewStringList(reg, []string{}).Add(list) != list {
		t.Error("Adding empty lists resulted in new list creation.")
	}
}

func TestStringList_Add_Error(t *testing.T) {
	reg := NewRegistry()
	if !IsError(NewStringList(reg, []string{}).Add(True)) {
		t.Error("Got list, expected error.")
	}
}

func TestStringList_Add_Heterogenous(t *testing.T) {
	reg := NewRegistry()
	listA := NewStringList(reg, []string{"hello"})
	listB := NewDynamicList(reg, []int32{1, 2, 3})
	list := listA.Add(listB).(traits.Lister).Value().([]interface{})
	expected := []interface{}{"hello", int64(1), int64(2), int64(3)}
	if len(list) != len(expected) {
		t.Errorf("Unexpected list size. Got '%d', expected 4", len(list))
	}
	for i, v := range expected {
		if list[i] != v {
			t.Errorf("elem[%d] Got '%v', expected '%v'", i, list[i], v)
		}
	}
}

func TestStringList_Add_StringLists(t *testing.T) {
	reg := NewRegistry()
	listA := NewStringList(reg, []string{"hello"})
	listB := NewStringList(reg, []string{"world", "!"})
	list := listA.Add(listB).(traits.Lister)
	if list.Size() != Int(3) {
		t.Error("Combined list did not have correct size.")
	}
	expected := []string{"hello", "world", "!"}
	for i, v := range expected {
		if list.Get(Int(i)).Equal(String(v)) != True {
			t.Errorf("elem[%d] Got '%v', expected '%v'", i, list.Get(Int(i)), v)
		}
	}
}

func TestStringList_ConvertToNative(t *testing.T) {
	reg := NewRegistry()
	list := NewStringList(reg, []string{"h", "e", "l", "p"})
	val, err := list.ConvertToNative(reflect.TypeOf([]string{}))
	if err != nil {
		t.Error("Unable to convert string list to itself.")
	}
	if !reflect.DeepEqual(val, []string{"h", "e", "l", "p"}) {
		t.Errorf(`Got %v, expected ["h", "e", "l", "p"]`, val)
	}
}

func TestStringList_ConvertToNative_ListInterface(t *testing.T) {
	reg := NewRegistry()
	list := NewStringList(reg, []string{"h", "e", "l", "p"})
	val, err := list.ConvertToNative(reflect.TypeOf([]interface{}{}))
	if err != nil {
		t.Error("Unable to convert string list to itself.")
	}
	if !reflect.DeepEqual(val, []interface{}{"h", "e", "l", "p"}) {
		t.Errorf(`Got %v, expected ["h", "e", "l", "p"]`, val)
	}
}

func TestStringList_ConvertToNative_Error(t *testing.T) {
	reg := NewRegistry()
	list := NewStringList(reg, []string{"h", "e", "l", "p"})
	_, err := list.ConvertToNative(jsonStructType)
	if err == nil {
		t.Error("Conversion of list to unsupported type did not error.")
	}
}

func TestStringList_ConvertToNative_Json(t *testing.T) {
	reg := NewRegistry()
	list := NewStringList(reg, []string{"h", "e", "l", "p"})
	json, err := list.ConvertToNative(jsonValueType)
	if err != nil {
		t.Errorf("Got '%v', expected '%v'", err, json)
	}
	jsonTxt, err := (&jsonpb.Marshaler{}).MarshalToString(json.(proto.Message))
	if err != nil {
		t.Error(err)
	}
	if jsonTxt != "[\"h\",\"e\",\"l\",\"p\"]" {
		t.Errorf("Got '%v', expected [\"h\",\"e\",\"l\",\"p\"]", jsonTxt)
	}

	jsonList, err := list.ConvertToNative(jsonListValueType)
	if err != nil {
		t.Errorf("Got '%v', expected '%v'", err, jsonList)
	}
	jsonListTxt, _ := (&jsonpb.Marshaler{}).MarshalToString(jsonList.(proto.Message))
	if jsonTxt != jsonListTxt {
		t.Errorf("Json value and list value not equal.")
	}
}

func TestStringList_Get_OutOfRange(t *testing.T) {
	reg := NewRegistry()
	list := NewStringList(reg, []string{"hello", "world"})
	if !IsError(list.Get(Int(-1))) {
		t.Error("Negative index did not return error.")
	}
	if !IsError(list.Get(Int(2))) {
		t.Error("Index out of range did not return error.")
	}
	if !IsError(list.Get(Uint(1))) {
		t.Error("Invalid index type did not return error.")
	}
}

func TestValueList_Add(t *testing.T) {
	reg := NewRegistry()
	listA := NewValueList(reg, []ref.Val{String("hello")})
	listB := NewValueList(reg, []ref.Val{String("world")})
	listConcat := listA.Add(listB).(traits.Lister)
	if listConcat.Contains(String("goodbye")) != False {
		t.Error("Homogeneous concatenated value list did not return false on missing input")
	}
	if listConcat.Contains(String("hello")) != True {
		t.Error("Homogeneous concatenated value list did not return true on valid input")
	}
}

func TestValueList_ConvertToNative_Json(t *testing.T) {
	reg := NewRegistry()
	list := NewValueList(reg, []ref.Val{String("hello"), String("world")})
	json, err := list.ConvertToNative(jsonListValueType)
	if err != nil {
		t.Errorf("Got '%v', expected '%v'", err, json)
	}
	jsonTxt, err := (&jsonpb.Marshaler{}).MarshalToString(json.(proto.Message))
	if err != nil {
		t.Error(err)
	}
	if jsonTxt != `["hello","world"]` {
		t.Errorf(`Got '%v', expected ["hello","world"]`, jsonTxt)
	}
}

func getElem(t *testing.T, list traits.Indexer, index Int) interface{} {
	t.Helper()
	val := list.Get(index)
	if IsError(val) {
		t.Errorf("Error reading list index %d, %v", index, val)
		return nil
	}
	return val
}

func validateList123(t *testing.T, list traits.Lister) {
	t.Helper()
	if getElem(t, list, 0) != Int(1) ||
		getElem(t, list, 1) != Int(2) ||
		getElem(t, list, 2) != Int(3) {
		t.Errorf("List values by index did not match expectations")
	}
	if val := list.Get(Int(-1)); !IsError(val) {
		t.Errorf("Should not have been able to read a negative index")
	}
	if val := list.Get(Int(3)); !IsError(val) {
		t.Errorf("Should not have been able to read beyond end of list")
	}
	if !IsError(list.Get(Uint(3))) {
		t.Error("Invalid index type did not result in error")
	}
}

func validateIterator123(t *testing.T, list traits.Lister) {
	t.Helper()
	it := list.Iterator()
	var i = int64(0)
	for ; it.HasNext() == True; i++ {
		elem := it.Next()
		if getElem(t, list, Int(i)) != elem {
			t.Errorf(
				"List iterator returned incorrect value: list[%d]: %v", i, elem)
		}
	}
	if it.Next() != nil {
		t.Errorf("List iterator attempted to continue beyond list size")
	}
	if i != 3 {
		t.Errorf("Iterator did not iterate until last value")
	}
}
