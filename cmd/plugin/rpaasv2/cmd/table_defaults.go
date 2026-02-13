package cmd

import (
	"io"
	"os"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

var tableColorizedConfig = renderer.ColorizedConfig{
	Header: renderer.Tint{
		FG: renderer.Colors{
			color.Reset,
		},
		BG: renderer.Colors{
			color.Reset,
		},
	},
	Column: renderer.Tint{
		FG: renderer.Colors{color.Reset},
	},
	Footer: renderer.Tint{
		FG: renderer.Colors{color.Reset},
	},
	Border:    renderer.Tint{FG: renderer.Colors{color.Reset}},
	Separator: renderer.Tint{FG: renderer.Colors{color.Reset}},
}

var (
	enabledColorizedTables = false
	fallbackToASCII        = false
)

func init() {
	tableColor := os.Getenv("TSURU_TABLE_COLOR")

	if tableColor != "" {
		enabledColorizedTables = true
		setBorderColorByString(tableColor)
	}

	tableUTF8 := os.Getenv("TSURU_TABLE_UTF8")
	if tableUTF8 == "false" {
		fallbackToASCII = true
	}
}

func setBorderColorByString(tableColor string) {
	if fgColor := colorMap[tableColor]; fgColor != 0 {
		tableColorizedConfig.Border.FG = renderer.Colors{fgColor}
		tableColorizedConfig.Separator.FG = renderer.Colors{fgColor}
	}
	// TODO: there is no support to RGB on tablewriter
	// } else if strings.HasPrefix(tableColor, "#") {
	// RGB support
	// }
}

var colorMap = map[string]color.Attribute{
	"black":      color.FgBlack,
	"red":        color.FgRed,
	"green":      color.FgGreen,
	"yellow":     color.FgYellow,
	"blue":       color.FgBlue,
	"magenta":    color.FgMagenta,
	"cyan":       color.FgCyan,
	"white":      color.FgWhite,
	"hi-black":   color.FgHiBlack,
	"hi-red":     color.FgHiRed,
	"hi-green":   color.FgHiGreen,
	"hi-yellow":  color.FgHiYellow,
	"hi-blue":    color.FgHiBlue,
	"hi-magenta": color.FgHiMagenta,
	"hi-cyan":    color.FgHiCyan,
	"hi-white":   color.FgHiWhite,
}

func newTable(w io.Writer, extraOptions ...tablewriter.Option) *tablewriter.Table {
	defaults := []tablewriter.Option{
		tablewriter.WithHeaderAutoFormat(tw.Off),
		tablewriter.WithHeaderAlignment(tw.AlignLeft),
	}
	defaults = append(defaults, tableCommonOptions()...)
	defaults = append(defaults, extraOptions...)
	return tablewriter.NewTable(w, defaults...)
}

func tableCommonOptions() []tablewriter.Option {
	options := []tablewriter.Option{}

	if enabledColorizedTables {
		options = append(options, tablewriter.WithRenderer(renderer.NewColorized(tableColorizedConfig)))
	}

	if fallbackToASCII {
		options = append(options, tablewriter.WithSymbols(tw.NewSymbols(tw.StyleASCII)))
	}

	return options
}
