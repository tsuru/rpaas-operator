module github.com/tsuru/rpaas-operator

go 1.13

require (
	github.com/ajg/form v1.5.1
	github.com/davecgh/go-spew v1.1.1
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-logr/logr v0.2.1 // indirect
	github.com/go-logr/zapr v0.2.0 // indirect
	github.com/go-openapi/spec v0.19.3
	github.com/google/gops v0.3.12
	github.com/gorilla/websocket v1.4.0
	github.com/imdario/mergo v0.3.9
	github.com/labstack/echo/v4 v4.1.17
	github.com/mitchellh/mapstructure v1.1.2
	github.com/olekukonko/tablewriter v0.0.4
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.4.0
	github.com/tsuru/nginx-operator v0.6.0
	github.com/urfave/cli/v2 v2.0.0
	github.com/willf/bitset v1.1.11
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0 // indirect
	golang.org/x/net v0.0.0-20200822124328-c89045814202
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	k8s.io/kube-openapi v0.0.0-20200805222855-6aeccd4b50c6
	k8s.io/kubectl v0.19.2
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/go-open-service-broker-client/v2 v2.0.0-20200925085050-ae25e62aaf10
)
