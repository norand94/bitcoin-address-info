package app

import (
	"github.com/norand94/bitcoin-address-info/app/config"
	"github.com/globalsign/mgo"
)

type app struct {
	Config *config.M
	MgoSession *mgo.Session
}

func Run(conf *config.M) {
	app := new(app)
	app.Config = conf

}
