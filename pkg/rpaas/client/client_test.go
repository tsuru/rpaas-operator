// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/types"
)

func TestNewClient(t *testing.T) {
	t.Run("creating new client", func(t *testing.T) {
		rpaasClient := New("http://fakehosturi.example.com")
		require.NotNil(t, rpaasClient)
		assert.Equal(t, rpaasClient.hostAPI, "http://fakehosturi.example.com")
		assert.NotNil(t, rpaasClient.httpClient)
	})
}

func TestRpaasClient_Scale(t *testing.T) {
	testCases := []struct {
		name      string
		scaleInst ScaleInstance
		assertion func(t *testing.T, err error)
		handler   http.HandlerFunc
	}{
		{
			name: "passing nil instance",
			assertion: func(t *testing.T, err error) {
				assert.Equal(t, fmt.Errorf("instance can't be nil"), err)
			},
		},
		{
			name:      "passing invalid number of replicas",
			scaleInst: ScaleInstance{Instance: Instance{Name: "test-instance"}, Replicas: int32(-1)},
			assertion: func(t *testing.T, err error) {
				assert.Equal(t, fmt.Errorf("replicas number must be greater or equal to zero"), err)
			},
		},
		{
			name:      "testing valid request",
			scaleInst: ScaleInstance{Instance: Instance{Name: "test-instance"}, Replicas: int32(2)},
			assertion: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.RequestURI(), "/resources/test-instance/scale")
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.Equal(t, string(bodyBytes), "quantity=2")
				w.WriteHeader(http.StatusCreated)
			},
		},
		{
			name:      "testing error response from handler",
			scaleInst: ScaleInstance{Instance: Instance{Name: "test-instance"}, Replicas: int32(2)},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.RequestURI(), "/resources/test-instance/scale")
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.Equal(t, string(bodyBytes), "quantity=2")
				w.Write([]byte("Some Error"))
				w.WriteHeader(http.StatusBadRequest)
			},
			assertion: func(t *testing.T, err error) {
				assert.Equal(t, fmt.Errorf("unexpected status code: body: Some Error"), err)
			},
		},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			inst := tt.scaleInst
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest := &RpaasClient{httpClient: &http.Client{}, hostAPI: sv.URL}
			err := clientTest.Scale(context.TODO(), inst)
			tt.assertion(t, err)
		})
	}
}

func TestScaleWithTsuru(t *testing.T) {
	type testStruct struct {
		name      string
		scaleInst ScaleInstance
		assertion func(t *testing.T, err error, clientTest *RpaasClient, tt testStruct)
		handler   http.HandlerFunc
	}
	testCases := []testStruct{
		{
			name:      "testing with existing service and string in tsuru target",
			scaleInst: ScaleInstance{Instance: Instance{Name: "test-instance"}, Replicas: int32(1)},
			assertion: func(t *testing.T, err error, clientTest *RpaasClient, tt testStruct) {
				assert.NoError(t, err)
				inst := tt.scaleInst
				assert.Equal(t, nil, clientTest.Scale(context.TODO(), inst))
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, "/services/example-service/proxy/test-instance?callback=/resources/test-instance/scale", r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.Equal(t, string(bodyBytes), "quantity=1")
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte("Instance successfully scaled to 1 unit(s)"))
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest, err := NewTsuruClient(sv.URL, "example-service", "f4k3t0k3n")
			tt.assertion(t, err, clientTest, tt)
		})
	}
}

func TestGetPlansTroughHostAPI(t *testing.T) {
	testCases := []struct {
		name      string
		infoInst  func() InfoInstance
		assertion func(t *testing.T, plans []types.Plan, err error)
		handler   http.HandlerFunc
	}{
		{
			name: "valid request",
			infoInst: func() InfoInstance {
				name := "test-instance"
				return InfoInstance{Name: &name}
			},
			assertion: func(t *testing.T, plans []types.Plan, err error) {
				assert.NoError(t, err)
				expectedPlans := []types.Plan{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
						Default:     true,
					},
				}
				assert.Equal(t, expectedPlans, plans)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.RequestURI(), "/resources/test-instance/plans")
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.NotNil(t, bodyBytes)
				w.Header().Set("Content-Type", "application/json")
				helper := []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					Default     bool   `json:"default"`
				}{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
						Default:     true,
					},
				}
				body, err := json.Marshal(helper)
				require.NoError(t, err)
				w.Write(body)
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "some error returned on the request",
			infoInst: func() InfoInstance {
				name := "test-instance"
				return InfoInstance{Name: &name}
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.RequestURI(), "/resources/test-instance/plans")
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.NotNil(t, bodyBytes)
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Some Error"))
			},
			assertion: func(t *testing.T, plans []types.Plan, err error) {
				assert.Error(t, fmt.Errorf("unexpected status code: body: Some Error"), err)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest := &RpaasClient{httpClient: &http.Client{}, hostAPI: sv.URL}
			plans, err := clientTest.GetPlans(context.TODO(), tt.infoInst())
			tt.assertion(t, plans, err)
		})
	}
}

func TestPlansTroughTsuru(t *testing.T) {
	testCases := []struct {
		name      string
		infoInst  func() InfoInstance
		assertion func(t *testing.T, plans []types.Plan, err error)
		handler   http.HandlerFunc
	}{
		{
			name: "testing with existing service and string in tsuru target",
			infoInst: func() InfoInstance {
				name := "test-instance"
				return InfoInstance{Name: &name}
			},
			assertion: func(t *testing.T, plans []types.Plan, err error) {
				assert.NoError(t, err)
				expectedPlans := []types.Plan{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
						Default:     true,
					},
				}
				assert.Equal(t, expectedPlans, plans)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, "/services/example-service/proxy/test-instance?callback=/resources/test-instance/plans", r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				_, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				helper := []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					Default     bool   `json:"default"`
				}{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
						Default:     true,
					},
				}

				body, err := json.Marshal(helper)
				require.NoError(t, err)
				w.Write(body)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest, err := NewTsuruClient(sv.URL, "example-service", "f4k3t0k3n")
			plans, err := clientTest.GetPlans(context.TODO(), tt.infoInst())
			tt.assertion(t, plans, err)
		})
	}
}

func TestGetFlavorsTroughHostAPI(t *testing.T) {
	testCases := []struct {
		name      string
		infoInst  func() InfoInstance
		assertion func(t *testing.T, flavors []types.Flavor, err error)
		handler   http.HandlerFunc
	}{
		{
			name: "valid request",
			infoInst: func() InfoInstance {
				name := "test-instance"
				return InfoInstance{Name: &name}
			},
			assertion: func(t *testing.T, flavors []types.Flavor, err error) {
				assert.NoError(t, err)
				expectedFlavors := []types.Flavor{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
					},
				}
				assert.Equal(t, expectedFlavors, flavors)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.RequestURI(), "/resources/test-instance/flavors")
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.NotNil(t, bodyBytes)
				w.Header().Set("Content-Type", "application/json")
				helper := []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				}{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
					},
				}
				body, err := json.Marshal(helper)
				require.NoError(t, err)
				w.Write(body)
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "some error returned on the request",
			infoInst: func() InfoInstance {
				name := "test-instance"
				return InfoInstance{Name: &name}
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.RequestURI(), "/resources/test-instance/flavors")
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.NotNil(t, bodyBytes)
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Some Error"))
			},
			assertion: func(t *testing.T, flavors []types.Flavor, err error) {
				assert.Error(t, fmt.Errorf("unexpected status code: body: Some Error"), err)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest := &RpaasClient{httpClient: &http.Client{}, hostAPI: sv.URL}
			flavors, err := clientTest.GetFlavors(context.TODO(), tt.infoInst())
			tt.assertion(t, flavors, err)
		})
	}
}

func TestFlavorsTroughTsuru(t *testing.T) {
	testCases := []struct {
		name      string
		infoInst  func() InfoInstance
		assertion func(t *testing.T, flavors []types.Flavor, err error)
		handler   http.HandlerFunc
	}{
		{
			name: "testing with existing service and string in tsuru target",
			infoInst: func() InfoInstance {
				name := "test-instance"
				return InfoInstance{Name: &name}
			},
			assertion: func(t *testing.T, flavors []types.Flavor, err error) {
				assert.NoError(t, err)
				expectedPlans := []types.Flavor{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
					},
				}
				assert.Equal(t, expectedPlans, flavors)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, "/services/example-service/proxy/test-instance?callback=/resources/test-instance/flavors", r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				_, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				helper := []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				}{
					{
						Name:        "dsr",
						Description: "rpaas dsr",
					},
				}

				body, err := json.Marshal(helper)
				require.NoError(t, err)
				w.Write(body)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest, err := NewTsuruClient(sv.URL, "example-service", "f4k3t0k3n")
			flavors, err := clientTest.GetFlavors(context.TODO(), tt.infoInst())
			tt.assertion(t, flavors, err)
		})
	}
}

func TestCertificateTroughTsuru(t *testing.T) {
	type testStruct struct {
		name      string
		certPem   string
		keyPem    string
		boundary  string
		certInst  CertificateInstance
		assertion func(t *testing.T, err error)
		handler   http.HandlerFunc
	}
	testCases := []testStruct{
		{
			name: "when a valid key and certificate are passed",

			boundary: "XXXXXXXXXXXXXXX",

			certInst: CertificateInstance{Instance: Instance{Name: "test-instance"}, Certificate: "tmp/cert.cert", Key: "tmp/key.cert", DestName: "test-destination"},

			certPem: `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`,

			keyPem: `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`,
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, "/services/test-service/proxy/test-instance?callback=/resources/test-instance/certificate", r.URL.RequestURI())
				partReader, err := r.MultipartReader()
				assert.NoError(t, err)
				certPart, err := partReader.NextPart()
				assert.NoError(t, err)
				certBytes, err := ioutil.ReadAll(certPart)
				assert.NoError(t, err)
				assert.Equal(t, `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`, string(certBytes))
				keyPart, err := partReader.NextPart()
				assert.NoError(t, err)
				keyBytes, err := ioutil.ReadAll(keyPart)
				assert.NoError(t, err)
				assert.Equal(t, `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`, string(keyBytes))
				w.WriteHeader(http.StatusOK)
			},
			assertion: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// setup
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest, err := NewTsuruClient(sv.URL, "test-service", "f4k3t0k3n")
			assert.NoError(t, err)
			// we create a temporary folder to mock the passed certificate and key
			defer func() {
				err := removeTmpFolder()
				assert.NoError(t, err)
			}()
			err = createCert(tt.certPem)
			assert.NoError(t, err)
			err = createKey(tt.keyPem)
			assert.NoError(t, err)
			// end of setup

			err = clientTest.Certificate(context.TODO(), tt.certInst)
			tt.assertion(t, err)
		})
	}
}

func TestCertificateTroughAPI(t *testing.T) {
	type testStruct struct {
		name      string
		certPem   string
		keyPem    string
		boundary  string
		certInst  CertificateInstance
		initArgs  func() CertificateInstance
		assertion func(t *testing.T, err error)
		handler   http.HandlerFunc
	}
	testCases := []testStruct{
		{
			name: "when a valid key and certificate are passed",

			boundary: "XXXXXXXXXXXXXXX",

			certPem: `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`,

			keyPem: `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`,
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, "/resources/test-instance/certificate", r.URL.RequestURI())
				partReader, err := r.MultipartReader()
				assert.NoError(t, err)
				certPart, err := partReader.NextPart()
				assert.NoError(t, err)
				certBytes, err := ioutil.ReadAll(certPart)
				assert.NoError(t, err)
				assert.Equal(t, `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`, string(certBytes))
				keyPart, err := partReader.NextPart()
				assert.NoError(t, err)
				keyBytes, err := ioutil.ReadAll(keyPart)
				assert.NoError(t, err)
				assert.Equal(t, `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`, string(keyBytes))
				w.WriteHeader(http.StatusOK)
			},
			assertion: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},

			certInst: CertificateInstance{Instance: Instance{Name: "test-instance"}, Certificate: "tmp/cert.cert", Key: "tmp/key.cert", DestName: "test-destination"},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// setup
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest := &RpaasClient{httpClient: &http.Client{}, hostAPI: sv.URL}
			// we create a temporary folder to mock the passed certificate and key
			defer func() {
				err := removeTmpFolder()
				assert.NoError(t, err)
			}()
			err := createCert(tt.certPem)
			assert.NoError(t, err)
			err = createKey(tt.keyPem)
			assert.NoError(t, err)
			// end of setup

			err = clientTest.Certificate(context.TODO(), tt.certInst)
			tt.assertion(t, err)
		})
	}
}

func createCert(cert string) error {
	if _, err := os.Stat("tmp"); err != nil {
		if os.IsNotExist(err) {
			os.Mkdir("tmp", os.ModePerm)
		}
	}
	f, err := os.Create("tmp/cert.cert")
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(cert)
	if err != nil {
		return err
	}
	return nil
}

func createKey(key string) error {
	if _, err := os.Stat("tmp"); err != nil {
		if os.IsNotExist(err) {
			os.Mkdir("tmp", os.ModePerm)
		}
	}
	f, err := os.Create("tmp/key.cert")
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(key)
	if err != nil {
		return err
	}
	return nil
}

func removeTmpFolder() error {
	return os.RemoveAll("tmp")
}

func TestUpdateTroughTsuru(t *testing.T) {
	type testStruct struct {
		name       string
		service    string
		updateInst UpdateInstance
		assertion  func(t *testing.T, err error)
		handler    http.HandlerFunc
	}
	testCases := []testStruct{
		{
			name:       "testing with existing plan and flavor",
			updateInst: UpdateInstance{Instance: Instance{Name: "test-instance"}, Team: "test-team", User: "test-user", Flavors: []string{"test-flavor"}},
			service:    "test-service",
			assertion: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "PUT")
				assert.Equal(t, "/services/test-service/proxy/test-instance?callback=/resources/test-instance", r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.Equal(t, "name=test-instance&plan=&tag=flavor%3Dtest-flavor&team=test-team&user=test-user", string(bodyBytes))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name:       "testing with only plan passed",
			updateInst: UpdateInstance{Instance: Instance{Name: "test-instance"}, Team: "test-team", User: "test-user"},
			service:    "test-service",
			assertion: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "PUT")
				assert.Equal(t, "/services/test-service/proxy/test-instance?callback=/resources/test-instance", r.URL.RequestURI())
				assert.Equal(t, "Bearer f4k3t0k3n", r.Header.Get("Authorization"))
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.Equal(t, "name=test-instance&plan=&team=test-team&user=test-user", string(bodyBytes))
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest, err := NewTsuruClient(sv.URL, tt.service, "f4k3t0k3n")
			assert.NoError(t, err)
			err = clientTest.Update(context.TODO(), tt.updateInst)
			tt.assertion(t, err)
		})
	}
}

func TestUpdateTroughAPI(t *testing.T) {
	type testStruct struct {
		name       string
		service    string
		updateInst UpdateInstance
		assertion  func(t *testing.T, err error)
		handler    http.HandlerFunc
	}
	testCases := []testStruct{
		{
			name:       "testing with existing plan and flavor",
			updateInst: UpdateInstance{Instance: Instance{Name: "test-instance"}, Team: "test-team", User: "test-user", Flavors: []string{"test-flavor"}},
			service:    "test-service",
			assertion: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "PUT")
				assert.Equal(t, "/resources/test-instance", r.URL.RequestURI())
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.Equal(t, "name=test-instance&plan=&tag=flavor%3Dtest-flavor&team=test-team&user=test-user", string(bodyBytes))
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name:       "testing with only plan passed",
			updateInst: UpdateInstance{Instance: Instance{Name: "test-instance"}, Team: "test-team", User: "test-user"},

			service: "test-service",
			assertion: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "PUT")
				assert.Equal(t, "/resources/test-instance", r.URL.RequestURI())
				bodyBytes, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()
				assert.Equal(t, "name=test-instance&plan=&team=test-team&user=test-user", string(bodyBytes))
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sv := httptest.NewServer(tt.handler)
			defer sv.Close()
			clientTest := &RpaasClient{httpClient: &http.Client{}, hostAPI: sv.URL}
			err := clientTest.Update(context.TODO(), tt.updateInst)
			tt.assertion(t, err)
		})
	}
}
