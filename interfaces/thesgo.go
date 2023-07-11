package ifc

import (
	"thesgo/config"
)

type Thesgo interface {
	Matrix() MatrixContainer
	Config() *config.Config

	Start()
	Stop(save bool)
}
