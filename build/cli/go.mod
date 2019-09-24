module github.com/tsuru/rpaas-operator/build/cli

go 1.12

require (
	github.com/docker/docker v0.0.0-00010101000000-000000000000
	github.com/mitchellh/go-homedir v1.1.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.3.0
	github.com/tsuru/tsuru v0.0.0-20190917161403-b6b3f8bee958
	gotest.tools v2.2.0+incompatible
)

replace (
	github.com/docker/docker => github.com/docker/engine v0.0.0-20190219214528-cbe11bdc6da8
	github.com/docker/machine => github.com/cezarsa/machine v0.7.1-0.20190219165632-cdcfd549f935
	github.com/rancher/kontainer-engine => github.com/cezarsa/kontainer-engine v0.0.4-dev.0.20190725184110-8b6c46d5dadd
)
