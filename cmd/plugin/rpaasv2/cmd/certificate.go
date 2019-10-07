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
	certificateCmd.Flags().StringP("name", "", "default", "Names the provided certificate-key file")
	certificateCmd.MarkFlagRequired("service")
	certificateCmd.MarkFlagRequired("certificate")
	certificateCmd.MarkFlagRequired("key")
	certificateCmd.MarkFlagRequired("instance")
}

type certificateArgs struct {
	service     string
	instance    string
	certificate string
	key         string
	name        string
	prox        *proxy.Proxy
}

var certificateCmd = &cobra.Command{
	Use:   "certificate",
	Short: "Sends certificate + private key to existing instance",
	Long: `Given a certificate and private key located in the filesystem, send them both to the existing instance.
The rpaas instance can now be accessed via HTTPS`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.ParseFlags(args)
		service := cmd.Flag("service").Value.String()
		instance := cmd.Flag("instance").Value.String()
		certificate := cmd.Flag("certificate").Value.String()
		key := cmd.Flag("key").Value.String()
		name := cmd.Flag("name").Value.String()

		certInst := certificateArgs{
			service:     service,
			instance:    instance,
			certificate: certificate,
			key:         key,
			name:        name,
			prox:        proxy.New(service, instance, "POST", &proxy.TsuruServer{}),
		}

		return runCert(certInst)
	},
}

func encodeBody(certInst certificateArgs) (string, string, error) {
	certBytes, err := ioutil.ReadFile(certInst.certificate)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to read certificate file: %v", err)
	}

	keyFile, err := ioutil.ReadFile(certInst.key)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to read key file: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	certPart, err := writer.CreateFormFile("cert", certInst.certificate)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to create certificate form file: %v", err)
	}

	_, err = certPart.Write(certBytes)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to write the certificate to the file: %v", err)
	}

	keyPart, err := writer.CreateFormFile("key", certInst.key)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to create key form file: %v", err)
	}
	_, err = keyPart.Write(keyFile)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to write the key to the file: %v", err)
	}

	writer.WriteField("name", certInst.name)
	err = writer.Close()
	if err != nil {
		return "", "", fmt.Errorf("Error while closing file: %v", err)
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
