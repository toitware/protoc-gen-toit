// Code generated by protoc-gen-toit. DO NOT EDIT.
// source: oneof.proto

import encoding.protobuf as _protobuf
import core as _core

// MESSAGE START: .MessageWithOneOf
class MessageWithOneOf extends _protobuf.Message:
  // ONEOF START: .MessageWithOneOf.value
  value_ := null
  value_oneof_case_/int? := null

  value_oneof_clear -> none:
    value_ = null
    value_oneof_case_ = null

  static VALUE_I/int ::= 1
  static VALUE_S/int ::= 2

  value_oneof_case -> int?:
    return value_oneof_case_

  value_i -> int:
    return value_

  value_i= value/int -> none:
    value_ = value
    value_oneof_case_ = VALUE_I

  value_s -> string:
    return value_

  value_s= value/string -> none:
    value_ = value
    value_oneof_case_ = VALUE_S

  // ONEOF END: .MessageWithOneOf.value

  constructor
      --value_i/int?=null
      --value_s/string?=null:
    if value_i != null:
      this.value_i = value_i
    if value_s != null:
      this.value_s = value_s

  constructor.deserialize r/_protobuf.Reader:
    r.read_message:
      r.read_field 1:
        value_i = r.read_primitive _protobuf.PROTOBUF_TYPE_UINT32
      r.read_field 2:
        value_s = r.read_primitive _protobuf.PROTOBUF_TYPE_STRING

  serialize w/_protobuf.Writer --as_field/int?=null --oneof/bool=false -> none:
    w.write_message_header this --as_field=as_field --oneof=oneof
    if value_oneof_case_ == VALUE_I:
      w.write_primitive _protobuf.PROTOBUF_TYPE_UINT32 value_ --as_field=VALUE_I --oneof
    if value_oneof_case_ == VALUE_S:
      w.write_primitive _protobuf.PROTOBUF_TYPE_STRING value_ --as_field=VALUE_S --oneof

  num_fields_set -> int:
    return (value_oneof_case_ == null ? 0 : 1)

  protobuf_size -> int:
    return (value_oneof_case_ == VALUE_I ? (_protobuf.size_primitive _protobuf.PROTOBUF_TYPE_UINT32 value_i --as_field=1) : 0)
      + (value_oneof_case_ == VALUE_S ? (_protobuf.size_primitive _protobuf.PROTOBUF_TYPE_STRING value_s --as_field=2) : 0)

// MESSAGE END: .MessageWithOneOf

