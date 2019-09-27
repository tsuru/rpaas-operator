package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/tsuru/rpaas-operator/build/cli/proxy"
	"github.com/tsuru/rpaas-operator/build/cli/tablewriter"
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
	tablewriter.WriteInfo(infoType, helperSlice)

	return nil
}
