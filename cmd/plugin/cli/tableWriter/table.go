package tableWriter

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
)

func prepareInfoSlice(data []interface{}) [][]string {
	dataSlice := [][]string{}
	for _, mapVal := range data {
		m := mapVal.(map[string]interface{})
		target := []string{fmt.Sprintf("%v", m["name"]),
			fmt.Sprintf("%v", m["description"])}
		dataSlice = append(dataSlice, target)
	}

	return dataSlice
}

func prepareStatusSlice(data map[string]interface{}) [][]string {
	dataSlice := [][]string{}
	for k, v := range data {
		v := v.(map[string]interface{})
		target := []string{
			fmt.Sprintf("%v", k),
			fmt.Sprintf("%v", v["status"]),
			fmt.Sprintf("%v", v["address"]),
		}
		dataSlice = append(dataSlice, target)
	}

	return dataSlice
}

func WriteInfo(prefix string, data []interface{}) {
	// flushing stdout
	fmt.Println()

	dataSlice := prepareInfoSlice(data)

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

func WriteStatus(data map[string]interface{}) {
	dataSlice := prepareStatusSlice(data)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetHeader([]string{"Node Name", "Status", "Address"})
	// table.SetHeaderColor(tablewriter.Colors{tablewriter.Bold, tablewriter.BgCyanColor},
	// 	tablewriter.Colors{tablewriter.Bold, tablewriter.BgHiGreenColor},
	// 	tablewriter.Colors{tablewriter.Bold, tablewriter.BgMagentaColor},
	// )
	// table.SetColumnColor(tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
	// 	tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiGreenColor},
	// 	tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiMagentaColor},
	// )
	for _, v := range dataSlice {
		table.Append(v)
	}

	table.Render()
}
