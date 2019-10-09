// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

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
	Short: `Scales the specified rpaas instance to [-q] units`,
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.ParseFlags(args)
		serviceName := cmd.Flag("service").Value.String()
		instanceName := cmd.Flag("instance").Value.String()
		quantity, err := cmd.Flags().GetInt("quantity")
		if err != nil {
			return err
		}
		scale := scaleArgs{service: serviceName, instance: instanceName, 
				quantity: quantity,
				prox: proxy.New(serviceName, instanceName, "POST", &proxy.TsuruServer{}),
		}

		output, err := runScale(scale)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(os.Stdout, output)
		if err != nil {
			return err
		}
		return nil
	},
}

func runScale(scale scaleArgs) (string, error) {
	scale.prox.Path = "/resources/" + scale.instance + "/scale"
	scale.prox.Headers["Content-Type"] = "application/json"
	bodyReq, err := json.Marshal(map[string]string{
		"quantity=": strconv.Itoa(scale.quantity),
	})
	if err != nil {
		return "", err
	}
	scale.prox.Body = bytes.NewBuffer(bodyReq)

	return postScale(scale.prox, scale.quantity)
}

func postScale(prox *proxy.Proxy, quantity int) (string, error) {
	resp, err := prox.ProxyRequest()
	if err != nil {
		return "", err
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusCreated {
		bodyString := string(respBody)
		return "", fmt.Errorf("Status Code: %v\nResponse Body:\n%v", resp.Status, bodyString)
	}
	return fmt.Sprintf("Instance successfully scaled to %d unit(s)\n", quantity), nil
}

func init() {
	rootCmd.AddCommand(scaleCmd)

	scaleCmd.Flags().IntP("quantity", "q", 0, "Quantity of units to scale")
	scaleCmd.Flags().StringP("service", "s", "", "Service name")
	scaleCmd.Flags().StringP("instance", "i", "", "Service instance name")
	scaleCmd.MarkFlagRequired("service")
	scaleCmd.MarkFlagRequired("instance")
	scaleCmd.MarkFlagRequired("quantity")
}
