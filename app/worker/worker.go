package worker

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/norand94/bitcoin-address-info/app/models"
)

type Worker struct {
	RequestCh      chan Request
	loaderRoutines int
}

type Request struct {
	Height int
	RespCh chan HeightDone
}

type HeightDone struct {
	Blocks *models.Blocks
	Error  error
}

func New(loaderRoutines int) *Worker {
	w := &Worker{
		RequestCh:      make(chan Request, 100),
		loaderRoutines: loaderRoutines,
	}
	go w.run()
	return w
}

func (w *Worker) run() {
	for i := 0; i < w.loaderRoutines; i++ {
		go func(ch <-chan Request) {
			for {
				req := <-ch

				log.Println("Getting block from api: ", req.Height)
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

				log.Println("Done! block:", req.Height)
				req.RespCh <- HeightDone{Blocks: blocks}
			}

		}(w.RequestCh)
	}

}
