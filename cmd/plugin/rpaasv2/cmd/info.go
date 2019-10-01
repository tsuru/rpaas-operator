// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/tsuru/rpaas-operator/build/cli/proxy"
)

func init() {
	rootCmd.AddCommand(infoCmd)

	infoCmd.Flags().StringP("service", "s", "", "Service name")
	infoCmd.Flags().StringP("instance", "i", "", "Service instance name")
	infoCmd.MarkFlagRequired("service")
	infoCmd.MarkFlagRequired("instance")
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Lists available plans and flavors for the specified instance",
	Long:  `Lists available plans and flavors for the specified instance`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.ParseFlags(args)
		service := cmd.Flag("service").Value.String()
		instance := cmd.Flag("instance").Value.String()
		info := infoArgs{
			service:  service,
			instance: instance,
			prox:     proxy.New(service, instance, "GET", &proxy.TsuruServer{}),
		}
		return runInfo(info)
	},
}

type infoArgs struct {
	service  string
	instance string
	prox     *proxy.Proxy
}

func runInfo(info infoArgs) error {
	for _, resource := range []string{"plans", "flavors"} {
		info.prox.Path = "/resources/" + info.instance + "/" + resource
		if err := getInfo(info.prox, resource); err != nil {
			return err
		}
		fmt.Printf("\n\n")
	}
	return nil
}

func getInfo(prox *proxy.Proxy, infoType string) error {
	res, err := prox.ProxyRequest()
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%v", res.Status)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var fp interface{}
	err = json.Unmarshal(body, &fp)
	if err != nil {
		return err
	}
	helperSlice := fp.([]interface{})
	WriteInfo(infoType, helperSlice)

	return nil
}

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

func WriteInfo(prefix string, data []interface{}) {
	// flushing stdout
	fmt.Println()

	dataSlice := prepareInfoSlice(data)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetHeader([]string{prefix, "Description"})
	for _, v := range dataSlice {
		table.Append(v)
	}

	table.Render()
}
