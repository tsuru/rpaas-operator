package client

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"mime/multipart"
)

type Instance struct {
	Name string
}

type InfoInstance struct {
	Name *string
}

type CertificateInstance struct {
	Instance
	Certificate string
	Key         string
	DestName    string
}

func (cp *CertificateInstance) encode() (string, string, error) {
	certBytes, err := ioutil.ReadFile(cp.Certificate)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to read certificate file: %v", err)
	}

	keyFile, err := ioutil.ReadFile(cp.Key)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to read key file: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	certPart, err := writer.CreateFormFile("cert", cp.Certificate)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to create certificate form file: %v", err)
	}

	_, err = certPart.Write(certBytes)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to write the certificate to the file: %v", err)
	}

	keyPart, err := writer.CreateFormFile("key", cp.Key)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to create key form file: %v", err)
	}
	_, err = keyPart.Write(keyFile)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to write the key to the file: %v", err)
	}

	writer.WriteField("name", cp.DestName)
	err = writer.Close()
	if err != nil {
		return "", "", fmt.Errorf("Error while closing file: %v", err)
	}

	return body.String(), writer.Boundary(), nil
}

type ScaleInstance struct {
	Instance
	Replicas int32
}

type UpdateInstance struct {
	Instance
	Flavors     []string
	Tags        []string
	PlanOverr   string
	Ip          string
	Description string
	User        string
	Team        string
	Plan        string
}

func (up *UpdateInstance) validate() error {
	if up.Name == "" || up.Team == "" {
		return fmt.Errorf("must provide a valid instance name, plan, team and user")
	}
	return nil
}
