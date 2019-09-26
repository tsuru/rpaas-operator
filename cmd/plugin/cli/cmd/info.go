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
	rootCmd.AddCommand(infoCmd)

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	infoCmd.Flags().StringP("service", "s", "", "Service name")
	infoCmd.Flags().StringP("instance", "i", "", "Service instance name")
	infoCmd.MarkFlagRequired("service")
	infoCmd.MarkFlagRequired("instance")
}

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Lists avaiable plans and flavors for the specified rpaas-instance",
	Long:  `Lists avaiable plans and flavors for the specified rpaas-instance`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.ParseFlags(args)
		info := infoArgs{}
		info.service = cmd.Flag("service").Value.String()
		info.instance = cmd.Flag("instance").Value.String()
		info.prox = &proxy.Proxy{ServiceName: info.service, InstanceName: info.instance, Method: "GET"}
		info.prox.Server = &proxy.TsuruServer{}
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
	tableWriter.WriteInfo(infoType, helperSlice)

	return nil
}
