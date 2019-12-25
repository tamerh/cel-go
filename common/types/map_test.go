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

	"github.com/tamerh/cel-go/common/types/traits"

	structpb "github.com/golang/protobuf/ptypes/struct"
)

type testStruct struct {
	M       string
	Details []string
	private string
}

func TestBaseMap_Contains(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]map[int32]float32{
		"nested": {1: -1.0, 2: 2.0},
		"empty":  {}}).(traits.Mapper)
	if mapValue.Contains(String("nested")) != True {
		t.Error("Expected key 'nested' contained in map.")
	}
	if mapValue.Contains(String("unknown")) != False {
		t.Error("Expected key 'unknown' not contained in map.")
	}
	if !IsError(mapValue.Contains(Int(123))) {
		t.Error("Expected key of Int type would error with 'no such overload'.")
	}
	if !reflect.DeepEqual(mapValue.Contains(Unknown{1}), Unknown{1}) {
		t.Error("Expected Unknown key in would yield Unknown key out.")
	}
}

func TestStringMap_Contains(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewStringStringMap(reg, map[string]string{
		"first":  "hello",
		"second": "world"}).(traits.Mapper)
	if mapValue.Contains(String("first")) != True {
		t.Error("Expected key 'first' contained in map.")
	}
	if mapValue.Contains(String("third")) != False {
		t.Error("Expected key 'third' not contained in map.")
	}
	if !IsError(mapValue.Contains(Int(123))) {
		t.Error("Expected key of Int type would error with 'no such overload'.")
	}
	if !reflect.DeepEqual(mapValue.Contains(Unknown{1}), Unknown{1}) {
		t.Error("Expected Unknown key in would yield Unknown key out.")
	}
}

func TestBaseMap_ConvertToNative_Any(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]map[string]float32{
		"nested": {"1": -1.0}})
	val, err := mapValue.ConvertToNative(anyValueType)
	if err != nil {
		t.Error(err)
	}
	jsonMap := &structpb.Struct{}
	if jsonpb.UnmarshalString(`{"nested":{"1":-1}}`, jsonMap) != nil {
		t.Error("Unable to unmarshal json to protobuf.Value")
	}
	want, err := ptypes.MarshalAny(jsonMap)
	if err != nil {
		t.Error(err)
	}
	if !proto.Equal(val.(proto.Message), want) {
		t.Errorf("Got %v, wanted %v", val, want)
	}
}

func TestBaseMap_ConvertToNative_Error(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]map[string]float32{
		"nested": {"1": -1.0}})
	val, err := mapValue.ConvertToNative(reflect.TypeOf(""))
	if err == nil {
		t.Errorf("Got '%v', expected error", val)
	}
}

func TestBaseMap_ConvertToNative_Json(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]map[string]float32{
		"nested": {"1": -1.0}})
	json, err := mapValue.ConvertToNative(jsonValueType)
	if err != nil {
		t.Error(err)
	}
	jsonTxt, _ := (&jsonpb.Marshaler{}).MarshalToString(json.(proto.Message))
	if jsonTxt != `{"nested":{"1":-1}}` {
		t.Error(jsonTxt)
	}
}

func TestBaseMap_ConvertToNative_Struct(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]interface{}{
		"m":       "hello",
		"details": []string{"world", "universe"},
	})
	ts, err := mapValue.ConvertToNative(reflect.TypeOf(testStruct{}))
	if err != nil {
		t.Error(err)
	}
	want := testStruct{M: "hello", Details: []string{"world", "universe"}}
	if !reflect.DeepEqual(ts, want) {
		t.Errorf("Got %v, wanted %v", ts, want)
	}
}

func TestBaseMap_ConvertToNative_StructPtr(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]interface{}{
		"m":       "hello",
		"details": []string{"world", "universe"},
	})
	ts, err := mapValue.ConvertToNative(reflect.TypeOf(&testStruct{}))
	if err != nil {
		t.Error(err)
	}
	want := &testStruct{M: "hello", Details: []string{"world", "universe"}}
	if !reflect.DeepEqual(ts, want) {
		t.Errorf("Got %v, wanted %v", ts, want)
	}
}

func TestBaseMap_ConvertToNative_StructPtrPtr(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]interface{}{
		"m":       "hello",
		"details": []string{"world", "universe"},
	})
	ptr := &testStruct{}
	ts, err := mapValue.ConvertToNative(reflect.TypeOf(&ptr))
	if err == nil {
		t.Errorf("Got %v, wanted error", ts)
	}
}

func TestBaseMap_ConvertToNative_Struct_InvalidFieldError(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]interface{}{
		"m":       "hello",
		"details": []string{"world", "universe"},
		"invalid": "invalid field",
	})
	ts, err := mapValue.ConvertToNative(reflect.TypeOf(&testStruct{}))
	if err == nil {
		t.Errorf("Got %v, wanted error", ts)
	}
}

func TestBaseMap_ConvertToNative_Struct_EmptyFieldError(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]interface{}{
		"m":       "hello",
		"details": []string{"world", "universe"},
		"":        "empty field",
	})
	ts, err := mapValue.ConvertToNative(reflect.TypeOf(&testStruct{}))
	if err == nil {
		t.Errorf("Got %v, wanted error", ts)
	}
}

func TestBaseMap_ConvertToNative_Struct_PrivateFieldError(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]interface{}{
		"message": "hello",
		"details": []string{"world", "universe"},
		"private": "private field",
	})
	ts, err := mapValue.ConvertToNative(reflect.TypeOf(&testStruct{}))
	if err == nil {
		t.Errorf("Got %v, wanted error", ts)
	}
}

func TestStringMap_ConvertToNative_Json(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewStringStringMap(reg, map[string]string{
		"first":  "hello",
		"second": "world"}).(traits.Mapper)
	json, err := mapValue.ConvertToNative(jsonStructType)
	if err != nil {
		t.Error(err)
	}
	jsonTxt, _ := (&jsonpb.Marshaler{}).MarshalToString(json.(proto.Message))
	if jsonTxt != `{"first":"hello","second":"world"}` {
		t.Error(jsonTxt)
	}
}

func TestBaseMap_ConvertToType(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]string{"key": "value"})
	if mapValue.ConvertToType(MapType) != mapValue {
		t.Error("Map could not be converted to a map.")
	}
	if mapValue.ConvertToType(TypeType) != MapType {
		t.Error("Map type was not listed as a map.")
	}
	if !IsError(mapValue.ConvertToType(ListType)) {
		t.Error("Map conversion to unsupported type was not an error.")
	}
}

func TestStringMap_ConvertToType(t *testing.T) {
	reg := NewRegistry()
	mapValue := reg.NativeToValue(map[string]string{"key": "value"})
	if mapValue.ConvertToType(MapType) != mapValue {
		t.Error("Map could not be converted to a map.")
	}
	if mapValue.ConvertToType(TypeType) != MapType {
		t.Error("Map type was not listed as a map.")
	}
	if !IsError(mapValue.ConvertToType(ListType)) {
		t.Error("Map conversion to unsupported type was not an error.")
	}
}

func TestBaseMap_Equal_True(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]map[int32]float32{
		"nested": {1: -1.0, 2: 2.0},
		"empty":  {}})
	if mapValue.Equal(mapValue) != True {
		t.Error("Map value was not equal to itself")
	}
	if nestedVal := mapValue.Get(String("nested")); IsError(nestedVal) {
		t.Error(nestedVal)
	} else if mapValue.Equal(nestedVal) == True ||
		nestedVal.Equal(mapValue) == True {
		t.Error("Same length, but different key names did not result in error")
	}
}

func TestStringMap_Equal_True(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewStringStringMap(reg, map[string]string{
		"first":  "hello",
		"second": "world"})
	if mapValue.Equal(mapValue) != True {
		t.Error("Map value was not equal to itself")
	}
	equivDyn := NewDynamicMap(reg, map[string]string{
		"second": "world",
		"first":  "hello"})
	if mapValue.Equal(equivDyn) != True {
		t.Error("Map value equality was key-order dependent")
	}
	equivJSON := NewJSONStruct(reg, &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"first":  {Kind: &structpb.Value_StringValue{StringValue: "hello"}},
			"second": {Kind: &structpb.Value_StringValue{StringValue: "world"}},
		}})
	if mapValue.Equal(equivJSON) != True && equivJSON.Equal(mapValue) != True {
		t.Error("Map value was not equivalent to json struct")
	}
}

func TestBaseMap_Equal_NotTrue(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]map[int32]float32{
		"nested": {1: -1.0, 2: 2.0},
		"empty":  {}})
	other := NewDynamicMap(reg, map[string]map[int64]float64{
		"nested": {1: -1.0, 2: 2.0, 3: 3.14},
		"empty":  {}})
	if mapValue.Equal(other) != False {
		t.Error("Inequal map values were deemed equal.")
	}
	other = NewDynamicMap(reg, map[string]map[int64]float64{
		"nested": {1: -1.0, 2: 2.0, 3: 3.14},
		"absent": {}})
	if mapValue.Equal(other) != False {
		t.Error("Inequal map keys were deemed equal.")
	}
}

func TestStringMap_Equal_NotTrue(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewStringStringMap(reg, map[string]string{
		"first":  "hello",
		"second": "world"})
	if mapValue.Equal(mapValue) != True {
		t.Error("Map value was not equal to itself")
	}
	other := NewStringStringMap(reg, map[string]string{
		"second": "world",
		"first":  "goodbye"})
	if mapValue.Equal(other) != False {
		t.Error("Map of same size with same keys and different values not false")
	}
	other = NewStringStringMap(reg, map[string]string{
		"first": "hello"})
	if mapValue.Equal(other) != False {
		t.Error("Equality between maps of different size did not return false")
	}
	other = NewStringStringMap(reg, map[string]string{
		"first": "hello",
		"third": "goodbye"})
	if mapValue.Equal(other) != False {
		t.Error("Equality between maps of different size did not return false")
	}
	other = NewDynamicMap(reg, map[string]interface{}{
		"first":  "hello",
		"second": 1})
	if !IsError(mapValue.Equal(other)) {
		t.Error("Equality between maps of different value types did not error")
	}
}

func TestBaseMap_Get(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]map[int32]float32{
		"nested": {1: -1.0, 2: 2.0},
		"empty":  {}}).(traits.Mapper)
	if nestedVal := mapValue.Get(String("nested")); IsError(nestedVal) {
		t.Error(nestedVal)
	} else if floatVal := nestedVal.(traits.Indexer).Get(Int(1)); IsError(floatVal) {
		t.Error(floatVal)
	} else if floatVal.Equal(Double(-1.0)) != True {
		t.Error("Nested map access of float property not float64")
	}
	e, isError := mapValue.Get(String("absent")).(*Err)
	if !isError || e.Error() != "no such key: absent" {
		t.Errorf("Got %v, wanted no such key: absent.", e)
	}
}

func TestStringMap_Get(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewStringStringMap(reg, map[string]string{
		"first":  "hello",
		"second": "world"}).(traits.Mapper)
	val := mapValue.Get(String("first"))
	if val.Equal(String("hello")) != True {
		t.Errorf("Got '%v', wanted 'hello'", val)
	}
	if !IsError(mapValue.Get(Int(1))) {
		t.Error("Got real value, wanted error")
	}
	if !IsError(mapValue.Get(String("third"))) {
		t.Error("Got real value, wanted error")
	}
}

func TestBaseMap_Iterator(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]map[int32]float32{
		"nested": {1: -1.0, 2: 2.0},
		"empty":  {}}).(traits.Mapper)
	it := mapValue.Iterator()
	var i = 0
	var fieldNames []interface{}
	for ; it.HasNext() == True; i++ {
		fieldName := it.Next()
		if value := mapValue.Get(fieldName); IsError(value) {
			t.Error(value)
		} else {
			fieldNames = append(fieldNames, fieldName)
		}
	}
	if len(fieldNames) != 2 {
		t.Errorf("Did not find the correct number of fields: %v", fieldNames)
	}
	if it.Next() != nil {
		t.Error("Iterator ran off the end of the field names")
	}
}

func TestStringMap_Iterator(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewStringStringMap(reg, map[string]string{
		"first":  "hello",
		"second": "world"}).(traits.Mapper)
	it := mapValue.Iterator()
	var i = 0
	var fieldNames []interface{}
	for ; it.HasNext() == True; i++ {
		fieldName := it.Next()
		if value := mapValue.Get(fieldName); IsError(value) {
			t.Error(value)
		} else {
			fieldNames = append(fieldNames, fieldName)
		}
	}
	if len(fieldNames) != 2 {
		t.Errorf("Did not find the correct number of fields: %v", fieldNames)
	}
	fieldsMap := map[string]bool{
		"first":  false,
		"second": false,
	}
	expectedMap := map[string]bool{
		"first":  true,
		"second": true,
	}
	for _, fieldName := range fieldNames {
		key := string(fieldName.(String))
		if _, found := fieldsMap[key]; found {
			fieldsMap[key] = true
		}
	}
	if !reflect.DeepEqual(fieldsMap, expectedMap) {
		t.Errorf("Got '%v', wanted '%v'", fieldsMap, expectedMap)
	}
	if it.Next() != nil {
		t.Error("Iterator ran off the end of the field names")
	}
}

func TestBaseMap_Size(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewDynamicMap(reg, map[string]int{
		"first":  1,
		"second": 2}).(traits.Mapper)
	if mapValue.Size() != Int(2) {
		t.Errorf("Got '%v', expected 2", mapValue.Size())
	}
}

func TestStringMap_Size(t *testing.T) {
	reg := NewRegistry()
	mapValue := NewStringStringMap(reg, map[string]string{
		"first":  "hello",
		"second": "world"}).(traits.Mapper)
	if mapValue.Size() != Int(2) {
		t.Errorf("Got '%v', expected 2", mapValue.Size())
	}
}
