package main

import (
	"thesgo/config"
	"thesgo/matrix"
)

type Thesgo struct {
	wrapper *matrix.ClientWrapper
	config  *config.Config
}
