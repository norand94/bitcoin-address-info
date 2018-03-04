package app

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/garyburd/redigo/redis"

	"github.com/gin-gonic/gin"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/norand94/bitcoin-address-info/app/config"
	"github.com/norand94/bitcoin-address-info/app/models"
	"github.com/norand94/bitcoin-address-info/app/mongo"
	"github.com/norand94/bitcoin-address-info/app/worker"
	"github.com/sirupsen/logrus"
)

type app struct {
	Config  *config.M
	MgoCli  *mongo.Client
	RConn   redis.Conn
	Worker  *worker.Worker
	AddrMap *addrMap
	InfoLog *logrus.Logger
	ErrLog  *logrus.Logger
}

const ginLog = "bitcoin.gin.log"
const infoLog = "bitcoin.info.log"
const errLog = "bitcoin.err.log"

func Run(conf *config.M) {
	app := new(app)
	app.Config = conf

	appF := initLogFile(infoLog)
	defer appF.Close()
	app.InfoLog = logrus.New()
	app.InfoLog.Out = io.MultiWriter(appF, os.Stdout)
	app.InfoLog.Formatter = &logrus.TextFormatter{}

	errF := initLogFile(errLog)
	defer errF.Close()
	app.ErrLog = logrus.New()
	app.ErrLog.Out = io.MultiWriter(errF, appF, os.Stderr)
	app.InfoLog.Formatter = &logrus.TextFormatter{}

	app.InfoLog.Infoln("Application initializing")

	var err error
	app.RConn, err = redis.Dial("tcp", conf.Redis.Address)
	if err != nil {
		app.ErrLog.Errorln("Не удалось подключиться к Redis: ", err.Error())
	}

	resp, err := app.RConn.Do("AUTH", conf.Redis.Password)
	if err != nil {
		app.ErrLog.Errorln("Не удалось подключиться к Redis: ", err.Error())
	}
	app.InfoLog.WithField("RedisResponse", resp)

	app.MgoCli = mongo.New(conf)

	app.InfoLog.WithField("LoaderRoutinesCount", conf.LoaderRoutines)
	app.Worker = worker.New(conf.LoaderRoutines, app.InfoLog)

	app.AddrMap = newAddrMap()

	ginF := initLogFile(ginLog)
	defer ginF.Close()
	gin.DefaultWriter = ginF

	r := gin.Default()

	r.GET("/address/:address", app.addressHandler)
	app.InfoLog.Infoln("Application started")
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
		app.ErrLog.Errorln(err)
	}

	if addrBts != nil {
		app.InfoLog.Info("Loaded address", address, "from redis cache")
		c.Writer.Header().Add("Content-Type", "application/json; charset=utf-8")
		c.Writer.Write(addrBts.([]byte))
		return
	}

	addrTime, exists := app.AddrMap.GetOrSet(address, time.Now())
	if exists {
		c.JSON(200, gin.H{
			"message":      "this address already processed",
			"startProcess": addrTime,
		})
		return
	}
	defer app.AddrMap.Del(address)

	addressInfo := new(models.Address)

	err = app.MgoCli.FindOne(mongo.Address, bson.M{"address": address}, addressInfo)
	if err != nil && mgo.ErrNotFound.Error() != err.Error() {
		app.errorResp(c, err)
		return
	}

	if err != nil && mgo.ErrNotFound.Error() == err.Error() {
		//Если адрес не найден в базе
		addressInfo, err = GetAddrFromApi(address, 0)
		if err != nil {
			app.errorResp(c, err)
			return
		}
		addressInfo.TxsCount = len(addressInfo.Txs)

		err = app.loadBlocks(addressInfo)
		if err != nil {
			app.errorResp(c, err)
			return
		}

		go func(addrInfo *models.Address) {
			err := app.saveAddrInfo(addrInfo)
			if err != nil {
				app.ErrLog.Errorln(err)
			}
		}(addressInfo)

	} else {
		//В базе уже существует, необходимо обновить
		inpAddr, err := GetAddrFromApi(address, addressInfo.TxsCount)
		if err != nil {
			app.errorResp(c, err)
			return
		}

		app.InfoLog.Infoln("New transactions ", len(inpAddr.Txs), " for: ", address)
		addressInfo.TxsCount += len(inpAddr.Txs)
		addressInfo.Txs = append(addressInfo.Txs, inpAddr.Txs...)

		err = app.loadBlocks(inpAddr)
		if err != nil {
			app.errorResp(c, err)
			return
		}
		go func(addrInfo *models.Address) {
			err := app.saveAddrInfo(addrInfo)
			if err != nil {
				app.ErrLog.Errorln(err)
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
				app.InfoLog.Infoln("loading block ", blockHeight, " from mongo")
				blocks.Source = "cahce"
				blockMap[blockHeight] = blocks
			}

		}
	}

	for _, req := range requests {
		resp := <-req.RespCh
		if resp.Error != nil {
			return resp.Error
		}

		blockMap[req.Height] = resp.Blocks
		go func(blocks *models.Blocks) {
			err := app.MgoCli.Exec(mongo.Blocks, func(c *mgo.Collection) error {
				return c.Insert(blocks)
			})
			if err != nil {
				app.ErrLog.Errorln(err)
			}
		}(resp.Blocks)

	}

	for i := range address.Txs {
		address.Txs[i].Blocks = blockMap[address.Txs[i].BlockHeight].RespBlocks()
	}

	return nil
}

func (app *app) errorResp(c *gin.Context, err error) {
	app.ErrLog.Errorln(err)
	c.JSON(500, gin.H{
		"err": err.Error(),
	})
}
