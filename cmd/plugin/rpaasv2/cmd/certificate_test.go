package cmd

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2/proxy"
	"gotest.tools/assert"
)

type testCase struct {
	name     string
	cert     certificateArgs
	handler  http.HandlerFunc
	certPem  string
	keyPem   string
	boundary string
}

func TestPostValidCertificate(t *testing.T) {
	testCase := testCase{
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

		cert: certificateArgs{
			service:     "test-service",
			instance:    "test-instance",
			certificate: "../tmp/cert.cert",
			key:         "../tmp/key.cert",
		},
		handler: func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Method, "POST")
			assert.Equal(t, "/services/test-service/proxy/test-instance?callback=/resources/test-instance/certificate", r.URL.RequestURI())
			partReader, err := r.MultipartReader()
			assert.NilError(t, err)
			certPart, err := partReader.NextPart()
			assert.NilError(t, err)
			certBytes, err := ioutil.ReadAll(certPart)
			assert.NilError(t, err)
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
			assert.NilError(t, err)
			keyBytes, err := ioutil.ReadAll(keyPart)
			assert.NilError(t, err)
			assert.Equal(t, `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`, string(keyBytes))
			w.WriteHeader(http.StatusOK)
		},
	}
	t.Run(testCase.name, func(t *testing.T) {
		ts := httptest.NewServer(testCase.handler)
		testCase.cert.prox = proxy.New(testCase.cert.service, testCase.cert.instance, "POST", &mockServer{ts: ts})
		testCase.cert.prox.Path = "/resources/test-instance/certificate"
		defer ts.Close()
		err := createCert(`-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`)
		assert.NilError(t, err)
		err = createKey(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`)
		body, boundary, err := encodeBody(testCase.cert)
		assert.NilError(t, err)
		err = postCertificate(testCase.cert.prox, body, boundary)
		assert.NilError(t, err)
		err = removeTmpFolder()
		assert.NilError(t, err)
	})
}

func createCert(cert string) error {
	if _, err := os.Stat("../tmp"); err != nil {
		if os.IsNotExist(err) {
			os.Mkdir("../tmp", os.ModePerm)
		}
	}
	f, err := os.Create("../tmp/cert.cert")
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
	if _, err := os.Stat("../tmp"); err != nil {
		if os.IsNotExist(err) {
			os.Mkdir("../tmp", os.ModePerm)
		}
	}
	f, err := os.Create("../tmp/key.cert")
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
	return os.RemoveAll("../tmp")
}
