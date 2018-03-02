package models

type Address struct {
	Hash160       string        `json:"hash160"`
	Address       string        `json:"address"`
	NTx           int           `json:"n_tx"`
	TotalReceived int64         `json:"total_received"`
	TotalSent     int64         `json:"total_sent"`
	FinalBalance  int           `json:"final_balance"`
	Txs           []Transaction `json:"txs"`
	Source        string        `json:"source,omitempty"`
	TxsCount      int           `json:"txs_count"`
}

type Transaction struct {
	Ver    int `json:"ver"`
	Inputs []struct {
		Sequence int64  `json:"sequence"`
		Witness  string `json:"witness"`
		PrevOut  struct {
			Spent   bool   `json:"spent"`
			TxIndex int    `json:"tx_index"`
			Type    int    `json:"type"`
			Addr    string `json:"addr"`
			Value   int    `json:"value"`
			N       int    `json:"n"`
			Script  string `json:"script"`
		} `json:"prev_out"`
		Script string `json:"script"`
	} `json:"inputs"`
	Weight      int    `json:"weight"`
	BlockHeight int    `json:"block_height"`
	RelayedBy   string `json:"relayed_by"`
	Out         []struct {
		Spent   bool   `json:"spent"`
		TxIndex int    `json:"tx_index"`
		Type    int    `json:"type"`
		Addr    string `json:"addr"`
		Value   int    `json:"value"`
		N       int    `json:"n"`
		Script  string `json:"script"`
	} `json:"out"`
	LockTime int         `json:"lock_time"`
	Result   int         `json:"result"`
	Size     int         `json:"size"`
	Time     int         `json:"time"`
	TxIndex  int         `json:"tx_index"`
	VinSz    int         `json:"vin_sz"`
	Hash     string      `json:"hash"`
	VoutSz   int         `json:"vout_sz"`
	Blocks   []RespBlock `json:"blocks"`
}
