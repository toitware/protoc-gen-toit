// Copyright (C) 2021 Toitware ApS. All rights reserved.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package generator

import (
	"path"
	"strings"

	"github.com/toitware/protoc-gen-toit/toit"
)

func protoToFile(f string) string {
	if strings.HasSuffix(f, ".proto") {
		f = strings.TrimSuffix(f, ".proto") + "_pb.toit"
	}
	return f
}

func fileImportAlias(f string) string {
	name := strings.TrimSuffix(path.Base(f), path.Ext(f))
	return toit.ToSnakeCase("_" + name)
}

func relToitPath(fromFile, toFile string) string {
	n := strings.Count(fromFile, "/")
	return toit.Path(strings.Repeat("../", n) + toFile)
}
