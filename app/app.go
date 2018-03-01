package app

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/norand94/bitcoin-address-info/app/config"
	"github.com/norand94/bitcoin-address-info/app/mongo"
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
			"err": "address is not set",
		})
		return
	}
	fmt.Println(address)
	var addressInfo = make(map[string]interface{})
	app.MgoCli.FindOne(mongo.Address, bson.M{"address": address}, &addressInfo)

	if len(addressInfo) == 0 {
		resp, err := http.Get("https://blockchain.info/ru/rawaddr/" + address + "?confirmations=6")
		if err != nil {
			errorResp(c, err)
			return
		}
		defer resp.Body.Close()

		dec := json.NewDecoder(resp.Body)

		err = dec.Decode(&addressInfo)
		if err != nil {
			errorResp(c, err)
			return
		}

		err = app.MgoCli.Exec(mongo.Address, func(c *mgo.Collection) error {
			return c.Insert(addressInfo)
		})
		if err != nil {
			errorResp(c, err)
			return
		}
		addressInfo["source"] = "bitcoin.info"
	} else {
		addressInfo["source"] = "cache"
	}

	err := app.loadBlocks(&addressInfo)
	if err != nil {
		errorResp(c, err)
		return
	}

	c.JSON(200, addressInfo)
}

func (app *app) loadBlocks(addressInfo *map[string]interface{}) error {
	blocks := make(map[int]map[string]interface{})

	address := *addressInfo
	txs := address["txs"].([]interface{})
	for _, tx := range txs {
		m := tx.(map[string]interface{})
		blockHeight := int(m["block_height"].(float64))

		if _, exists := blocks[blockHeight]; !exists {
			block := make(map[string]interface{})
			app.MgoCli.FindOne(mongo.Block, bson.M{"height": blockHeight}, &block)

			if len(block) == 0 {
				var err error
				block, err = getBlockFromApi(blockHeight)
				if err != nil {
					return err
				}

				err = app.MgoCli.Exec(mongo.Block, func(c *mgo.Collection) error {
					return c.Insert(block)
				})
				if err != nil {
					return err
				}
			}
			blocks[blockHeight] = block

		}
		m["block"] = blocks[blockHeight]

	}
	return nil
}

func getBlockFromApi(blockHeight int) (map[string]interface{}, error) {
	log.Println("Getting block from api: ", blockHeight)
	resp, err := http.Get("https://blockchain.info/ru/block-height/" + strconv.Itoa(blockHeight) + "?format=json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)

	var block map[string]interface{}
	err = dec.Decode(&block)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func errorResp(c *gin.Context, err error) {
	log.Println(err)
	c.JSON(500, gin.H{
		"err": err.Error(),
	})
	panic(err)
}
