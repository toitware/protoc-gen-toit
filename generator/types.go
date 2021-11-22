// Copyright (C) 2021 Toitware ApS. All rights reserved.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package generator

import (
	"fmt"
	"strings"

	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
)

type referType struct {
	file   *descriptor.FileDescriptorProto
	parent *referType
	enum   *descriptor.EnumDescriptorProto
	msg    *descriptor.DescriptorProto
}

func (t *referType) Name() string {
	if t.parent != nil {
		return t.parent.Name() + "." + t.elementName()
	}

	if t.file == nil || t.file.Package == nil {
		return "." + t.elementName()
	}

	return "." + t.file.GetPackage() + "." + t.elementName()
}

func (t *referType) elementName() string {
	if t.enum != nil {
		return t.enum.GetName()
	}
	return t.msg.GetName()
}

func (t *referType) ToitType(importAlias string) string {
	var types []string
	parent := t.parent
	for parent != nil {
		types = append(types, parent.elementName())
		parent = parent.parent
	}
	res := toitClassName(t.elementName(), types...)
	if importAlias == "" {
		return res
	}
	return importAlias + "." + res
}

func toitClassName(typ string, super ...string) string {
	if len(super) == 0 {
		return typ
	}
	return strings.Join(super, "_") + "_" + typ
}

type oneofType struct {
	FieldName     string
	CaseName      string
	CaseGetter    string
	ClearFunction string
	CaseFields    map[int32]string
	Descriptor    *descriptor.OneofDescriptorProto
}

type fieldTypeClass int

const (
	fieldTypeClassPrimitive fieldTypeClass = iota
	fieldTypeClassList
	fieldTypeClassMap
	fieldTypeClassObject
)

type fieldType struct {
	g         *generator
	class     fieldTypeClass
	field     *descriptor.FieldDescriptorProto
	t         *referType
	valueType *fieldType
	keyType   *fieldType
}

func (f *fieldType) FieldName(oneofTypes []*oneofType) string {
	if f.field.OneofIndex == nil {
		return uniqueName(f.field.GetName(), reservedFieldNames, "_")
	}
	oneof := oneofTypes[f.field.GetOneofIndex()]
	return oneof.CaseFields[f.field.GetNumber()]
}

func (f *fieldType) DefaultValue() (string, error) {
	switch f.class {
	case fieldTypeClassPrimitive:
		switch f.field.GetType() {
		case descriptor.FieldDescriptorProto_TYPE_DOUBLE, descriptor.FieldDescriptorProto_TYPE_FLOAT:
			return "0.0", nil
		case descriptor.FieldDescriptorProto_TYPE_INT64, descriptor.FieldDescriptorProto_TYPE_UINT64,
			descriptor.FieldDescriptorProto_TYPE_INT32, descriptor.FieldDescriptorProto_TYPE_UINT32,
			descriptor.FieldDescriptorProto_TYPE_FIXED64, descriptor.FieldDescriptorProto_TYPE_FIXED32,
			descriptor.FieldDescriptorProto_TYPE_SFIXED64, descriptor.FieldDescriptorProto_TYPE_SFIXED32,
			descriptor.FieldDescriptorProto_TYPE_SINT64, descriptor.FieldDescriptorProto_TYPE_SINT32:
			return "0", nil
		case descriptor.FieldDescriptorProto_TYPE_BOOL:
			return "false", nil
		case descriptor.FieldDescriptorProto_TYPE_STRING:
			return `""`, nil
		case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
			importAlias, ok := f.g.imports[f.t.file.GetName()]
			if !ok {
				return "", fmt.Errorf("failed to find import alias for file: '%s' - object: '%s'", f.t.file.GetName(), f.t.Name())
			}
			return f.t.ToitType(importAlias), nil
		case descriptor.FieldDescriptorProto_TYPE_BYTES:
			return "ByteArray 0", nil
		case descriptor.FieldDescriptorProto_TYPE_ENUM:
			return "0", nil
		default:
			return "", fmt.Errorf("failed to find default value for object: %s, primitve: %s", f.field.GetName(), f.field.GetType())
		}
	case fieldTypeClassObject:
		if f.g.options.CoreObjects {
			if f.t.Name() == coreDurationMessage {
				return "_core.Duration.ZERO", nil
			}
			if f.t.Name() == coreTimestampMessage {
				return "_protobuf.TIME_ZERO_EPOCH", nil
			}
		}

		importAlias, ok := f.g.imports[f.t.file.GetName()]
		if !ok {
			return "", fmt.Errorf("failed to find import alias for file: '%s' - object: '%s'", f.t.file.GetName(), f.t.Name())
		}
		return f.t.ToitType(importAlias), nil
	case fieldTypeClassList:
		return "[]", nil
	case fieldTypeClassMap:
		return "{:}", nil
	}
	return "", fmt.Errorf("failed to find default value for object: %s", f.field.GetName())
}

func (f *fieldType) ToitTypeAnnotation(optional bool) (string, error) {
	return f.toitTypeAnnotation(optional, false)
}

func (f *fieldType) toitTypeAnnotation(optional, inComment bool) (string, error) {
	switch f.class {
	case fieldTypeClassPrimitive, fieldTypeClassObject:
		return FieldTypeToToitType(f.field.GetType(), optional, inComment, f.t, f.g)
	case fieldTypeClassList:
		v, err := f.valueType.toitTypeAnnotation(false, true)
		if err != nil {
			return "", err
		}
		if inComment {
			return optionalType("List<"+v+">", optional), nil
		}
		return optionalType("List", optional) + "/*<" + v + ">*/", nil
	case fieldTypeClassMap:
		k, err := f.keyType.toitTypeAnnotation(false, true)
		if err != nil {
			return "", err
		}
		v, err := f.valueType.toitTypeAnnotation(false, true)
		if err != nil {
			return "", err
		}
		if inComment {
			return optionalType("Map<"+k+","+v+">", optional), nil
		}
		return optionalType("Map", optional) + "/*<" + k + "," + v + ">*/", nil
	default:
		return "", fmt.Errorf("invalid fieldClass: %v", f.class)
	}
}

func optionalType(typ string, optional bool) string {
	if optional {
		return typ + "?"
	}
	return typ
}

func FieldTypeToToitType(ft descriptor.FieldDescriptorProto_Type, optional, inComment bool, t *referType, g *generator) (string, error) {
	switch ft {
	case descriptor.FieldDescriptorProto_TYPE_DOUBLE:
		return optionalType("float", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_FLOAT:
		return optionalType("float", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_INT64:
		return optionalType("int", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_UINT64:
		return optionalType("int", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_INT32:
		return optionalType("int", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_UINT32:
		return optionalType("int", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_FIXED64:
		return optionalType("int", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_FIXED32:
		return optionalType("int", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_BOOL:
		return optionalType("bool", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_STRING:
		return optionalType("string", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
		if g.options.CoreObjects {
			if t.Name() == coreDurationMessage {
				return optionalType("_core.Duration", optional), nil
			}
			if t.Name() == coreTimestampMessage {
				return optionalType("_core.Time", optional), nil
			}
		}
		importAlias, ok := g.imports[t.file.GetName()]
		if !ok {
			return "", fmt.Errorf("failed to find import alias for file: '%s' - object: '%s'", t.file.GetName(), t.Name())
		}
		return optionalType(t.ToitType(importAlias), optional), nil
	case descriptor.FieldDescriptorProto_TYPE_BYTES:
		return optionalType("ByteArray", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_ENUM:
		importAlias, ok := g.imports[t.file.GetName()]
		if !ok {
			return "", fmt.Errorf("failed to find import alias for file: '%s' - object: '%s'", t.file.GetName(), t.Name())
		}
		className := t.ToitType(importAlias)
		actual := optionalType("enum<"+className+">", optional)
		if inComment {
			return actual, nil
		}
		return optionalType("int", optional) + "/*" + actual + "*/", nil
	case descriptor.FieldDescriptorProto_TYPE_SFIXED32:
		return optionalType("int", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_SFIXED64:
		return optionalType("int", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_SINT32:
		return optionalType("int", optional), nil
	case descriptor.FieldDescriptorProto_TYPE_SINT64:
		return optionalType("int", optional), nil
	default:
		return "", fmt.Errorf("unkonwn type: %v", ft)
	}
}

func protobufTypeConst(ft descriptor.FieldDescriptorProto_Type) (string, error) {
	switch ft {
	case descriptor.FieldDescriptorProto_TYPE_DOUBLE:
		return "PROTOBUF_TYPE_DOUBLE", nil
	case descriptor.FieldDescriptorProto_TYPE_FLOAT:
		return "PROTOBUF_TYPE_FLOAT", nil
	case descriptor.FieldDescriptorProto_TYPE_INT64:
		return "PROTOBUF_TYPE_INT64", nil
	case descriptor.FieldDescriptorProto_TYPE_SINT64:
		return "PROTOBUF_TYPE_SINT64", nil
	case descriptor.FieldDescriptorProto_TYPE_SFIXED64:
		return "PROTOBUF_TYPE_SFIXED64", nil
	case descriptor.FieldDescriptorProto_TYPE_FIXED64:
		return "PROTOBUF_TYPE_FIXED64", nil
	case descriptor.FieldDescriptorProto_TYPE_UINT64:
		return "PROTOBUF_TYPE_UINT64", nil
	case descriptor.FieldDescriptorProto_TYPE_UINT32:
		return "PROTOBUF_TYPE_UINT32", nil
	case descriptor.FieldDescriptorProto_TYPE_INT32:
		return "PROTOBUF_TYPE_INT32", nil
	case descriptor.FieldDescriptorProto_TYPE_SINT32:
		return "PROTOBUF_TYPE_SINT32", nil
	case descriptor.FieldDescriptorProto_TYPE_SFIXED32:
		return "PROTOBUF_TYPE_SFIXED32", nil
	case descriptor.FieldDescriptorProto_TYPE_FIXED32:
		return "PROTOBUF_TYPE_FIXED32", nil
	case descriptor.FieldDescriptorProto_TYPE_BOOL:
		return "PROTOBUF_TYPE_BOOL", nil
	case descriptor.FieldDescriptorProto_TYPE_STRING:
		return "PROTOBUF_TYPE_STRING", nil
	case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
		return "PROTOBUF_TYPE_MESSAGE", nil
	case descriptor.FieldDescriptorProto_TYPE_ENUM:
		return "PROTOBUF_TYPE_ENUM", nil
	case descriptor.FieldDescriptorProto_TYPE_BYTES:
		return "PROTOBUF_TYPE_BYTES", nil
	default:
		return "", fmt.Errorf("unsupported protobuf type: %v", ft)
	}
}
