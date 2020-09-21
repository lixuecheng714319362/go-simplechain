package pbft

const defaultMaxBlockTxs = 1000

var MaxBlockTxs uint64 = 1000

type SealContext struct {
	MaxBlockTxs    uint64
	LastTimeoutTx  int
	MaxNoTimeoutTx int
	LastSealTime   int64
}

func CreateSealContext() *SealContext {
	return &SealContext{
		MaxBlockTxs: defaultMaxBlockTxs,
	}
}
