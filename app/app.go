package app

import (
	"github.com/norand94/bitcoin-address-info/app/config"
	"github.com/norand94/bitcoin-address-info/app/mongo"
	"github.com/gin-gonic/gin"
	"net/http"
	"log"
	"github.com/globalsign/mgo"
	"encoding/json"
)

type app struct {
	Config *config.M
	MgoCli *mongo.Client
}

func Run(conf *config.M) {
	app := new(app)
	app.Config = conf
	app.MgoCli = mongo.New(conf)


	r := gin.Default()
	r.GET("/address/:address", app.addressHandler)
	r.Run(conf.Port)
}

func (app *app) addressHandler(c *gin.Context) {
	address := c.Param("address")

	if address == "" {
		c.JSON(400, gin.H{
			"err" : "address is not set",
		})
		return
	}


	//app.MgoCli.Find(mongo.AddressInfo, bson.M{"hash" : ""})

	resp, err := http.Get("https://blockchain.info/ru/rawaddr/" + address + "?confirmations=6")
	if err != nil {
		errorResp(c, err)
		return
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	var addressInfo map[string]interface{}
	dec.Decode(&addressInfo)

	err = app.MgoCli.Exec(mongo.AddressInfo, func(c *mgo.Collection) error {
		return c.Insert(addressInfo)
	})
	if err != nil {
		errorResp(c, err)
		return
	}

	c.JSON(200, addressInfo)
}

func errorResp(c *gin.Context, err error) {
	log.Println(err)
	c.JSON(500, gin.H{
		"err" : err,
	})
}

