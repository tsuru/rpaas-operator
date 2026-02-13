package cmd

import (
	"bytes"
	"testing"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/stretchr/testify/assert"
)

func TestNewTable(t *testing.T) {
	tests := []struct {
		name          string
		options       []tablewriter.Option
		headers       []string
		rows          [][]string
		asciiFallback bool
		expected      string
	}{
		{
			name:    "default options",
			headers: []string{"Name", "Value"},
			rows:    [][]string{{"foo", "bar"}},
			expected: `┌──────┬───────┐
│ Name │ Value │
├──────┼───────┤
│ foo  │ bar   │
└──────┴───────┘
`,
		},
		{
			name: "with row separator between rows",
			options: []tablewriter.Option{
				tablewriter.WithRendition(tw.Rendition{
					Settings: tw.Settings{Separators: tw.Separators{BetweenRows: tw.On}},
				}),
			},
			headers: []string{"Name", "Value"},
			rows:    [][]string{{"a", "1"}, {"b", "2"}},
			expected: `┌──────┬───────┐
│ Name │ Value │
├──────┼───────┤
│ a    │ 1     │
├──────┼───────┤
│ b    │ 2     │
└──────┴───────┘
`,
		},
		{
			name: "with right-aligned column",
			options: []tablewriter.Option{
				tablewriter.WithRowAlignmentConfig(tw.CellAlignment{
					PerColumn: []tw.Align{tw.AlignLeft, tw.AlignRight},
				}),
			},
			headers: []string{"Name", "Count"},
			rows:    [][]string{{"foo", "123"}, {"bar", "4"}},
			expected: `┌──────┬───────┐
│ Name │ Count │
├──────┼───────┤
│ foo  │   123 │
│ bar  │     4 │
└──────┴───────┘
`,
		},
		{
			name:          "ASCII fallback",
			asciiFallback: true,
			headers:       []string{"Name", "Value"},
			rows:          [][]string{{"foo", "bar"}},
			expected: `+------+-------+
| Name | Value |
+------+-------+
| foo  | bar   |
+------+-------+
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalASCII := fallbackToASCII
			originalColor := enabledColorizedTables
			defer func() {
				fallbackToASCII = originalASCII
				enabledColorizedTables = originalColor
			}()
			fallbackToASCII = tt.asciiFallback
			enabledColorizedTables = false

			var buf bytes.Buffer
			table := newTable(&buf, tt.options...)
			table.Header(tt.headers)
			table.Bulk(tt.rows)
			table.Render()

			assert.Equal(t, tt.expected, buf.String())
		})
	}
}

func TestSetBorderColorByString(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedBorder  renderer.Colors
		expectedChanged bool
	}{
		{
			name:            "known color",
			input:           "red",
			expectedBorder:  renderer.Colors{color.FgRed},
			expectedChanged: true,
		},
		{
			name:            "unknown color name",
			input:           "nonexistent",
			expectedChanged: false,
		},
		{
			name:            "hex color (unsupported)",
			input:           "#FF0000",
			expectedChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := tableColorizedConfig
			defer func() { tableColorizedConfig = original }()

			setBorderColorByString(tt.input)

			if tt.expectedChanged {
				assert.Equal(t, tt.expectedBorder, tableColorizedConfig.Border.FG)
				assert.Equal(t, tt.expectedBorder, tableColorizedConfig.Separator.FG)
			} else {
				assert.Equal(t, original.Border.FG, tableColorizedConfig.Border.FG)
				assert.Equal(t, original.Separator.FG, tableColorizedConfig.Separator.FG)
			}
		})
	}
}

func TestTableCommonOptions(t *testing.T) {
	tests := []struct {
		name          string
		colorized     bool
		asciiFallback bool
		expectedLen   int
	}{
		{
			name:        "no flags set",
			expectedLen: 0,
		},
		{
			name:        "colorized only",
			colorized:   true,
			expectedLen: 1,
		},
		{
			name:          "ASCII fallback only",
			asciiFallback: true,
			expectedLen:   1,
		},
		{
			name:          "both enabled",
			colorized:     true,
			asciiFallback: true,
			expectedLen:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalColor := enabledColorizedTables
			originalASCII := fallbackToASCII
			defer func() {
				enabledColorizedTables = originalColor
				fallbackToASCII = originalASCII
			}()

			enabledColorizedTables = tt.colorized
			fallbackToASCII = tt.asciiFallback

			opts := tableCommonOptions()
			assert.Len(t, opts, tt.expectedLen)
		})
	}
}
