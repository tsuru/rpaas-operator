// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package macro

import (
	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

type MacroExpr struct {
	Name      string           `parser:"@MacroName"`
	Arguments []*MacroArgument `parser:"(@@*)"`
}

type MacroArgument struct {
	KV  *MacroKV `parser:"@@ |" json:",omitempty"`
	Arg string   `parser:"@(Key|String)" json:",omitempty"`
}

type MacroKV struct {
	Key   string `parser:"@(Key|String)'='"`
	Value string `parser:"@(Key|String)"`
}

func ParseExp(input string) (*MacroExpr, error) {
	macroLexer, err := lexer.NewSimple([]lexer.SimpleRule{
		{Name: "MacroName", Pattern: `[A-Z_][A-Z0-9_]*`},
		{Name: "Key", Pattern: `[a-zA-Z_][a-zA-Z0-9_:/]*`},
		{Name: "String", Pattern: `"[^"]*"|'[^']*'`},
		{Name: "Whitespace", Pattern: `\s+`},
		{Name: "Equals", Pattern: `=`},
	})
	if err != nil {
		return nil, err
	}

	parser := participle.MustBuild[MacroExpr](
		participle.Lexer(macroLexer),
		participle.Elide("Whitespace"),
		participle.Unquote("String"),
	)

	parsed, err := parser.ParseString("", input)
	if err != nil {
		return nil, err
	}

	return parsed, nil
}
