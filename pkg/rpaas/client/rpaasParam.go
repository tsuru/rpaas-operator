package client

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"strconv"
)

type Parameters struct {
	instance string
}

func (p *Parameters) SetInstance(s string) {
	p.instance = s
}

type InfoParams struct {
	instance *string
}

func (p *InfoParams) SetInstance(s string) {
	p.instance = &s
}

type CertificateParams struct {
	Parameters
	certificate string
	key         string
	destName    string
}

func (cp *CertificateParams) SetCertificate(filePath string) {
	cp.certificate = filePath
}

func (cp *CertificateParams) SetKey(filePath string) {
	cp.key = filePath
}

func (cp *CertificateParams) SetDestination(filePath string) {
	cp.destName = filePath
}

func (cp *CertificateParams) encode() (string, string, error) {
	certBytes, err := ioutil.ReadFile(cp.certificate)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to read certificate file: %v", err)
	}

	keyFile, err := ioutil.ReadFile(cp.key)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to read key file: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	certPart, err := writer.CreateFormFile("cert", cp.certificate)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to create certificate form file: %v", err)
	}

	_, err = certPart.Write(certBytes)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to write the certificate to the file: %v", err)
	}

	keyPart, err := writer.CreateFormFile("key", cp.key)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to create key form file: %v", err)
	}
	_, err = keyPart.Write(keyFile)
	if err != nil {
		return "", "", fmt.Errorf("Error while trying to write the key to the file: %v", err)
	}

	writer.WriteField("name", cp.destName)
	err = writer.Close()
	if err != nil {
		return "", "", fmt.Errorf("Error while closing file: %v", err)
	}

	return body.String(), writer.Boundary(), nil
}

type ScaleParams struct {
	Parameters
	replicas int32
}

func (sp *ScaleParams) SetReplicas(q int32) {
	sp.replicas = q
}

func (sp *ScaleParams) GetReplicaString() string {
	return strconv.Itoa(int(sp.replicas))
}

type UpdateParams struct {
	Parameters
	Flavors     []string
	PlanOverr   string
	Plan        string
	Team        string
	User        string
	Ip          string
	Tags        []string
	Description string
}

func (up *UpdateParams) SetFlavors(flavors []string) {
	up.Flavors = flavors
}

func (up *UpdateParams) SetTags(tags []string) {
	up.Tags = tags
}

func (up *UpdateParams) SetPlan(plan string) {
	up.Plan = plan
}

func (up *UpdateParams) SetTeam(team string) {
	up.Team = team
}

func (up *UpdateParams) SetUser(user string) {
	up.User = user
}

func (up *UpdateParams) SetIp(ip string) {
	up.Ip = ip
}

func (up *UpdateParams) SetDescription(desc string) {
	up.Description = desc
}

func (up *UpdateParams) SetPlanOverride(plan string) {
	up.PlanOverr = plan
}

func NewUpdateParams() UpdateParams {
	return UpdateParams{
		Plan: "default",
	}
}

func (up *UpdateParams) validateUpdateArgs() error {
	if up.instance == "" || up.Plan == "" || up.Team == "" || up.User == "" {
		return fmt.Errorf("must provide a valid instance name, plan, team and user")
	}
	return nil
}
