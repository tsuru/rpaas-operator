package tableWriter

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
)

func prepareStringSlice(data []interface{}) [][]string {
	dataSlice := [][]string{}
	for _, mapVal := range data {
		m := mapVal.(map[string]interface{})
		target := []string{fmt.Sprintf("%v", m["name"]),
			fmt.Sprintf("%v", m["description"])}
		dataSlice = append(dataSlice, target)
	}

	return dataSlice
}

func WriteData(prefix string, data []interface{}) {
	// flushing stdout
	fmt.Println()

	dataSlice := prepareStringSlice(data)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetHeader([]string{prefix, "Description"})
	// table.SetHeaderColor(tablewriter.Colors{tablewriter.Bold, tablewriter.BgCyanColor},
	// 	tablewriter.Colors{tablewriter.Bold, tablewriter.BgHiGreenColor},
	// )
	// table.SetColumnColor(tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
	// 	tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiGreenColor},
	// )
	for _, v := range dataSlice {
		table.Append(v)
	}

	table.Render()
}
