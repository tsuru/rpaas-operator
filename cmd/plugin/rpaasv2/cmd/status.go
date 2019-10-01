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
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringP("service", "s", "", "Service name")
	statusCmd.Flags().StringP("instance", "i", "", "Service instance name")
	statusCmd.MarkFlagRequired("service")
	statusCmd.MarkFlagRequired("instance")
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of service instance",
	Long:  `Displays Node(vm) name, nginx status, and it's respective node ip address`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.ParseFlags(args)
		status := statusArgs{}
		status.service = cmd.Flag("service").Value.String()
		status.instance = cmd.Flag("instance").Value.String()
		status.prox = proxy.New(status.service, status.instance, "GET", &proxy.TsuruServer{})
		// status.prox = &proxy.Proxy{ServiceName: status.service, InstanceName: status.instance, Method: "GET"}
		// status.prox.Server = &proxy.TsuruServer{}

		return runStatus(status)
	},
}

type statusArgs struct {
	service  string
	instance string
	prox     *proxy.Proxy
}

func runStatus(status statusArgs) error {
	status.prox.Path = "/resources/" + status.instance + "/node_status"
	if err := getStatus(status.prox); err != nil {
		return err
	}
	return nil
}

func getStatus(prox *proxy.Proxy) error {
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
	helperSlice := fp.(map[string]interface{})
	WriteStatus(helperSlice)
	return nil
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

func WriteStatus(data map[string]interface{}) {
	dataSlice := prepareStatusSlice(data)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetHeader([]string{"Node Name", "Status", "Address"})
	for _, v := range dataSlice {
		table.Append(v)
	}

	table.Render()
}
