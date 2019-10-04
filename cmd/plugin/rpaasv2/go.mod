module github.com/tsuru/rpaas-operator/cmd/plugin/rpaasv2

go 1.12

require (
	github.com/docker/docker v0.7.3-0.20190327010347-be7ac8be2ae0 // indirect
	github.com/docker/go-units v0.3.3 // indirect
	github.com/howeyc/fsnotify v0.9.0 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/olekukonko/tablewriter v0.0.1
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.4.0 // indirect
	github.com/tsuru/config v0.0.0-20180418191556-87403ee7da02 // indirect
	github.com/tsuru/tsuru v0.0.0-20190917161403-b6b3f8bee958
	golang.org/x/sys v0.0.0-20190813064441-fde4db37ae7a // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gotest.tools v2.2.0+incompatible
)

replace (
	github.com/docker/docker => github.com/docker/engine v0.0.0-20190219214528-cbe11bdc6da8
	github.com/docker/machine => github.com/cezarsa/machine v0.7.1-0.20190219165632-cdcfd549f935
	github.com/rancher/kontainer-engine => github.com/cezarsa/kontainer-engine v0.0.4-dev.0.20190725184110-8b6c46d5dadd
)
