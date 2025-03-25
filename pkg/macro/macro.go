// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package macro

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/template"
)

type Macro struct {
	Name string
	Args []MacroArg

	Template        string
	TemplateFuncMap template.FuncMap

	// private fields
	dynamicStructType reflect.Type
}

type MacroArgType string

const (
	MacroArgTypeString MacroArgType = "string"
	MacroArgTypeInt    MacroArgType = "int"
	MacroArgTypeBool   MacroArgType = "bool"
)

type MacroArg struct {
	Name     string
	Type     MacroArgType
	Default  string
	Required bool
}

var macros = map[string]*Macro{}

func init() {
	register(&corsMacro)
}

func register(macro *Macro) {
	macros[macro.Name] = macro
}

func Execute(name string, args map[string]string) (string, error) {
	macro, ok := macros[name]
	if !ok {
		return "", fmt.Errorf("macro %q not found", name)
	}

	structuredArgs, err := createStructOnTheFly(macro, args)
	if err != nil {
		return "", err
	}

	tpl, err := template.New("macro").Funcs(macro.TemplateFuncMap).Parse(macro.Template)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer

	err = tpl.Execute(&buf, structuredArgs)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func createStructOnTheFly(macro *Macro, args map[string]string) (any, error) {
	if macro.dynamicStructType == nil {
		dynamicFields := []reflect.StructField{}

		for _, arg := range macro.Args {
			var fieldType reflect.Type
			switch arg.Type {
			case MacroArgTypeString:
				fieldType = reflect.TypeOf("")
			case MacroArgTypeInt:
				fieldType = reflect.TypeOf(0)
			case MacroArgTypeBool:
				fieldType = reflect.TypeOf(false)
			}

			field := reflect.StructField{
				Name: capitalizeFirst(arg.Name),
				Type: fieldType,
				Tag:  reflect.StructTag(fmt.Sprintf(`json:"%s"`, arg.Name)),
			}

			dynamicFields = append(dynamicFields, field)
		}
		macro.dynamicStructType = reflect.StructOf(dynamicFields)
	}

	dynamicStruct := reflect.New(macro.dynamicStructType).Elem()

	for i, arg := range macro.Args {
		value := args[arg.Name]
		if value == "" {
			value = arg.Default
		}

		if arg.Required && value == "" {
			return nil, fmt.Errorf("missing required argument %q", arg.Name)
		}

		if arg.Type == MacroArgTypeInt {
			intValue, _ := strconv.Atoi(value)
			dynamicStruct.Field(i).SetInt(int64(intValue))

		} else if arg.Type == MacroArgTypeBool {
			boolValue, _ := strconv.ParseBool(value)
			dynamicStruct.Field(i).SetBool(boolValue)
		} else if arg.Type == MacroArgTypeString {
			dynamicStruct.Field(i).SetString(value)
		}
	}

	return dynamicStruct.Interface(), nil
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
