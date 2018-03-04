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
		log.Fatalln("Не удалось подключиться к Redis: ", err.Error())
	}

	resp, err := app.RConn.Do("AUTH", conf.Redis.Password)
	if err != nil {
		log.Fatalln("Не удалось подключиться к Redis: ", err.Error())
	}
	log.Println("REDIS: ", resp)

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

	err = app.MgoCli.FindOne(mongo.Address, bson.M{"address": address}, addressInfo)
	if err != nil && mgo.ErrNotFound.Error() != err.Error() {
		errorResp(c, err)
		return
	}

	if err != nil && mgo.ErrNotFound.Error() == err.Error() {
		//Если адрес не найден в базе
		addressInfo, err = GetAddrFromApi(address, 0)
		if err != nil {
			errorResp(c, err)
			return
		}
		addressInfo.TxsCount = len(addressInfo.Txs)

		err = app.loadBlocks(addressInfo)
		if err != nil {
			errorResp(c, err)
			return
		}

		go func(addrInfo *models.Address) {
			err := app.saveAddrInfo(addrInfo)
			if err != nil {
				log.Println("err: ", err.Error())
			}
		}(addressInfo)

	} else {
		//В базе уже существует, необходимо обновить
		inpAddr, err := GetAddrFromApi(address, addressInfo.TxsCount)
		if err != nil {
			errorResp(c, err)
			return
		}

		log.Println("New transactions ", len(inpAddr.Txs), " for: ", address)
		addressInfo.TxsCount += len(inpAddr.Txs)
		addressInfo.Txs = append(addressInfo.Txs, inpAddr.Txs...)

		err = app.loadBlocks(inpAddr)
		if err != nil {
			errorResp(c, err)
			return
		}
		go func(addrInfo *models.Address) {
			err := app.saveAddrInfo(addrInfo)
			if err != nil {
				log.Println("err: ", err.Error())
			}
		}(addressInfo)
	}

	addressInfo.Source = "bitcoin.info"

	c.JSON(200, addressInfo.RespAddress())
}

func (app *app) saveAddrInfo(addrInfo *models.Address) error {
	bts, err := json.Marshal(addrInfo.RespAddress())
	if err != nil {
		return err
	}

	_, err = app.RConn.Do("SET", "address:"+addrInfo.Address, bts)
	_, err = app.RConn.Do("EXPIRE", "address:"+addrInfo.Address, app.Config.Redis.ExpireSec)
	if err != nil {
		return err
	}

	err = app.MgoCli.Exec(mongo.Address, func(c *mgo.Collection) error {
		_, err := c.Upsert(bson.M{"address": addrInfo.Address}, addrInfo)
		return err
	})
	if err != nil {
		return err
	}
	return nil
}

func GetAddrFromApi(address string, offset int) (*models.Address, error) {
	resp, err := http.Get("https://blockchain.info/ru/rawaddr/" + address + "?confirmations=6&limit=100000&offset=" + strconv.Itoa(offset))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)

	inpAddr := new(models.Address)
	err = dec.Decode(inpAddr)
	if err != nil {
		return nil, err
	}
	return inpAddr, nil
}

func (app *app) loadBlocks(address *models.Address) error {
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

func errorResp(c *gin.Context, err error) {
	log.Println(err)
	c.JSON(500, gin.H{
		"err": err.Error(),
	})
}
