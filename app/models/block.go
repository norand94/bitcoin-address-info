package models

type Blocks struct {
	Source string  `json:"source"`
	Blocks []Block `json:"blocks"`
}

type Block struct {
	Hash         string        `json:"hash"`
	Ver          int           `json:"ver"`
	PrevBlock    string        `json:"prev_block"`
	MrklRoot     string        `json:"mrkl_root"`
	Time         int           `json:"time"`
	Bits         int           `json:"bits"`
	Fee          int           `json:"fee"`
	Nonce        int           `json:"nonce"`
	NTx          int           `json:"n_tx"`
	Size         int           `json:"size"`
	BlockIndex   int           `json:"block_index"`
	MainChain    bool          `json:"main_chain"`
	Height       int           `json:"height"`
	ReceivedTime int           `json:"received_time"`
	RelayedBy    string        `json:"relayed_by"`
	Source       string        `json:"source,omitempty"`
	Tx           []Transaction `json:"tx"`
}
