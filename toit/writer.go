// Copyright (C) 2021 Toitware ApS. All rights reserved.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package toit

import (
	"bytes"
	"io"

	"github.com/toitware/protoc-gen-toit/util"
)

var (
	newline = []byte("\n")
	indent  = []byte("  ")
)

type Writer struct {
	w         io.Writer
	ident     int
	emptyLine bool
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w:         w,
		ident:     0,
		emptyLine: true,
	}
}

func (w *Writer) Import(path string) error {
	return util.FirstError(
		w.write("import "),
		w.write(path),
		w.EndLine(),
	)
}

func (w *Writer) ImportAs(path string, alias string) error {
	return util.FirstError(
		w.write("import "),
		w.write(path),
		w.write(" as "),
		w.write(alias),
		w.EndLine(),
	)
}

func (w *Writer) SingleLineComment(s string) error {
	return util.FirstError(
		w.write("// "),
		w.write(s),
		w.EndLine(),
	)
}

func (w *Writer) MultiLineComment(s string) error {
	return util.FirstError(
		w.write("/*"),
		w.write(s),
		w.write("*/"),
	)
}

func (w *Writer) StaticConst(name string, typ string, value string) error {
	return util.FirstError(
		w.write("static "),
		w.Const(name, typ, value),
	)
}

func (w *Writer) Const(name string, typ string, value string) error {
	return util.FirstError(
		w.write(name),
		w.Type(typ),
		w.write(" ::= "),
		w.write(value),
		w.EndLine(),
	)
}

func (w *Writer) Variable(name string, typ string, value string) error {
	return util.FirstError(
		w.write(name),
		w.Type(typ),
		w.write(" := "),
		w.write(value),
		w.EndLine(),
	)
}

func (w *Writer) Parameter(name string, typ string) error {
	return util.FirstError(
		w.Space(),
		w.write(name),
		w.Type(typ),
	)
}

func (w *Writer) ParameterWithDefault(name string, typ string, def string) error {
	return util.FirstError(
		w.Space(),
		w.write(name),
		w.Type(typ),
		w.write("="),
		w.write(def),
	)
}

func (w *Writer) Argument(name string) error {
	return util.FirstError(
		w.Space(),
		w.write(name),
	)
}

func (w *Writer) NamedArgument(name string, value string) error {
	return util.FirstError(
		w.Space(),
		w.write(name),
		func() error {
			if value != "" {
				return util.FirstError(w.write("="), w.write(value))
			}
			return nil
		}(),
	)
}

func (w *Writer) StartCall(fn string) error {
	defer w.incIdent()
	return w.write(fn)
}

func (w *Writer) EndCall(endLine bool) error {
	defer w.decIdent()
	if endLine {
		return w.EndLine()
	}
	return nil
}

func (w *Writer) StartParens() error {
	return w.write("(")
}

func (w *Writer) EndParens() error {
	return w.write(")")
}

func (w *Writer) StartAssignment(field string) error {
	return util.FirstError(
		w.write(field),
		w.write(" ="),
	)
}

func (w *Writer) EndAssignment() error {
	return w.EndLine()
}

func (w *Writer) Literal(s string) error {
	return w.write(s)
}

func (w *Writer) StartBlock(newLined bool, parameters ...string) error {
	if newLined {
		defer w.incIdent()
		if err := w.EndLine(); err != nil {
			return err
		}
	}
	if err := w.write(":"); err != nil {
		return err
	}

	if len(parameters) == 0 {
		return w.EndLine()
	}
	return util.FirstError(
		w.write(" | "),
		w.writeParameters(parameters...),
		w.write(" | "),
		w.EndLine(),
	)
}

func (w *Writer) EndBlock(newLined bool) error {
	if newLined {
		defer w.decIdent()
	}
	return w.EndLine()
}

func (w *Writer) Type(typ string) error {
	if typ == "" {
		return nil
	}
	if _, err := w.w.Write([]byte("/")); err != nil {
		return err
	}
	_, err := w.w.Write([]byte(typ))
	return err
}

func (w *Writer) incIdent() {
	w.ident++
}

func (w *Writer) decIdent() {
	w.ident--
}

func (w *Writer) StartClass(name string, extends string, implements ...string) error {
	defer w.incIdent()
	var res []error
	res = append(res,
		w.write("class "),
		w.write(name),
	)
	if extends != "" {
		res = append(res,
			w.write(" extends "),
			w.write(extends),
		)
	}
	first := true
	for _, implement := range implements {
		if first {
			res = append(res, w.write(" implements "), w.write(implement))
			first = false
		} else {
			res = append(res, w.write(" "), w.write(implement))
		}
	}
	res = append(res,
		w.write(":"),
		w.EndLine(),
	)
	return util.FirstError(res...)
}

func (w *Writer) EndClass() error {
	defer w.decIdent()
	return nil
}

func (w *Writer) StartFunctionDecl(name string) error {
	defer w.incIdent()
	defer w.incIdent()
	return util.FirstError(
		w.write(name),
	)
}

func (w *Writer) StartStaticFunctionDecl(name string) error {
	defer w.incIdent()
	defer w.incIdent()
	return util.FirstError(
		w.write("static "),
		w.write(name),
	)
}

func (w *Writer) EndFunctionDecl(returnType string) error {
	defer w.decIdent()
	var res []error
	if returnType != "" {
		res = append(res, w.write(" -> "), w.write(returnType))
	}

	return util.FirstError(append(res, w.write(":"), w.EndLine())...)
}

func (w *Writer) EndFunction() error {
	defer w.decIdent()
	return w.NewLine()
}

func (w *Writer) StartConstructorDecl(name string) error {
	if name == "" {
		return w.StartFunctionDecl("constructor")
	}
	return util.FirstError(
		w.write("constructor."),
		w.StartFunctionDecl(name),
	)
}

func (w *Writer) EndConstructorDecl() error {
	return w.EndFunctionDecl("")
}

func (w *Writer) EndConstructor() error {
	return w.EndFunction()
}

func (w *Writer) ReturnStart() error {
	defer w.incIdent()
	return util.FirstError(
		w.write("return"),
	)
}

func (w *Writer) ReturnEnd() error {
	defer w.decIdent()
	return util.FirstError(
		w.EndLine(),
	)
}

func (w *Writer) ConditionExpression(cond string, trueCase string, falseCase string) error {
	return util.FirstError(
		w.StartParens(),
		w.write(cond),
		w.write(" ? "),
		w.write(trueCase),
		w.write(" : "),
		w.write(falseCase),
		w.EndParens(),
	)
}

func (w *Writer) writeParameters(strs ...string) error {
	for i, s := range strs {
		if i != 0 {
			w.Space()
		}
		if err := w.write(s); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) write(b string) error {
	if w.emptyLine {
		if _, err := w.w.Write(bytes.Repeat(indent, w.ident)); err != nil {
			return err
		}
		w.emptyLine = false
	}
	_, err := w.w.Write([]byte(b))
	return err
}

func (w *Writer) Space() error {
	if w.emptyLine {
		return nil
	}
	return w.write(" ")
}

func (w *Writer) NewLine() error {
	if _, err := w.w.Write(newline); err != nil {
		return err
	}
	w.emptyLine = true
	return nil
}

func (w *Writer) EndLine() error {
	if w.emptyLine {
		return nil
	}
	return w.NewLine()
}
