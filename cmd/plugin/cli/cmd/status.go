/*
Copyright © 2019 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/tsuru/rpaas-operator/build/cli/proxy"
	"github.com/tsuru/rpaas-operator/build/cli/tableWriter"
)

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringP("service", "s", "", "Service name")
	statusCmd.Flags().StringP("instance", "i", "", "Service instance name")
	statusCmd.MarkFlagRequired("service")
	statusCmd.MarkFlagRequired("instance")
}

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of service instance",
	Long:  `Displays Node(vm) name, nginx status, and it's respective node ip address`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.ParseFlags(args)
		status := statusArgs{}
		status.service = cmd.Flag("service").Value.String()
		status.instance = cmd.Flag("instance").Value.String()
		status.prox = &proxy.Proxy{ServiceName: status.service, InstanceName: status.instance, Method: "GET"}
		status.prox.Server = &proxy.TsuruServer{}

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
	tableWriter.WriteStatus(helperSlice)
	return nil
}
