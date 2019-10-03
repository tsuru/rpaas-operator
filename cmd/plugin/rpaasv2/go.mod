module github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2

go 1.12

require (
	github.com/NYTimes/gziphandler v1.0.1 // indirect
	github.com/bradfitz/go-smtpd v0.0.0-20170404230938-deb6d6237625 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/gophercloud/gophercloud v0.1.0 // indirect
	github.com/gorilla/context v1.1.1 // indirect
	github.com/kisielk/errcheck v1.2.0 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/olekukonko/tablewriter v0.0.1
	github.com/spf13/cobra v0.0.5
	github.com/tsuru/rpaas-operator v0.4.0 // indirect
	github.com/tsuru/tsuru v0.0.0-20190917161403-b6b3f8bee958
	gotest.tools v2.2.0+incompatible
)

replace (
	github.com/docker/docker => github.com/docker/engine v0.0.0-20190219214528-cbe11bdc6da8
	github.com/docker/machine => github.com/cezarsa/machine v0.7.1-0.20190219165632-cdcfd549f935
	github.com/rancher/kontainer-engine => github.com/cezarsa/kontainer-engine v0.0.4-dev.0.20190725184110-8b6c46d5dadd
)
