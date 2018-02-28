package main

import (
	"os"
	"io/ioutil"
	"github.com/norand94/bitcoin-address-info/app/config"
	"gopkg.in/yaml.v2"
	"github.com/norand94/bitcoin-address-info/app"
)

func main()  {
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