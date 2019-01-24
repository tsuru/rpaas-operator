package controller

import (
	"github.com/tsuru/rpaas-operator/pkg/controller/rpaasinstance"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, rpaasinstance.Add)
}
