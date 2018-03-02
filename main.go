package main

import (
	"io/ioutil"
	"os"

	"github.com/norand94/bitcoin-address-info/app"
	"github.com/norand94/bitcoin-address-info/app/config"
	"gopkg.in/yaml.v2"
)

func main() {
	file := os.Getenv("APP_CONFIG")
	if file == "" {
		file = "conf.yaml"
	}

	bts, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}

	conf := new(config.M)
	err = yaml.Unmarshal(bts, conf)
	if err != nil {
		panic(err)
	}

	app.Run(conf)
}
