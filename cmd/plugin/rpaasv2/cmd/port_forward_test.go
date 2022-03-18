// Copyright 2020 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	rpaasclient "github.com/tsuru/rpaas-operator/pkg/rpaas/client"
	"github.com/tsuru/rpaas-operator/pkg/rpaas/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	"k8s.io/kubectl/pkg/scheme"
)

type fakePortForwarder struct {
	method string
	url    *url.URL
	pfErr  error
}

type PortForwardArgs struct {
	Pod     string
	Address string
	Port    string
}

func (f *fakePortForwarder) ForwardPorts(method string, url *url.URL, opts PortForwardOptions) error {
	f.method = method
	f.url = url
	return f.pfErr
}

func TestPortForward(t *testing.T) {
	version := "v1"
	tests := []struct {
		name                       string
		args                       []string
		podPath, pfPath, container string
		pod                        *corev1.Pod
		pfErr                      bool
		expected                   string
		expectedError              string
		expectedCalled             bool
		client                     client.Client
	}{
		{
			name: "when port forward is successful",
			args: []string{"./rpaasv2", "port-forward", "-s", "some-service", "-p", "my-pod", "localhost", "127.0.0.1", "-l", "8080"},
			//expected: "sucessul",
			podPath: "/api/" + version + "/pods/my-pods",
			pfPath:  "/api/" + version + "/pods/my-pods/portforward",
			pod:     execPod(),
			client:  &fake.FakeClient{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			tf := cmdtesting.NewTestFactory()
			defer tf.Cleanup()
			codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
			ns := scheme.Codecs.WithoutConversion()
			tf.Client = &kfake.RESTClient{
				VersionedAPIPath:     "/api/v1",
				GroupVersion:         schema.GroupVersion{Group: "", Version: "v1"},
				NegotiatedSerializer: ns,
				Client: kfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					switch p, m := req.URL.Path, req.Method; {
					case p == tt.podPath && m == "GET":
						body := cmdtesting.ObjBody(codec, tt.pod)
						return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: body}, nil
					default:
						t.Errorf("%s: unexpected request: %#v\n%#v", tt.name, req.URL, req)
						return nil, nil
					}
				}),
			}
			tf.ClientConfigVal = cmdtesting.DefaultClientConfig()
			ff := &fakePortForwarder{}
			if tt.pfErr {
				ff.pfErr = fmt.Errorf("pf error")
			}
			var port = []string{"8888"}

			tst := rpaasclient.PortForwardArgs{
				Pod:     "my-pod",
				Address: "127.0.0.1",
				Port:    port,
			}
			opts := &PortForwardOptions{}

			cmd := NewCmdPortForward()

			if err = opts.Complete(tf, cmd, tst); err != nil {
				return
			}

			opts.PortForwarder = ff

			if err = opts.Validate(); err != nil {
				return
			}

			err = opts.portForwa()

			if tt.pfErr && err != ff.pfErr {
				t.Errorf("%s: Unexpected port-forward error: %v", tt.name, err)
			}
			if !tt.pfErr && err != nil {
				t.Errorf("%s: Unexpected error: %v", tt.name, err)
			}
			if tt.pfErr {
				return
			}

			if ff.url == nil || ff.url.Path != tt.pfPath {
				t.Errorf("%s: Did not get expected path for portforward request", tt.name)
			}
			if ff.method != "POST" {
				t.Errorf("%s: Did not get method for attach request: %s", tt.name, ff.method)
			}

		})
	}
}

func execPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod", ResourceVersion: "10"},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			DNSPolicy:     corev1.DNSClusterFirst,
			Containers: []corev1.Container{
				{
					Name: "bar",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}
