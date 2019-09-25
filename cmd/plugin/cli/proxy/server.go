package proxy

import tsuruCmd "github.com/tsuru/tsuru/cmd"

type Server interface {
	GetTarget() (string, error)
	GetURL(path string) (string, error)
	ReadToken() (string, error)
}

type TsuruServer struct{}

func (t *TsuruServer) GetTarget() (string, error) {
	return tsuruCmd.GetTarget()
}

func (t *TsuruServer) GetURL(path string) (string, error) {
	return tsuruCmd.GetURL(path)
}

func (t *TsuruServer) ReadToken() (string, error) {
	return tsuruCmd.ReadToken()
}
