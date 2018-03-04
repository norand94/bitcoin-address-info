package worker

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/norand94/bitcoin-address-info/app/models"
	"github.com/sirupsen/logrus"
)

type Worker struct {
	RequestCh      chan Request
	loaderRoutines int
	InfoLog        *logrus.Logger
}

type Request struct {
	Height int
	RespCh chan HeightDone
}

type HeightDone struct {
	Blocks *models.Blocks
	Error  error
}

func New(loaderRoutines int, logger *logrus.Logger) *Worker {
	w := &Worker{
		RequestCh:      make(chan Request, 100),
		loaderRoutines: loaderRoutines,
		InfoLog:        logger,
	}
	go w.run()
	return w
}

func (w *Worker) run() {
	for i := 0; i < w.loaderRoutines; i++ {
		go func(ch <-chan Request, l *logrus.Logger) {
			for {
				req := <-ch

				l.Infoln("Getting block from api: ", req.Height)
				resp, err := http.Get("https://blockchain.info/ru/block-height/" + strconv.Itoa(req.Height) + "?format=json")
				if err != nil {
					req.RespCh <- HeightDone{Error: err}
				}
				defer resp.Body.Close()

				dec := json.NewDecoder(resp.Body)

				blocks := new(models.Blocks)
				err = dec.Decode(blocks)
				if err != nil {
					req.RespCh <- HeightDone{Error: err}
				}

				l.Infoln("Done! block:", req.Height)
				req.RespCh <- HeightDone{Blocks: blocks}
				close(req.RespCh)
			}

		}(w.RequestCh, w.InfoLog)
	}

}
