// Copyright (C) 2021 Toitware ApS. All rights reserved.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package generator

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	plugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
	"github.com/toitware/protoc-gen-toit/toit"
	"github.com/toitware/protoc-gen-toit/util"
)

const (
	// constructor_initializers (bool), if set, will add field initializers to constructors
	constructorInitializersParam = "constructor_initializers"
	// importLibraryParam (key1=value1(;key2=value2)*), will change the library import path as prefix for the current one.
	importLibraryParam = "import_library"
	// convert_hooks (bool), if set, will import as relative paths.
	convertHooksParam = "convert_hooks"
	// core_objects (bool), if set, will decode core protobuf messages into their toit counterparts (Timestamp and Duration).
	// enabled by default.
	coreObjectsParam = "core_objects"

	protoLibrary         = "protogen"
	coreDurationMessage  = ".google.protobuf.Duration"
	coreTimestampMessage = ".google.protobuf.Timestamp"
)

type generator struct {
	req            *plugin.CodeGeneratorRequest
	options        generatorOptions
	importResolver *importResolver
	types          map[string]*referType
	imports        map[string]string
}

type generatorOptions struct {
	ConstructorInitializers bool
	ConvertHooks            bool
	CoreObjects             bool
	ImportLibraries         map[string]string
}

func parseGeneratorOptions(params map[string]string) (generatorOptions, error) {
	options := generatorOptions{
		ImportLibraries: map[string]string{},
		CoreObjects:     true,
	}
	if v, ok := params[constructorInitializersParam]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return options, fmt.Errorf("failed to parse '%s' option reason: %w", constructorInitializersParam, err)
		}
		options.ConstructorInitializers = b
	}

	if v, ok := params[convertHooksParam]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return options, fmt.Errorf("failed to parse '%s' option reason: %w", convertHooksParam, err)
		}
		options.ConvertHooks = b
	}

	if v, ok := params[coreObjectsParam]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return options, fmt.Errorf("failed to parse '%s' option reason: %w", coreObjectsParam, err)
		}
		options.CoreObjects = b
	}

	if v, ok := params[importLibraryParam]; ok {
		options.ImportLibraries = parseMap(v, ",", "=")
	}

	return options, nil
}

func newGenerator(req *plugin.CodeGeneratorRequest, params map[string]string) (*generator, error) {
	options, err := parseGeneratorOptions(params)
	if err != nil {
		return nil, err
	}
	return &generator{
		req:            req,
		options:        options,
		importResolver: newImportResolver(options.ImportLibraries),
		types:          map[string]*referType{},
		imports:        map[string]string{},
	}, nil
}

type importResolver struct {
	keys   []string
	values map[string]string
}

func newImportResolver(importLibraries map[string]string) *importResolver {
	importLibraries["google/protobuf/"] = protoLibrary + ".google.protobuf"

	res := &importResolver{
		values: importLibraries,
	}
	for k := range importLibraries {
		res.keys = append(res.keys, k)
	}
	sort.Strings(res.keys)
	return res
}

func (g *generator) resolveTypes() {
	for i := range g.req.ProtoFile {
		f := g.req.ProtoFile[i]
		g.resolveEnumTypes(f.GetEnumType(), nil, f)
		g.resolveMessageTypes(f.GetMessageType(), nil, f)
	}
}

func (r *importResolver) resolveImport(sourceFile string, importFile string) string {
	var prefixKey *string
	for i := len(r.keys) - 1; i >= 0; i-- {
		prefix := r.keys[i]
		if strings.HasPrefix(importFile, prefix) {
			prefixKey = &prefix
			break
		}
	}

	importProtoFile := protoToFile(importFile)
	if prefixKey == nil {
		return relToitPath(sourceFile, importProtoFile)
	}

	libraryPath := r.values[*prefixKey]
	return libraryPath + toit.Path(strings.TrimPrefix(importProtoFile, *prefixKey))
}

func (g *generator) resolveEnumTypes(enums []*descriptor.EnumDescriptorProto, parent *referType, file *descriptor.FileDescriptorProto) {
	for i := range enums {
		enum := enums[i]
		t := &referType{
			file:   file,
			enum:   enum,
			parent: parent,
		}
		g.types[t.Name()] = t
	}
}

func (g *generator) resolveMessageTypes(msgs []*descriptor.DescriptorProto, parent *referType, file *descriptor.FileDescriptorProto) {
	for i := range msgs {
		msg := msgs[i]
		t := &referType{
			file:   file,
			msg:    msg,
			parent: parent,
		}
		g.types[t.Name()] = t
		g.resolveEnumTypes(msg.GetEnumType(), t, file)
		g.resolveMessageTypes(msg.GetNestedType(), t, file)
	}
}

func (g *generator) lookupType(typ string) (*referType, bool) {
	t, ok := g.types[typ]
	return t, ok
}

func uniqueName(name string, namespace util.StringSet, prefix string) string {
	for namespace.Contains(name) {
		name = prefix + name
	}
	return name
}

var (
	reservedFieldNames = util.NewStringSet("operator", "static", "class", "constructor", "interface")
)

func (g *generator) generateFile(file *descriptor.FileDescriptorProto) (*plugin.CodeGeneratorResponse_File, error) {
	resp := &plugin.CodeGeneratorResponse_File{}
	resp.Name = util.StringPtr(protoToFile(file.GetName()))

	buffer := bytes.NewBuffer(nil)
	w := toit.NewWriter(buffer)

	w.SingleLineComment("Code generated by protoc-gen-toit. DO NOT EDIT.")
	w.SingleLineComment("source: " + file.GetName())

	w.NewLine()

	// create imports
	g.imports[file.GetName()] = ""
	w.ImportAs("encoding.protobuf", "_protobuf")
	w.ImportAs("core", "_core")
	importNames := util.NewStringSet("_protobuf", "_core")

	for _, dep := range file.GetDependency() {
		alias := uniqueName(fileImportAlias(dep), importNames, "_")
		g.imports[dep] = alias

		w.ImportAs(g.importResolver.resolveImport(file.GetName(), dep), alias)
	}

	w.NewLine()

	var typePath []string
	if file != nil && file.Package != nil {
		typePath = append(typePath, file.GetPackage())
	}

	// create enums
	for _, enum := range file.GetEnumType() {
		if err := g.writeEnum(w, enum, typePath...); err != nil {
			return nil, err
		}
	}

	for _, msg := range file.GetMessageType() {
		if err := g.writeMessage(w, msg, typePath...); err != nil {
			return nil, err
		}
	}

	resp.Content = util.StringPtr(buffer.String())

	return resp, nil
}

func typeName(name string, typePath ...string) string {
	return "." + strings.Join(append(typePath, name), ".")
}

func (g *generator) writeOneof(w *toit.Writer, msg *descriptor.DescriptorProto, oneof *descriptor.OneofDescriptorProto, typePath ...string) (*oneofType, error) {
	res := &oneofType{
		Descriptor:    oneof,
		FieldName:     uniqueName(oneof.GetName()+"_", reservedFieldNames, "_"),
		CaseGetter:    uniqueName(oneof.GetName()+"_oneof_case", reservedFieldNames, "_"),
		ClearFunction: uniqueName(oneof.GetName()+"_oneof_clear", reservedFieldNames, "_"),
		CaseFields:    map[int32]string{},
	}
	res.CaseName = res.CaseGetter + "_"

	typeName := typeName(oneof.GetName(), typePath...)
	if err := util.FirstError(
		w.SingleLineComment("ONEOF START: "+typeName),
		w.Variable(res.FieldName, "", "null"),
		w.Variable(res.CaseName, "int?", "null"),
		w.NewLine(),
	); err != nil {
		return nil, err
	}

	if err := util.FirstError(
		w.StartFunctionDecl(res.ClearFunction),
		w.EndFunctionDecl("none"),
		w.StartAssignment(res.FieldName),
		w.Argument("null"),
		w.EndAssignment(),
		w.StartAssignment(res.CaseName),
		w.Argument("null"),
		w.EndAssignment(),
		w.EndFunction(),
	); err != nil {
		return nil, err
	}

	for _, field := range msg.GetField() {
		if field.OneofIndex == nil || msg.GetOneofDecl()[field.GetOneofIndex()] != oneof {
			continue
		}

		fieldName := uniqueName(res.FieldName+field.GetName(), reservedFieldNames, "_")
		if err := w.StaticConst(strings.ToUpper(fieldName), "int", strconv.Itoa(int(field.GetNumber()))); err != nil {
			return nil, err
		}
		res.CaseFields[field.GetNumber()] = fieldName
	}

	if err := util.FirstError(
		w.NewLine(),
		w.StartFunctionDecl(res.CaseGetter),
		w.EndFunctionDecl("int?"),
		w.ReturnStart(),
		w.Argument(res.CaseName),
		w.ReturnEnd(),
		w.EndFunction(),
	); err != nil {
		return nil, err
	}

	for _, field := range msg.GetField() {
		if field.OneofIndex == nil || msg.GetOneofDecl()[field.GetOneofIndex()] != oneof {
			continue
		}
		fieldType, err := g.resolveFieldType(field, false)
		if err != nil {
			return nil, err
		}

		fieldName := res.CaseFields[field.GetNumber()]
		t, err := fieldType.ToitTypeAnnotation(false)
		if err != nil {
			return nil, err
		}

		if err := util.FirstError(
			// Getter
			w.StartFunctionDecl(fieldName),
			w.EndFunctionDecl(t),
			w.ReturnStart(),
			w.Argument(res.FieldName),
			w.ReturnEnd(),
			w.EndFunction(),

			// Setter
			w.StartFunctionDecl(fieldName+"="),
			w.Parameter(oneof.GetName(), t),
			w.EndFunctionDecl("none"),
			w.StartAssignment(res.FieldName),
			w.Argument(oneof.GetName()),
			w.EndAssignment(),
			w.StartAssignment(res.CaseName),
			w.Argument(strings.ToUpper(fieldName)),
			w.EndAssignment(),
			w.EndFunction(),
		); err != nil {
			return nil, err
		}
	}

	return res, w.SingleLineComment("ONEOF END: " + typeName)
}

func (g *generator) writeEnum(w *toit.Writer, enum *descriptor.EnumDescriptorProto, typePath ...string) error {
	typeName := typeName(enum.GetName(), typePath...)
	typ, ok := g.lookupType(typeName)
	if !ok {
		return fmt.Errorf("failed to find local enum type: %v", typeName)
	}
	className := typ.ToitType("")
	w.SingleLineComment("ENUM START: " + className)
	for _, value := range enum.GetValue() {
		w.Const(toitClassName(value.GetName(), className), "int/*enum<"+className+">*/", strconv.Itoa(int(value.GetNumber())))
	}
	w.SingleLineComment("ENUM END: " + typeName)
	w.NewLine()
	return nil
}

func dot(s ...string) string {
	return strings.Join(s, ".")
}

func (g *generator) writeMessage(w *toit.Writer, msg *descriptor.DescriptorProto, typePath ...string) error {
	typeName := typeName(msg.GetName(), typePath...)
	typ, ok := g.lookupType(typeName)
	if !ok {
		return fmt.Errorf("failed to find local msg type: %v", typeName)
	}
	recTypePath := append(typePath, msg.GetName())
	className := typ.ToitType("")
	w.SingleLineComment("MESSAGE START: " + typeName)
	for _, enum := range msg.GetEnumType() {
		if err := g.writeEnum(w, enum, recTypePath...); err != nil {
			return err
		}
	}

	for _, subMsg := range msg.GetNestedType() {
		if !subMsg.GetOptions().GetMapEntry() {
			if err := g.writeMessage(w, subMsg, recTypePath...); err != nil {
				return err
			}
		}
	}

	definedNames := util.NewStringSet()

	w.StartClass(className, "_protobuf.Message")
	var oneofTypes []*oneofType
	for i := range msg.GetOneofDecl() {
		oneof := msg.OneofDecl[i]
		oneofType, err := g.writeOneof(w, msg, oneof, recTypePath...)
		if err != nil {
			return err
		}
		oneofTypes = append(oneofTypes, oneofType)

		if definedNames.Contains(oneofType.FieldName) {
			return fmt.Errorf("name clash for for oneof field: %v", oneofType.FieldName)
		}
		if definedNames.Contains(oneofType.CaseName) {
			return fmt.Errorf("name clash for for oneof case field: %v", oneofType.CaseName)
		}
		if definedNames.Contains(oneofType.CaseGetter) {
			return fmt.Errorf("name clash for for oneof case getter: %v", oneofType.CaseGetter)
		}
		definedNames.Add(oneofType.FieldName, oneofType.CaseGetter, oneofType.CaseName)

		for _, fieldName := range oneofType.CaseFields {
			constant := strings.ToUpper(fieldName)
			if definedNames.Contains(constant) {
				return fmt.Errorf("name clash for for oneof constant: %v", constant)
			}
			if definedNames.Contains(fieldName) {
				return fmt.Errorf("name clash for for oneof getter: %v", fieldName)
			}
			setter := fieldName + "="
			if definedNames.Contains(setter) {
				return fmt.Errorf("name clash for for oneof setter: %v", setter)
			}
			definedNames.Add(constant, fieldName, setter)
		}
	}

	var fields []*fieldType
	for _, field := range msg.GetField() {
		fieldType, err := g.resolveFieldType(field, false)
		if err != nil {
			return err
		}
		fields = append(fields, fieldType)
		if field.OneofIndex != nil {
			continue
		}

		fieldName := uniqueName(field.GetName(), reservedFieldNames, "_")
		if definedNames.Contains(fieldName) {
			return fmt.Errorf("name clash for for field: %v", fieldName)
		}
		definedNames.Add(fieldName)

		t, err := fieldType.ToitTypeAnnotation(false)
		if err != nil {
			return err
		}

		defaultValue, err := fieldType.DefaultValue()
		if err != nil {
			return err
		}
		w.Variable(fieldName, t, defaultValue)
	}
	w.NewLine()

	if g.options.ConvertHooks {
		if err := g.writeDeserializeIntoMethod(w, className, fields, oneofTypes); err != nil {
			return err
		}
	}

	if err := g.writeDefaultConstructor(w, fields, oneofTypes); err != nil {
		return err
	}

	if err := g.writeDeserializeConstructor(w, fields, oneofTypes); err != nil {
		return err
	}

	if g.options.ConvertHooks {
		if err := g.writeClassConvertHooks(w, fields, oneofTypes); err != nil {
			return err
		}
	}

	if err := g.writeSerializeMethod(w, fields, oneofTypes); err != nil {
		return err
	}

	if err := g.writeNumFieldsSetMethod(w, fields, oneofTypes); err != nil {
		return err
	}

	if err := g.writeProtobufSizeMethod(w, fields, oneofTypes); err != nil {
		return err
	}

	w.EndClass()

	w.SingleLineComment("MESSAGE END: " + typeName)
	w.NewLine()
	return nil
}

func (g *generator) writeDefaultConstructor(w *toit.Writer, fields []*fieldType, oneofTypes []*oneofType) error {
	if !g.options.ConstructorInitializers {
		return util.FirstError(
			w.StartConstructorDecl(""),
			w.EndConstructorDecl(),
			w.EndConstructor(),
		)
	}

	if err := w.StartConstructorDecl(""); err != nil {
		return err
	}

	var fieldNames []string
	for _, fieldType := range fields {
		t, err := fieldType.ToitTypeAnnotation(true)
		if err != nil {
			return err
		}

		fieldName := fieldType.FieldName(oneofTypes)
		if err := util.FirstError(
			w.EndLine(),
			w.ParameterWithDefault("--"+fieldName, t, "null"),
		); err != nil {
			return err
		}
		fieldNames = append(fieldNames, fieldName)
	}
	if err := w.EndConstructorDecl(); err != nil {
		return err
	}

	for _, fieldName := range fieldNames {
		if err := util.FirstError(
			w.StartCall("if"),
			w.Argument(fieldName+" != null"),
			w.StartBlock(false),
			w.StartAssignment("this."+fieldName),
			w.Argument(fieldName),
			w.EndAssignment(),
			w.EndBlock(false),
			w.EndCall(true),
		); err != nil {
			return err
		}
	}

	return w.EndConstructor()
}

func (g *generator) writeDeserializeConstructor(w *toit.Writer, fields []*fieldType, oneofTypes []*oneofType) error {
	return util.FirstError(
		w.StartConstructorDecl("deserialize"),
		w.Parameter("r", "_protobuf.Reader"),
		w.EndConstructorDecl(),
		func() error {
			if !g.options.ConvertHooks {
				return g.writeDeserializeBody(w, "", fields, oneofTypes)
			}
			return util.FirstError(
				w.StartCall("deserialize_into"),
				w.Argument("r"),
				w.Argument("this"),
				w.EndCall(true),
			)
		}(),
		w.EndConstructor(),
	)
}

func (g *generator) writeDeserializeIntoMethod(w *toit.Writer, objectType string, fields []*fieldType, oneofTypes []*oneofType) error {
	return util.FirstError(
		w.StartStaticFunctionDecl("deserialize_into"),
		w.Parameter("r", "_protobuf.Reader"),
		w.Parameter("obj", objectType),
		w.EndFunctionDecl(objectType),
		g.writeDeserializeBody(w, "obj", fields, oneofTypes),
		w.ReturnStart(),
		w.Argument("obj"),
		w.ReturnEnd(),
		w.EndFunction(),
	)
}

func (g *generator) writeClassConvertHooks(w *toit.Writer, fields []*fieldType, oneofTypes []*oneofType) error {
	for _, field := range fields {
		fieldName := field.FieldName(oneofTypes)
		if err := g.writeFieldConvertHooks(w, field, fieldName, false, oneofTypes); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) writeFieldConvertHooks(w *toit.Writer, fieldType *fieldType, methodSuffix string, inCollection bool, oneofTypes []*oneofType) error {
	switch fieldType.class {
	case fieldTypeClassList:
		return util.FirstError(
			g.writeDeserializeFieldMethod(w, fieldType, methodSuffix, false, oneofTypes),
			g.writeSerializeFieldMethod(w, fieldType, methodSuffix, false, oneofTypes),
			g.writeFieldConvertHooks(w, fieldType.valueType, methodSuffix+"_value", true, oneofTypes),
		)
	case fieldTypeClassMap:
		return util.FirstError(
			g.writeDeserializeFieldMethod(w, fieldType, methodSuffix, false, oneofTypes),
			g.writeSerializeFieldMethod(w, fieldType, methodSuffix, false, oneofTypes),
			g.writeFieldConvertHooks(w, fieldType.keyType, methodSuffix+"_key", true, oneofTypes),
			g.writeFieldConvertHooks(w, fieldType.valueType, methodSuffix+"_value", true, oneofTypes),
		)
	case fieldTypeClassObject:
		return util.FirstError(
			g.writeInitializeObjectMethod(w, fieldType, methodSuffix),
			g.writeSerializeFieldMethod(w, fieldType, methodSuffix, inCollection, oneofTypes),
		)
	case fieldTypeClassPrimitive:
		return util.FirstError(
			g.writeDeserializeFieldMethod(w, fieldType, methodSuffix, inCollection, oneofTypes),
			g.writeSerializeFieldMethod(w, fieldType, methodSuffix, inCollection, oneofTypes),
		)
	default:
		return fmt.Errorf("unkonwn fieldType: %v for field: %s", fieldType.class, fieldType.field.GetName())
	}
}

func (g *generator) writeInitializeObjectMethod(w *toit.Writer, fieldType *fieldType, methodSuffix string) error {
	t, err := fieldType.ToitTypeAnnotation(false)
	if err != nil {
		return err
	}

	defaultValue, err := fieldType.DefaultValue()
	if err != nil {
		return err
	}
	return util.FirstError(
		w.StartFunctionDecl("_initialize_"+methodSuffix),
		w.EndFunctionDecl(t),
		w.ReturnStart(),
		w.Argument(defaultValue),
		w.ReturnEnd(),
		w.EndFunction(),
	)
}

func (g *generator) writeDeserializeFieldMethod(w *toit.Writer, fieldType *fieldType, methodSuffix string, inCollection bool, oneofTypes []*oneofType) error {
	t, err := fieldType.ToitTypeAnnotation(false)
	if err != nil {
		return err
	}

	fieldName := fieldType.FieldName(oneofTypes)

	if inCollection {
		return util.FirstError(
			w.StartFunctionDecl("_deserialize_"+methodSuffix),
			w.Parameter("in", t),
			w.EndFunctionDecl("any"),
			w.ReturnStart(),
			w.Argument("in"),
			w.ReturnEnd(),
			w.EndFunction(),
		)
	}

	return util.FirstError(
		w.StartFunctionDecl("_deserialize_"+methodSuffix),
		w.Parameter("in", t),
		w.EndFunctionDecl(""),
		w.StartAssignment(fieldName),
		w.Argument("in"),
		w.EndAssignment(),
		w.EndFunction(),
	)
}

func (g *generator) writeSerializeFieldMethod(w *toit.Writer, fieldType *fieldType, methodSuffix string, inCollection bool, oneofTypes []*oneofType) error {
	t, err := fieldType.ToitTypeAnnotation(false)
	if err != nil {
		return err
	}
	fieldName := fieldType.FieldName(oneofTypes)
	if inCollection {
		return util.FirstError(
			w.StartFunctionDecl("_serialize_"+methodSuffix),
			w.Parameter("in", "any"),
			w.EndFunctionDecl(t),
			w.ReturnStart(),
			w.Argument("in"),
			w.ReturnEnd(),
			w.EndFunction(),
		)
	}

	return util.FirstError(
		w.StartFunctionDecl("_serialize_"+methodSuffix),
		w.EndFunctionDecl(t),
		w.ReturnStart(),
		w.Argument(fieldName),
		w.ReturnEnd(),
		w.EndFunction(),
	)
}

func (g *generator) writeDeserializeBody(w *toit.Writer, objectName string, fields []*fieldType, oneOfTypes []*oneofType) error {
	w.StartCall("r.read_message")
	w.StartBlock(false)

	if len(fields) == 0 {
		w.Literal("1")
	} else {
		for _, fieldType := range fields {
			if err := g.writeReadFieldCall(w, objectName, fieldType, oneOfTypes); err != nil {
				return err
			}
		}
	}

	w.EndBlock(false)
	w.EndCall(true)
	return nil
}

func (g *generator) writeReadFieldCall(w *toit.Writer, objectName string, fieldType *fieldType, oneofTypes []*oneofType) error {
	w.StartCall("r.read_field")
	w.Argument(strconv.Itoa(int(fieldType.field.GetNumber())))
	w.StartBlock(false)
	if err := g.writeReadFieldAssignment(w, objectName, fieldType, oneofTypes); err != nil {
		return err
	}

	w.EndBlock(false)
	w.EndCall(true)
	return nil
}

func (g *generator) writeReadFieldAssignment(w *toit.Writer, objectName string, fieldType *fieldType, oneofTypes []*oneofType) error {
	fieldName := fieldType.FieldName(oneofTypes)
	if g.options.ConvertHooks && fieldType.class != fieldTypeClassObject {
		return util.FirstError(
			w.StartCall(objectName+"._deserialize_"+fieldName),
			w.NewLine(),
			g.writeReadFieldType(w, objectName, fieldName, fieldType),
			w.EndCall(true),
		)
	}

	assignTo := fieldName
	if objectName != "" {
		assignTo = objectName + "." + assignTo
	}

	return util.FirstError(
		w.StartAssignment(assignTo),
		w.Argument(""),
		g.writeReadFieldType(w, objectName, fieldName, fieldType),
		w.EndAssignment(),
	)
}

func (g *generator) writeReadFieldType(w *toit.Writer, objectName string, fieldName string, fieldType *fieldType) error {
	switch fieldType.class {
	case fieldTypeClassList:
		protoType, err := protobufTypeConst(fieldType.valueType.field.GetType())
		if err != nil {
			return err
		}
		return util.FirstError(
			w.StartCall("r.read_array"),
			w.Argument("_protobuf."+protoType),
			w.Argument(fieldName),
			w.StartBlock(false),
			g.writeReadFieldType(w, objectName, fieldName+"_value", fieldType.valueType),
			w.EndBlock(false),
			w.EndCall(true),
		)
	case fieldTypeClassMap:
		return util.FirstError(
			w.StartCall("r.read_map"),
			w.Argument(fieldName),
			w.StartBlock(true),
			g.writeReadFieldType(w, objectName, fieldName+"_key", fieldType.keyType),
			w.EndBlock(true),
			w.StartBlock(true),
			g.writeReadFieldType(w, objectName, fieldName+"_value", fieldType.valueType),
			w.EndBlock(true),
			w.EndCall(true),
		)
	case fieldTypeClassObject:
		if g.options.CoreObjects {
			fnName := ""
			if fieldType.t.Name() == coreDurationMessage {
				fnName = "_protobuf.deserialize_duration"
			}
			if fieldType.t.Name() == coreTimestampMessage {
				fnName = "_protobuf.deserialize_timestamp"
			}

			if fnName != "" {
				return util.FirstError(
					w.StartCall(fnName),
					w.Argument("r"),
					w.EndCall(true),
				)
			}
		}

		importAlias, ok := g.imports[fieldType.t.file.GetName()]
		if !ok {
			return fmt.Errorf("failed to find import alias for field: '%s' - field: '%s'", fieldType.t.file.GetName(), fieldType.t.Name())
		}
		toitClass := fieldType.t.ToitType(importAlias)
		if g.options.ConvertHooks {
			return util.FirstError(
				w.StartCall(toitClass+".deserialize_into"),
				w.Argument("r"),
				w.Argument(objectName+"._initialize_"+fieldName),
				w.EndCall(true),
			)
		}

		return util.FirstError(
			w.StartCall(toitClass+".deserialize"),
			w.Argument("r"),
			w.EndCall(true),
		)
	case fieldTypeClassPrimitive:
		protoType, err := protobufTypeConst(fieldType.field.GetType())
		if err != nil {
			return err
		}
		return util.FirstError(
			w.StartCall("r.read_primitive"),
			w.Argument("_protobuf."+protoType),
			w.EndCall(true),
		)
	default:
		return fmt.Errorf("unkonwn fieldType: %v for field: %s", fieldType.class, fieldType.field.GetName())
	}
}

func (g *generator) writeSerializeMethod(w *toit.Writer, fields []*fieldType, oneOfTypes []*oneofType) error {
	w.StartFunctionDecl("serialize")
	w.Parameter("w", "_protobuf.Writer")
	w.ParameterWithDefault("--as_field", "int?", "null")
	w.ParameterWithDefault("--oneof", "bool", "false")
	w.EndFunctionDecl("none")

	w.StartCall("w.write_message_header")
	w.Argument("this")
	w.Argument("--as_field=as_field")
	w.Argument("--oneof=oneof")
	w.EndCall(true)

	if len(fields) == 0 {
		w.Literal("1")
	} else {
		for _, fieldType := range fields {
			if fieldType.field.OneofIndex != nil {
				if err := g.writeSerializeOneofField(w, fieldType, oneOfTypes); err != nil {
					return err
				}
			} else {
				if err := g.writeSerializeField(w, fieldType, "", nil, nil, nil); err != nil {
					return err
				}
			}
		}
	}

	w.EndFunction()
	return nil
}

func (g *generator) writeSerializeList(w *toit.Writer, fieldName string, fieldType *fieldType, asField *string, oneofFieldName *string) error {
	protoType, err := protobufTypeConst(fieldType.valueType.field.GetType())
	if err != nil {
		return err
	}
	toitType, err := fieldType.valueType.ToitTypeAnnotation(false)
	if err != nil {
		return err
	}
	return util.FirstError(
		w.StartCall("w.write_array"),
		w.Argument("_protobuf."+protoType),
		w.Argument(g.getSerializeFieldName(fieldName, oneofFieldName, nil)),
		writeSerializeNamedArguments(w, asField, oneofFieldName != nil),
		w.StartBlock(false, "value/"+toitType),
		g.writeSerializeField(w, fieldType.valueType, "value", nil, nil, &fieldName),
		w.EndBlock(false),
		w.EndCall(true),
	)
}

func (g *generator) writeSerializeMap(w *toit.Writer, fieldName string, fieldType *fieldType, asField *string, oneofFieldName *string) error {
	keyProtoType, err := protobufTypeConst(fieldType.keyType.field.GetType())
	if err != nil {
		return err
	}
	keyToitType, err := fieldType.keyType.ToitTypeAnnotation(false)
	if err != nil {
		return err
	}
	valueProtoType, err := protobufTypeConst(fieldType.valueType.field.GetType())
	if err != nil {
		return err
	}
	valueToitType, err := fieldType.valueType.ToitTypeAnnotation(false)
	if err != nil {
		return err
	}
	return util.FirstError(
		w.StartCall("w.write_map"),
		w.Argument("_protobuf."+keyProtoType),
		w.Argument("_protobuf."+valueProtoType),
		w.Argument(g.getSerializeFieldName(fieldName, oneofFieldName, nil)),
		writeSerializeNamedArguments(w, asField, oneofFieldName != nil),
		w.StartBlock(true, "key/"+keyToitType),
		g.writeSerializeField(w, fieldType.keyType, "key", nil, nil, &fieldName),
		w.EndBlock(true),
		w.StartBlock(true, "value/"+valueToitType),
		g.writeSerializeField(w, fieldType.valueType, "value", nil, nil, &fieldName),
		w.EndBlock(true),
		w.EndCall(true),
	)
}

func writeSerializeNamedArguments(w *toit.Writer, asField *string, oneof bool) error {
	if asField != nil {
		if err := w.NamedArgument("--as_field", *asField); err != nil {
			return err
		}
	}
	if oneof {
		if err := w.NamedArgument("--oneof", ""); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) getSerializeFieldName(fieldName string, oneofFieldName *string, collectionField *string) string {
	if !g.options.ConvertHooks {
		return fieldName
	}
	if oneofFieldName != nil {
		fieldName = *oneofFieldName
	}
	if collectionField != nil {
		return "(_serialize_" + *collectionField + "_" + fieldName + " " + fieldName + ")"
	}
	return "_serialize_" + fieldName
}

func (g *generator) writeSerializeMessage(w *toit.Writer, fieldType *fieldType, fieldName string, asField *string, oneofFieldName *string, collectionField *string) error {
	if g.options.CoreObjects {
		fnName := ""
		if fieldType.t.Name() == coreDurationMessage {
			fnName = "_protobuf.serialize_duration"
		}
		if fieldType.t.Name() == coreTimestampMessage {
			fnName = "_protobuf.serialize_timestamp"
		}

		if fnName != "" {
			return util.FirstError(
				w.StartCall(fnName),
				w.Argument(fieldName),
				w.Argument("w"),
				writeSerializeNamedArguments(w, asField, oneofFieldName != nil),
				w.EndCall(true),
			)
		}
	}

	return util.FirstError(
		w.StartCall(g.getSerializeFieldName(fieldName, oneofFieldName, collectionField)+".serialize"),
		w.Argument("w"),
		writeSerializeNamedArguments(w, asField, oneofFieldName != nil),
		w.EndCall(true),
	)
}

func (g *generator) writeSerializePrimitive(w *toit.Writer, fieldName string, fieldType *fieldType, asField *string, oneofFieldName *string, collectionField *string) error {
	ft := fieldType.field.GetType()
	protoType, err := protobufTypeConst(ft)
	if err != nil {
		return err
	}
	return util.FirstError(
		w.StartCall("w.write_primitive"),
		w.Argument("_protobuf."+protoType),
		w.Argument(g.getSerializeFieldName(fieldName, oneofFieldName, collectionField)),
		writeSerializeNamedArguments(w, asField, oneofFieldName != nil),
		w.EndCall(true),
	)
}

func (g *generator) writeSerializeOneofField(w *toit.Writer, fieldType *fieldType, oneofTypes []*oneofType) error {
	oneof := oneofTypes[fieldType.field.GetOneofIndex()]
	fieldConstant := strings.ToUpper(oneof.CaseFields[fieldType.field.GetNumber()])
	fieldName := fieldType.FieldName(oneofTypes)
	return util.FirstError(
		w.StartCall("if"),
		w.Argument(oneof.CaseName+" == "+fieldConstant),
		w.StartBlock(false),
		g.writeSerializeField(w, fieldType, oneof.FieldName, &fieldConstant, &fieldName, nil),
		w.EndBlock(false),
		w.EndCall(true),
	)
}

func (g *generator) writeSerializeField(w *toit.Writer, fieldType *fieldType, fieldName string, asField *string, oneofFieldName *string, collectionField *string) error {
	if fieldName == "" {
		fieldName = uniqueName(fieldType.field.GetName(), reservedFieldNames, "_")
		fieldNumber := strconv.Itoa(int(fieldType.field.GetNumber()))
		asField = &fieldNumber
	}
	switch fieldType.class {
	case fieldTypeClassList:
		return g.writeSerializeList(w, fieldName, fieldType, asField, oneofFieldName)
	case fieldTypeClassMap:
		return g.writeSerializeMap(w, fieldName, fieldType, asField, oneofFieldName)
	case fieldTypeClassObject:
		return g.writeSerializeMessage(w, fieldType, fieldName, asField, oneofFieldName, collectionField)
	case fieldTypeClassPrimitive:
		return g.writeSerializePrimitive(w, fieldName, fieldType, asField, oneofFieldName, collectionField)
	default:
		return fmt.Errorf("unkonwn fieldType: %v for field: %s", fieldType.class, fieldType.field.GetName())
	}
}

func (g *generator) writeNumFieldsSetMethod(w *toit.Writer, fields []*fieldType, oneofTypes []*oneofType) error {
	w.StartFunctionDecl("num_fields_set")
	w.EndFunctionDecl("int")

	if len(fields) == 0 {
		return util.FirstError(
			w.ReturnStart(),
			w.Argument("0"),
			w.ReturnEnd(),
			w.EndFunction(),
		)
	}
	w.ReturnStart()
	w.Argument("")
	i := 0
	for _, oneof := range oneofTypes {
		if i != 0 {
			w.Literal("+ ")
		}
		w.ConditionExpression(oneof.CaseName+" == null", "0", "1")
		w.EndLine()
		i++
	}

	for _, fieldType := range fields {
		if fieldType.field.OneofIndex != nil {
			continue
		}

		fieldName := uniqueName(fieldType.field.GetName(), reservedFieldNames, "_")
		if g.options.ConvertHooks {
			fieldName = "_serialize_" + fieldName
		}
		var condition string
		switch fieldType.class {
		case fieldTypeClassList, fieldTypeClassMap:
			condition = fieldName + ".is_empty"
		case fieldTypeClassObject:
			if g.options.CoreObjects {
				if fieldType.t.Name() == coreDurationMessage {
					condition = fmt.Sprintf("%s.is_zero", fieldName)
				} else if fieldType.t.Name() == coreTimestampMessage {
					condition = fmt.Sprintf("(_protobuf.time_is_zero_epoch %s)", fieldName)
				}
			}

			if len(condition) == 0 {
				condition = fieldName + ".is_empty"
			}
		case fieldTypeClassPrimitive:
			if fieldType.field.GetType() == descriptor.FieldDescriptorProto_TYPE_STRING ||
				fieldType.field.GetType() == descriptor.FieldDescriptorProto_TYPE_BYTES {
				condition = fieldName + ".is_empty"
			} else {
				defaultValue, err := fieldType.DefaultValue()
				if err != nil {
					return err
				}
				condition = fieldName + " == " + defaultValue
			}
		}

		if i != 0 {
			w.Literal("+ ")
		}
		w.ConditionExpression(condition, "0", "1")
		w.EndLine()
		i++
	}
	w.ReturnEnd()
	w.EndFunction()
	return nil
}

func (g *generator) writeProtobufSizeMethod(w *toit.Writer, fields []*fieldType, oneofTypes []*oneofType) error {
	w.StartFunctionDecl("protobuf_size")
	w.EndFunctionDecl("int")

	if len(fields) == 0 {
		return util.FirstError(
			w.ReturnStart(),
			w.Argument("0"),
			w.ReturnEnd(),
			w.EndFunction(),
		)
	}
	w.ReturnStart()
	w.Argument("")
	for i, fieldType := range fields {
		if i != 0 {
			w.EndLine()
			w.Literal("+ ")
		}

		if fieldType.field.OneofIndex != nil {
			oneof := oneofTypes[*fieldType.field.OneofIndex]
			fieldConstant := strings.ToUpper(oneof.CaseFields[fieldType.field.GetNumber()])
			if err := util.FirstError(
				w.StartParens(),
				w.Literal(oneof.CaseName+" == "+fieldConstant),
				w.Literal(" ? "),
				g.writeProtobufSizeField(w, fieldType, oneofTypes),
				w.Literal(" : "),
				w.Literal("0"),
				w.EndParens(),
			); err != nil {
				return err
			}
		} else {
			if err := g.writeProtobufSizeField(w, fieldType, oneofTypes); err != nil {
				return err
			}
		}
		w.EndLine()
	}
	w.ReturnEnd()
	w.EndFunction()
	return nil
}

func (g *generator) writeProtobufSizeField(w *toit.Writer, fieldType *fieldType, oneofTypes []*oneofType) error {
	fieldName := fieldType.FieldName(oneofTypes)
	if g.options.ConvertHooks {
		fieldName = "_serialize_" + fieldName
	}

	switch fieldType.class {
	case fieldTypeClassList:
		protoType, err := protobufTypeConst(fieldType.valueType.field.GetType())
		if err != nil {
			return err
		}
		if err := util.FirstError(
			w.StartParens(),
			w.StartCall("_protobuf.size_array"),
			w.Argument("_protobuf."+protoType),
			w.Argument(fieldName),
			w.NamedArgument("--as_field", strconv.Itoa(int(fieldType.field.GetNumber()))),
			w.EndParens(),
			w.EndCall(false),
		); err != nil {
			return err
		}
	case fieldTypeClassMap:
		keyProtoType, err := protobufTypeConst(fieldType.keyType.field.GetType())
		if err != nil {
			return err
		}
		valueProtoType, err := protobufTypeConst(fieldType.valueType.field.GetType())
		if err != nil {
			return err
		}
		if err := util.FirstError(
			w.StartParens(),
			w.StartCall("_protobuf.size_map"),
			w.Argument("_protobuf."+keyProtoType),
			w.Argument("_protobuf."+valueProtoType),
			w.Argument(fieldName),
			w.NamedArgument("--as_field", strconv.Itoa(int(fieldType.field.GetNumber()))),
			w.EndParens(),
			w.EndCall(false),
		); err != nil {
			return err
		}
	case fieldTypeClassObject:
		if g.options.CoreObjects {
			if fieldType.t.Name() == coreDurationMessage {
				if err := util.FirstError(
					w.StartParens(),
					w.StartCall("_protobuf.size_duration"),
					w.Argument(fieldName),
					w.NamedArgument("--as_field", strconv.Itoa(int(fieldType.field.GetNumber()))),
					w.EndParens(),
					w.EndCall(false),
				); err != nil {
					return err
				}
				return nil
			} else if fieldType.t.Name() == coreTimestampMessage {
				if err := util.FirstError(
					w.StartParens(),
					w.StartCall("_protobuf.size_timestamp"),
					w.Argument(fieldName),
					w.NamedArgument("--as_field", strconv.Itoa(int(fieldType.field.GetNumber()))),
					w.EndParens(),
					w.EndCall(false),
				); err != nil {
					return err
				}
				return nil
			}
		}

		if err := util.FirstError(
			w.StartParens(),
			w.StartCall("_protobuf.size_embedded_message"),
			w.Argument("("+fieldName+".protobuf_size)"),
			w.NamedArgument("--as_field", strconv.Itoa(int(fieldType.field.GetNumber()))),
			w.EndParens(),
			w.EndCall(false),
		); err != nil {
			return err
		}
		return nil
	case fieldTypeClassPrimitive:
		protoType, err := protobufTypeConst(fieldType.field.GetType())
		if err != nil {
			return err
		}
		if err := util.FirstError(
			w.StartParens(),
			w.StartCall("_protobuf.size_primitive"),
			w.Argument("_protobuf."+protoType),
			w.Argument(fieldName),
			w.NamedArgument("--as_field", strconv.Itoa(int(fieldType.field.GetNumber()))),
			w.EndParens(),
			w.EndCall(false),
		); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) resolveFieldType(field *descriptor.FieldDescriptorProto, ignoreRepeated bool) (*fieldType, error) {
	var t *referType
	if field.GetType() == descriptor.FieldDescriptorProto_TYPE_MESSAGE || field.GetType() == descriptor.FieldDescriptorProto_TYPE_ENUM {
		var ok bool
		if t, ok = g.lookupType(field.GetTypeName()); !ok {
			return nil, fmt.Errorf("failed to find fieldtype: %v for field: %s", field.GetTypeName(), field.GetName())
		}
	}

	label := field.GetLabel()
	if ignoreRepeated && label == descriptor.FieldDescriptorProto_LABEL_REPEATED {
		label = descriptor.FieldDescriptorProto_LABEL_REQUIRED
	}

	switch label {
	case descriptor.FieldDescriptorProto_LABEL_OPTIONAL, descriptor.FieldDescriptorProto_LABEL_REQUIRED:
		class := fieldTypeClassPrimitive
		if field.GetType() == descriptor.FieldDescriptorProto_TYPE_MESSAGE {
			class = fieldTypeClassObject
		}
		return &fieldType{
			g:     g,
			class: class,
			field: field,
			t:     t,
		}, nil
	case descriptor.FieldDescriptorProto_LABEL_REPEATED:
		if field.GetType() != descriptor.FieldDescriptorProto_TYPE_MESSAGE ||
			(t.msg != nil && !t.msg.GetOptions().GetMapEntry()) {
			valueField, err := g.resolveFieldType(field, true)
			if err != nil {
				return nil, err
			}
			return &fieldType{
				g:         g,
				class:     fieldTypeClassList,
				field:     field,
				t:         t,
				valueType: valueField,
			}, nil
		}

		if t.msg == nil {
			return nil, fmt.Errorf("fieldtype was not a message: %+v for field: %s", t, field.GetName())
		}

		k, v := t.msg.GetMapFields()
		keyField, err := g.resolveFieldType(k, false)
		if err != nil {
			return nil, err
		}

		valueField, err := g.resolveFieldType(v, false)
		if err != nil {
			return nil, err
		}

		return &fieldType{
			g:         g,
			class:     fieldTypeClassMap,
			field:     field,
			t:         t,
			keyType:   keyField,
			valueType: valueField,
		}, nil

	default:
		return nil, fmt.Errorf("unknown field label: %v", field.GetLabel())
	}
}

func (g *generator) Generate() (*plugin.CodeGeneratorResponse, error) {
	g.resolveTypes()
	res := &plugin.CodeGeneratorResponse{}
	files := util.NewStringSet(g.req.GetFileToGenerate()...)
	// This convenience method will return a structure of some types that I use
	for _, file := range g.req.ProtoFile {
		if files.Contains(file.GetName()) {
			r, err := g.generateFile(file)
			if err != nil {
				return nil, err
			}
			res.File = append(res.File, r)
		}
	}

	return res, nil
}

func parseMap(in, groupSep, kvSep string) map[string]string {
	res := make(map[string]string)
	groupkv := strings.Split(in, groupSep)
	for _, element := range groupkv {
		kv := strings.SplitN(element, kvSep, 2)
		if len(kv) > 1 {
			res[kv[0]] = kv[1]
		}
	}
	return res
}

func Run(req *plugin.CodeGeneratorRequest) (*plugin.CodeGeneratorResponse, error) {
	g, err := newGenerator(req, parseMap(req.GetParameter(), ";", "="))
	if err != nil {
		return nil, err
	}
	return g.Generate()
}
