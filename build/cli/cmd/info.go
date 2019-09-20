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
)

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Lists avaiable plans and flavors for the specified rpaas-instance",
	Long:  `Lists avaiable plans and flavors for the specified rpaas-instance`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.ParseFlags(args)
		// fmt.Printf("service = %v\n", cmd.Flag("service").Value)
		// fmt.Printf("instance = %v\n", cmd.Flag("instance").Value)
		service := cmd.Flag("service").Value.String()
		instance := cmd.Flag("instance").Value.String()
		path := "/resources/" + instance + "/plans"
		prox := &proxy.Proxy{ServiceName: service, InstanceName: instance, Path: path, Method: "GET"}
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

		var plans map[string]string
		err = json.Unmarshal(body, &plans)
		if err != nil {
			return err
		}
		for name, description := range plans {
			fmt.Printf("%vt\t\t%v\n", name, description)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// infoCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	infoCmd.Flags().StringP("service", "s", "", "Service name")
	infoCmd.Flags().StringP("instance", "i", "", "Service instance name")
}
