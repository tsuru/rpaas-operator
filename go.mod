module github.com/tsuru/rpaas-operator

go 1.24.0

toolchain go1.24.5

require (
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/ajg/form v1.5.1
	github.com/alecthomas/participle/v2 v2.1.4
	github.com/cert-manager/cert-manager v1.12.14
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc
	github.com/evanphx/json-patch/v5 v5.6.0
	github.com/fatih/color v1.18.0
	github.com/fsnotify/fsnotify v1.7.0
	github.com/globocom/echo-prometheus v0.1.2
	github.com/go-logr/logr v1.3.0
	github.com/google/gops v0.3.12
	github.com/gorilla/websocket v1.5.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/imdario/mergo v0.3.13
	github.com/kedacore/keda/v2 v2.10.1
	github.com/labstack/echo/v4 v4.10.2
	github.com/lnquy/cron v1.1.1
	github.com/mitchellh/mapstructure v1.5.0
	github.com/olekukonko/tablewriter v1.1.3
	github.com/opentracing-contrib/go-stdlib v1.0.1-0.20201028152118-adbfc141dfc2
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.16.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.10.0
	github.com/stern/stern v1.20.1
	github.com/stretchr/testify v1.11.1
	github.com/tsuru/go-tsuruclient v0.0.0-20230724221758-523def58dd2d
	github.com/tsuru/nginx-operator v0.15.2-0.20240515194244-a38b4b58e866
	github.com/uber/jaeger-client-go v2.25.0+incompatible
	github.com/urfave/cli/v3 v3.6.2
	golang.org/x/exp v0.0.0-20220722155223-a9213eeb770e
	golang.org/x/mod v0.32.0
	golang.org/x/net v0.49.0
	golang.org/x/term v0.39.0
	golang.org/x/time v0.3.0
	k8s.io/api v0.29.7
	k8s.io/apimachinery v0.29.7
	k8s.io/client-go v0.29.7
	k8s.io/klog/v2 v2.110.1
	k8s.io/kubectl v0.26.7
	k8s.io/metrics v0.26.7
	k8s.io/utils v0.0.0-20240502163921-fe8a2dddb1d0
	sigs.k8s.io/controller-runtime v0.15.0
	sigs.k8s.io/go-open-service-broker-client/v2 v2.0.0-20200925085050-ae25e62aaf10
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Azure/go-ntlmssp v0.0.0-20220621081337-cb9428e4ac1e // indirect
	github.com/HdrHistogram/hdrhistogram-go v1.0.0 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.1.1 // indirect
	github.com/antihax/optional v1.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/clipperhouse/displaywidth v0.6.2 // indirect
	github.com/clipperhouse/stringish v0.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.3.0 // indirect
	github.com/docker/docker v20.10.20+incompatible // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/evanphx/json-patch v5.7.0+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/fvbommel/sortorder v1.0.1 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.4 // indirect
	github.com/go-errors/errors v1.0.1 // indirect
	github.com/go-ldap/ldap/v3 v3.4.4 // indirect
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-version v0.0.0-20180716215031-270f2f71b1ee // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/howeyc/fsnotify v0.9.0 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/labstack/gommon v0.4.0 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.19 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/olekukonko/cat v0.0.0-20250911104152-50322a0618f6 // indirect
	github.com/olekukonko/errors v1.1.0 // indirect
	github.com/olekukonko/ll v0.1.4-0.20260115111900-9e59c2286df0 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.10.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sajari/fuzzy v1.0.0 // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/cobra v1.8.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/tsuru/config v0.0.0-20201023175036-375aaee8b560 // indirect
	github.com/tsuru/gnuflag v0.0.0-20151217162021-86b8c1b864aa // indirect
	github.com/tsuru/tablecli v0.0.0-20190131152944-7ded8a3383c6 // indirect
	github.com/tsuru/tsuru v0.0.0-20230721211340-f41fb455f8c3 // indirect
	github.com/uber/jaeger-lib v2.4.0+incompatible // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/xlab/treeprint v1.1.0 // indirect
	go.starlark.net v0.0.0-20200306205701-8dd3e2ee1dd5 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/crypto v0.47.0 // indirect
	golang.org/x/oauth2 v0.27.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/tools v0.41.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.3.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.66.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gotest.tools/v3 v3.5.2 // indirect
	k8s.io/apiextensions-apiserver v0.29.7 // indirect
	k8s.io/cli-runtime v0.26.7 // indirect
	k8s.io/component-base v0.29.7 // indirect
	k8s.io/kube-aggregator v0.29.7 // indirect
	k8s.io/kube-openapi v0.0.0-20231010175941-2dd684a91f00 // indirect
	knative.dev/pkg v0.0.0-20230306194819-b77a78c6c0ad // indirect
	sigs.k8s.io/gateway-api v0.7.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/kustomize/api v0.12.1 // indirect
	sigs.k8s.io/kustomize/kyaml v0.13.9 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace github.com/stern/stern => github.com/tsuru/stern v1.20.2-0.20210928180051-1157b938dc3f
