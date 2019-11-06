package controller

import (
	"nuodb/nuodb-operator/pkg/controller/nuodbycsbwl"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, nuodbycsbwl.Add)
}
