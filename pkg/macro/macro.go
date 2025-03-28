// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package macro

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
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
	Pos      *int
}

var macros = map[string]*Macro{}

func init() {
	register(&cors)
	register(&securityHeaders)
	register(&proxyPassWithHeaders)
	register(&geoIP2Headers)
}

func register(macro *Macro) {
	macros[macro.Name] = macro
}

var detectMacroRegex = regexp.MustCompile(`^(\s+)?([A-Z_]+)(.*);$`)

func Expand(input string) (string, error) {
	lines := strings.Split(input, "\n")
	var output strings.Builder

	for _, line := range lines {
		matches := detectMacroRegex.FindStringSubmatch(line)
		if len(matches) != 4 {
			output.WriteString(line)
			output.WriteString("\n")
			continue
		}

		identation := matches[1]
		name := strings.TrimSpace(matches[2])
		args := strings.TrimSpace(matches[3])

		parsedMacro, err := ParseExp(name + " " + args)
		if err != nil {
			output.WriteString(line)
			output.WriteString("\n")

			fmt.Println("Error parsing macro:", err)
			continue

		}

		result, err := Execute(strings.ToLower(parsedMacro.Name), listArgs(parsedMacro), mapKwargs(parsedMacro))
		if err != nil {
			output.WriteString(line)
			output.WriteString("\n")
			fmt.Println("Error parsing macro:", err)
			continue
		}
		output.WriteString(indentBlock(result, identation))
		output.WriteString("\n")
	}

	return output.String(), nil
}

func mapKwargs(m *MacroExpr) map[string]string {
	result := make(map[string]string)

	for _, arg := range m.Arguments {
		if arg.KV != nil {
			result[arg.KV.Key] = arg.KV.Value
		}
	}

	return result
}

func listArgs(m *MacroExpr) []string {
	result := make([]string, 0)

	for _, arg := range m.Arguments {
		if arg.Arg != "" {
			result = append(result, arg.Arg)
		}
	}

	return result
}

func indentBlock(block string, identation string) string {
	lines := strings.Split(block, "\n")
	var output strings.Builder

	last := len(lines) - 1
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			if i == last {
				continue
			}
			output.WriteString("\n")
			continue
		}
		output.WriteString(identation)
		output.WriteString(line)
		if i != last {
			output.WriteString("\n")
		}
	}

	return output.String()
}

func Execute(name string, args []string, kwargs map[string]string) (string, error) {
	macro, ok := macros[name]
	if !ok {
		return "", fmt.Errorf("macro %q not found", name)
	}

	structuredArgs, err := createStructOnTheFly(macro, args, kwargs)
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

func createStructOnTheFly(macro *Macro, args []string, kwargs map[string]string) (any, error) {
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
		value := kwargs[arg.Name]
		if value == "" {
			value = arg.Default
		}
		if value == "" && arg.Pos != nil && *arg.Pos < len(args) {
			value = args[*arg.Pos]
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
