package controller

import (
	"github.com/tsuru/nginx-operator/pkg/controller/nginx"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, nginx.Add)
}
