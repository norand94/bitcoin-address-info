package app

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/garyburd/redigo/redis"

	"github.com/gin-gonic/gin"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/norand94/bitcoin-address-info/app/config"
	"github.com/norand94/bitcoin-address-info/app/models"
	"github.com/norand94/bitcoin-address-info/app/mongo"
)

type app struct {
	Config *config.M
	MgoCli *mongo.Client
	RConn  redis.Conn
}

func Run(conf *config.M) {
	app := new(app)
	app.Config = conf
	app.MgoCli = mongo.New(conf)

	var err error
	app.RConn, err = redis.Dial("tcp", conf.Redis.Address)
	if err != nil {
		log.Fatalln("Не удалось подключиться к Redis")
		panic(err)
	}

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

	addrBts, err := app.RConn.Do("GET", "address:"+address)
	if err != nil {
		log.Println(err)
	}

	if addrBts != nil {
		log.Println("Redis cache")
		c.Writer.Header().Add("Content-Type", "application/json; charset=utf-8")
		c.Writer.Write(addrBts.([]byte))

		return
	}

	addressInfo := new(models.Address)

	if addressInfo.Hash160 == "" {
		log.Println("get address from api")

		resp, err := http.Get("https://blockchain.info/ru/rawaddr/" + address + "?confirmations=6")
		if err != nil {
			errorResp(c, err)
			return
		}
		defer resp.Body.Close()

		dec := json.NewDecoder(resp.Body)

		err = dec.Decode(addressInfo)
		if err != nil {
			errorResp(c, err)
			return
		}

		err = app.loadBlocks(addressInfo)
		if err != nil {
			errorResp(c, err)
			return
		}

		go func() {
			bts, err := json.Marshal(addressInfo)
			if err != nil {
				log.Println(err)
			}
			app.RConn.Do("SET", "address:"+address, bts)
			app.RConn.Do("EXPIRE", "address:"+address, app.Config.Redis.ExpireSec)
		}()

		addressInfo.Source = "bitcoin.info"
	}

	c.JSON(200, addressInfo)
}

func (app *app) loadBlocks(address *models.Address) error {
	log.Println("Load Blocks")
	blockMap := make(map[int]*models.Blocks)

	for i := range address.Txs {
		blockHeight := address.Txs[i].BlockHeight
		blocks := new(models.Blocks)

		if _, exists := blockMap[blockHeight]; !exists {
			app.MgoCli.FindOne(mongo.Blocks, bson.M{"blocks.height": blockHeight}, blocks)

			if len(blocks.Blocks) == 0 {

				var err error
				blocks, err = getBlockFromApi(blockHeight)
				if err != nil {
					return err
				}

				err = app.MgoCli.Exec(mongo.Blocks, func(c *mgo.Collection) error {
					return c.Insert(blocks)
				})
				if err != nil {
					log.Println(err)
				}

			} else {
				blocks.Source = "cahce"
			}

			blockMap[blockHeight] = blocks

		}
		address.Txs[i].Blocks = blocks
	}
	return nil
}

func getBlockFromApi(blockHeight int) (*models.Blocks, error) {
	log.Println("Getting block from api: ", blockHeight)
	resp, err := http.Get("https://blockchain.info/ru/block-height/" + strconv.Itoa(blockHeight) + "?format=json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)

	blocks := new(models.Blocks)
	err = dec.Decode(blocks)
	if err != nil {
		return nil, err
	}

	return blocks, nil
}

func errorResp(c *gin.Context, err error) {
	log.Println(err)
	c.JSON(500, gin.H{
		"err": err.Error(),
	})
	panic(err)
}
