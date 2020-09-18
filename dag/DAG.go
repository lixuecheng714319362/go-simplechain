package dag

import (
	"sync"

	"github.com/simplechain-org/go-simplechain/core/types"
	"github.com/simplechain-org/go-simplechain/log"
)

type DAG struct {
	mu     sync.Mutex
	Vertex Vertex
}

type Vertex struct {
	OutEdge  []uint32
	InDegree int
	ID       uint32
	Tx       *types.Transaction
}

var TopLevel = GetQueue()
var V_txs = make([]Vertex, 0)
var total_vtxs = 0

func (d *DAG) init(txSize int) {
	for i := 0; i < txSize; i++ {
		v := Vertex{
			OutEdge:  []uint32{},
			InDegree: 0,
			ID:       uint32(i),
		}
		V_txs = append(V_txs, v)
	}
	total_vtxs = txSize
}

func Clear() {
	V_txs = make([]Vertex, 0)
	total_vtxs = 0
	CriticalFileds = make(map[string]int, 0)
}

func generateDAG(id int) DAG {
	v := Vertex{
		OutEdge:  []uint32{},
		InDegree: 0,
		ID:       uint32(id),
	}
	return DAG{
		mu:     sync.Mutex{},
		Vertex: v,
	}
}

func (d *DAG) addEdge(fId, tId uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()

	V_txs[fId].OutEdge = append(V_txs[fId].OutEdge, tId)
	V_txs[tId].InDegree++
}

func (d *DAG) generate() {
	for _, v := range V_txs {
		if v.InDegree == 0 {
			TopLevel.Push(v.ID)
		}
	}
	//for test
	//fmt.Printf("TopLevel length is %d\n", TopLevel.Len())
	//for {
	//	if TopLevel.Len() == 0 {
	//		break
	//	}
	//	data, err := TopLevel.Pop()
	//	if err != nil {
	//		log.Error("error", err)
	//	}
	//	fmt.Printf("TopLevel %+v\n", data)
	//}
}

func WaitPop() ([]int, error) {
	var ret []int
	var topLevel_bak = GetQueue()
	for TopLevel.Len() != 0 {
		data, err := TopLevel.Pop()
		if err != nil {
			log.Error("get top id failed", "err", err)
			return ret, err
		}
		id := data.(uint32)
		ret = append(ret, int(id))
		//inDegree - 1 when data popped
		for _, v := range V_txs[id].OutEdge {
			V_txs[v].InDegree = V_txs[v].InDegree - 1
			if V_txs[v].InDegree == 0 {
				topLevel_bak.Push(v)
			}
		}
	}
	TopLevel = topLevel_bak
	return ret, nil
}
