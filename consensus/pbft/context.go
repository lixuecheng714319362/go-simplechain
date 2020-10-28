package pbft

const defaultMaxBlockTxs = 10000

var MaxBlockTxs uint64 = defaultMaxBlockTxs

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
