// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/proxy"
)

type scaleArgs struct {
	service  string
	instance string
	quantity int
	prox     *proxy.Proxy
}

// scaleCmd represents the scale command
var scaleCmd = &cobra.Command{
	Use:   "scale",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.ParseFlags(args)
		scale := scaleArgs{}
		scale.service = cmd.Flag("service").Value.String()
		scale.instance = cmd.Flag("instance").Value.String()
		scale.prox = proxy.New(scale.service, scale.instance, "GET", &proxy.TsuruServer{})
		var err error
		scale.quantity, err = cmd.Flags().GetInt("quantity")
		if err != nil {
			return err
		}

		return runScale(scale)
	},
}

func runScale(scale scaleArgs) error {
	scale.prox.Path = "/resources/" + scale.instance + "/scale"
	scale.prox.Body = strings.NewReader(string(scale.quantity))
	resp, err := scale.prox.ProxyRequest()
	if err != nil {
		return err
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		bodyString := string(respBody)
		return fmt.Errorf("Status Code: %v\nResponse Body:\n%v", resp.Status, bodyString)
	}

	fmt.Printf("Instance successfully scaled to %d unit(s)\n", scale.quantity)
	return nil
}

func init() {
	rootCmd.AddCommand(scaleCmd)

	scaleCmd.Flags().IntP("quantity", "q", 0, "Quantity of units to scale")
	scaleCmd.Flags().StringP("service", "s", "", "Service name")
	scaleCmd.Flags().StringP("instance", "i", "", "Service instance name")
	scaleCmd.MarkFlagRequired("service")
	scaleCmd.MarkFlagRequired("instance")
}
