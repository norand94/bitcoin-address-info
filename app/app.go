package app

import (
	"encoding/json"
	"fmt"
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
	"github.com/norand94/bitcoin-address-info/app/worker"
)

type app struct {
	Config *config.M
	MgoCli *mongo.Client
	RConn  redis.Conn
	Worker *worker.Worker
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

	fmt.Println("Loader routines: ", conf.LoaderRoutines)
	app.Worker = worker.New(conf.LoaderRoutines)

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
	requests := make([]worker.Request, 0, len(address.Txs))

	for i := range address.Txs {
		blockHeight := address.Txs[i].BlockHeight
		blocks := new(models.Blocks)

		if _, exists := blockMap[blockHeight]; !exists {
			app.MgoCli.FindOne(mongo.Blocks, bson.M{"blocks.height": blockHeight}, blocks)

			if len(blocks.Blocks) == 0 {

				req := worker.Request{
					Height: blockHeight,
					RespCh: make(chan worker.HeightDone, 1),
				}
				app.Worker.RequestCh <- req
				requests = append(requests, req)

			} else {
				blocks.Source = "cahce"
				blockMap[blockHeight] = blocks
			}

		}
	}

	for _, req := range requests {
		resp := <-req.RespCh
		if resp.Error != nil {
			log.Println(resp.Error)
			return resp.Error
		}

		blockMap[req.Height] = resp.Blocks
		go func(blocks *models.Blocks) {
			err := app.MgoCli.Exec(mongo.Blocks, func(c *mgo.Collection) error {
				return c.Insert(blocks)
			})
			if err != nil {
				log.Println(err)
			}
		}(resp.Blocks)

	}

	for i := range address.Txs {
		address.Txs[i].Blocks = blockMap[address.Txs[i].BlockHeight].RespBlocks()
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
