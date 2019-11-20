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

package pb

import (
	"testing"

	"github.com/golang/protobuf/proto"

	proto3pb "github.com/google/cel-go/test/proto3pb"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestTypeDescription_FieldCount(t *testing.T) {
	pbdb := NewDb()
	pbdb.RegisterMessage(&proto3pb.NestedTestAllTypes{})
	td, err := pbdb.DescribeType(proto.MessageName(&proto3pb.NestedTestAllTypes{}))
	if err != nil {
		t.Error(err)
	}
	if td.FieldCount() != 2 {
		t.Errorf("Unexpected field count. got '%d', wanted '%d'",
			td.FieldCount(), 2)
	}
}

func TestTypeDescription_Any(t *testing.T) {
	pbdb := NewDb()
	_, err := pbdb.DescribeType(".google.protobuf.Any")
	if err != nil {
		t.Error(err)
	}
}

func TestTypeDescription_Json(t *testing.T) {
	pbdb := NewDb()
	_, err := pbdb.DescribeType(".google.protobuf.Value")
	if err != nil {
		t.Error(err)
	}
}

func TestTypeDescription_JsonNotInTypeInit(t *testing.T) {
	pbdb := NewDb()
	_, err := pbdb.DescribeType(".google.protobuf.ListValue")
	if err != nil {
		t.Error(err)
	}
}

func TestTypeDescription_Wrapper(t *testing.T) {
	pbdb := NewDb()
	_, err := pbdb.DescribeType(".google.protobuf.BoolValue")
	if err != nil {
		t.Error(err)
	}
}

func TestTypeDescription_WrapperNotInTypeInit(t *testing.T) {
	pbdb := NewDb()
	if _, err := pbdb.DescribeType(".google.protobuf.BytesValue"); err != nil {
		t.Error(err)
	}
}

func TestTypeDescription_Field(t *testing.T) {
	pbdb := NewDb()
	msg := proto3pb.NestedTestAllTypes{}
	_, err := pbdb.RegisterMessage(&msg)
	if err != nil {
		t.Error(err)
	}
	td, err := pbdb.DescribeType(proto.MessageName(&msg))
	if err != nil {
		t.Error(err)
	}
	fd, found := td.FieldByName("payload")
	if !found {
		t.Error("Field 'payload' not found")
	}
	if fd.OrigName() != "payload" {
		t.Error("Unexpected proto name for field 'payload'", fd.OrigName())
	}
	if fd.Name() != "Payload" {
		t.Error("Unexpected struct name for field 'payload'", fd.Name())
	}
	if fd.IsOneof() {
		t.Error("Field payload is listed as a oneof and it is not.")
	}
	if fd.IsMap() {
		t.Error("Field 'payload' is listed as a map and it is not.")
	}
	if !fd.IsMessage() {
		t.Error("Field 'payload' is not marked as a message.")
	}
	if fd.IsEnum() {
		t.Error("Field 'payload' is marked as an enum.")
	}
	if fd.IsRepeated() {
		t.Error("Field 'payload' is marked as repeated.")
	}
	// Access the field by its Go struct name and check to see that it's index
	// matches the one determined by the TypeDescription utils.
	if !proto.Equal(fd.CheckedType(), &exprpb.Type{
		TypeKind: &exprpb.Type_MessageType{
			MessageType: "google.expr.proto3.test.TestAllTypes"}}) {
		t.Error("Field 'payload' had an unexpected checked type.")
	}
}
