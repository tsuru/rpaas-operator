// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/proxy"
)

func init() {
	rootCmd.AddCommand(certificateCmd)

	certificateCmd.Flags().StringP("service", "s", "", "Service name")
	certificateCmd.Flags().StringP("instance", "i", "", "Service instance name")
	certificateCmd.Flags().StringP("certificate", "c", "", "Certificate file name")
	certificateCmd.Flags().StringP("key", "k", "", "Key file name")
	certificateCmd.MarkFlagRequired("service")
}

type certificateArgs struct {
	service     string
	instance    string
	certificate string
	key         string
	prox        *proxy.Proxy
}

var certificateCmd = &cobra.Command{
	Use:   "certificate",
	Short: "Sends certificate + private key to existing instance",
	Long: `Given a certificate and private key located in the filesystem, send them both to the existing instance.
The rpaas instance can now be accessed via HTPPS`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.ParseFlags(args)
		service := cmd.Flag("service").Value.String()
		instance := cmd.Flag("instance").Value.String()
		certificate := cmd.Flag("certificate").Value.String()
		key := cmd.Flag("key").Value.String()

		certInst := certificateArgs{
			service:     service,
			instance:    instance,
			certificate: certificate,
			key:         key,
			prox:        proxy.New(service, instance, "POST", &proxy.TsuruServer{}),
		}

		return runCert(certInst)
	},
}

func encodeBody(certInst certificateArgs) (string, string, error) {
	certBytes, err := ioutil.ReadFile(certInst.certificate)
	if err != nil {
		return "", "", err
	}

	keyFile, err := ioutil.ReadFile(certInst.key)
	if err != nil {
		return "", "", err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	certPart, err := writer.CreateFormFile("cert", certInst.certificate)
	if err != nil {
		return "", "", err
	}

	_, err = certPart.Write(certBytes)
	if err != nil {
		return "", "", err
	}

	keyPart, err := writer.CreateFormFile("key", certInst.key)
	if err != nil {
		return "", "", err
	}
	_, err = keyPart.Write(keyFile)
	if err != nil {
		return "", "", err
	}

	err = writer.Close()
	if err != nil {
		return "", "", err
	}

	return body.String(), writer.Boundary(), nil
}

func runCert(certInst certificateArgs) error {
	certInst.prox.Path = "/resources/" + certInst.instance + "/certificate"
	body, boundary, err := encodeBody(certInst)
	if err != nil {
		return err
	}
	if err := postCertificate(certInst.prox, body, boundary); err != nil {
		return err
	}

	fmt.Printf("Certificate successfully updated\n")
	return nil
}

func postCertificate(prox *proxy.Proxy, body, boundary string) error {
	prox.Body = strings.NewReader(body)
	prox.Headers["Content-Type"] = "multipart/form-data; boundary=" + boundary
	resp, err := prox.ProxyRequest()
	if err != nil {
		return err
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		bodyString := string(respBody)
		return fmt.Errorf("Status Code: %v\nResponse Body:\n%v", resp.Status, bodyString)
	}
	return nil
}
