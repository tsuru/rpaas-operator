module github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2

go 1.13

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/google/go-cmp v0.3.0 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/mattn/go-runewidth v0.0.5 // indirect
	github.com/olekukonko/tablewriter v0.0.1
	github.com/pkg/errors v0.8.1 // indirect
	github.com/spf13/cobra v0.0.5 // indirect
	github.com/stretchr/testify v1.4.0
	github.com/tsuru/rpaas-operator/pkg/rpaas/client v0.0.0-00000000000000-000000000000
	github.com/urfave/cli v1.22.1
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gotest.tools v2.2.0+incompatible
)

replace (
	github.com/docker/docker => github.com/docker/engine v0.0.0-20190219214528-cbe11bdc6da8
	github.com/docker/machine => github.com/cezarsa/machine v0.7.1-0.20190219165632-cdcfd549f935
	github.com/rancher/kontainer-engine => github.com/cezarsa/kontainer-engine v0.0.4-dev.0.20190725184110-8b6c46d5dadd
	github.com/tsuru/rpaas-operator/pkg/rpaas/client => ../../../pkg/rpaas/client
)
