module github.com/tsuru/rpaas-operator

go 1.16

require (
	github.com/HdrHistogram/hdrhistogram-go v1.0.0 // indirect
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/ajg/form v1.5.1
	github.com/cert-manager/cert-manager v1.9.0
	github.com/davecgh/go-spew v1.1.1
	github.com/evanphx/json-patch/v5 v5.1.0
	github.com/fatih/color v1.13.0
	github.com/fsnotify/fsnotify v1.5.1
	github.com/globocom/echo-prometheus v0.1.2
	github.com/go-logr/logr v1.2.2
	github.com/google/go-containerregistry v0.8.0
	github.com/google/gops v0.3.12
	github.com/gorilla/websocket v1.4.2
	github.com/hashicorp/go-multierror v1.1.1
	github.com/imdario/mergo v0.3.12
	github.com/labstack/echo/v4 v4.6.1
	github.com/mitchellh/mapstructure v1.4.3
	github.com/olekukonko/tablewriter v0.0.5
	github.com/opentracing-contrib/go-stdlib v1.0.0
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.12.1
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.10.0
	github.com/stern/stern v1.20.1
	github.com/stretchr/testify v1.7.1
	github.com/tsuru/nginx-operator v0.13.1
	github.com/uber/jaeger-client-go v2.25.0+incompatible
	github.com/uber/jaeger-lib v2.4.0+incompatible // indirect
	github.com/urfave/cli/v2 v2.3.0
	github.com/willf/bitset v1.1.11
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	k8s.io/api v0.24.2
	k8s.io/apimachinery v0.24.2
	k8s.io/client-go v0.24.2
	k8s.io/kubectl v0.24.2
	k8s.io/metrics v0.24.2
	sigs.k8s.io/controller-runtime v0.12.3
	sigs.k8s.io/go-open-service-broker-client/v2 v2.0.0-20200925085050-ae25e62aaf10
)

replace github.com/stern/stern => github.com/tsuru/stern v1.20.2-0.20210928180051-1157b938dc3f
