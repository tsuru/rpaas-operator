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

func MakeInstance(name string) Instance {
	return Instance{Name: name}
}

type InfoInstance struct {
	Name *string
}

func MakeInfoInstance(name string) InfoInstance {
	s := &name
	return InfoInstance{Name: s}
}

type CertificateInstance struct {
	Instance
	Certificate string
	Key         string
	DestName    string
}

func MakeCertificateInstance(name, certificate, key, destName string) CertificateInstance {
	inst := CertificateInstance{Certificate: certificate, Key: key, DestName: destName}
	inst.Name = name
	return inst
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

func MakeScaleInstance(name string, replicas int32) ScaleInstance {
	inst := ScaleInstance{Replicas: replicas}
	inst.Name = name
	return inst
}

type UpdateInstance struct {
	Instance
	Flavors []string
	Flags   map[string]string
	Tags    []string
}

func MakeUpdateInstance(name string, flavors, tags []string) UpdateInstance {
	tmpMap := make(map[string]string)
	tmpMap["PlanOverr"] = ""
	tmpMap["Plan"] = ""
	tmpMap["Team"] = ""
	tmpMap["User"] = ""
	tmpMap["Ip"] = ""
	tmpMap["Description"] = ""

	inst := UpdateInstance{Flags: tmpMap, Flavors: flavors, Tags: tags}
	inst.Name = name

	return inst
}

func (up *UpdateInstance) validateUpdate() error {
	if up.Name == "" || up.Flags["Team"] == "" {
		return fmt.Errorf("must provide a valid instance name, plan, team and user")
	}
	return nil
}
