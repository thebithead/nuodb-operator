package controller

import (
	"nuodb/nuodb-operator/pkg/controller/nuodbinsightsserver"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, nuodbinsightsserver.Add)
}
