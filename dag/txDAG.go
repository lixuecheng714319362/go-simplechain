package dag
//
//import (
//	"container/list"
//	"fmt"
//	"math/big"
//	"sync"
//
//	"github.com/simplechain-org/go-simplechain/core/types"
//	"github.com/simplechain-org/go-simplechain/log"
//)
//
//var CriticalFileds = make(map[string]int, 0) //key:txId; value:txs index
//var dag DAG
//
//type Graph struct {
//	mu     sync.Mutex
//	list   list.List
//	Singer types.Signer
//}
//
//func NewGraph(chainID *big.Int) *Graph {
//	return &Graph{
//		mu:     sync.Mutex{},
//		list:   list.List{},
//		Singer: types.NewEIP155Signer(chainID),
//	}
//}
//
//func (g *Graph) InitGraphWithTxPriceAndNonce(txs types.Transactions, txSize int) error {
//	//dag.init(txSize)
//	for i, tx := range txs {
//		if tx == nil {
//			break
//		}
//		v := Vertex{
//			OutEdge:  []uint32{},
//			InDegree: 0,
//			ID:       uint32(i),
//			Tx:       tx,
//		}
//		V_txs = append(V_txs, v)
//		//get dependencies
//		from, to, err := g.getDependencies(tx)
//		if err != nil {
//			return err
//		}
//		g.critical(from, to, i, tx.Hash().String())
//	}
//	total_vtxs = len(V_txs)
//	dag.generate()
//	return nil
//}
//
//func (g *Graph) InitGraph(txs []*types.Transaction) error {
//	g.Singer = types.NewEIP155Signer(big.NewInt(110)) //todo:chainId setting
//	txSize := len(txs)
//	dag.init(txSize)
//	for i := 0; i < txSize; i++ {
//		tx := txs[i]
//		//get dependencies
//		from, to, err := g.getDependencies(tx)
//		if err != nil {
//			return err
//		}
//		g.critical(from, to, i, tx.Hash().String())
//	}
//	dag.generate()
//	return nil
//}
//
//func (g *Graph) critical(from, to string, index int, txHash string) {
//	if v, ok := CriticalFileds[from]; ok {
//		dag.addEdge(uint32(v), uint32(index))
//		CriticalFileds[from] = index
//	}
//	if v, ok := CriticalFileds[to]; ok {
//		dag.addEdge(uint32(v), uint32(index))
//		CriticalFileds[to] = index
//	} else {
//		CriticalFileds[from] = index
//		CriticalFileds[to] = index
//	}
//}
//
//func (g *Graph) getDependencies(tx *types.Transaction) (from, to string, err error) {
//	fromAddr, _ := types.Sender(g.Singer, tx)
//	if err != nil {
//		log.Error("get from address failed", "error", err)
//		return
//	}
//	return fromAddr.String(), tx.To().String(), err
//}
//
//func (g *Graph) AddDag(dag interface{}) {
//	g.mu.Lock()
//	defer g.mu.Unlock()
//	g.list.PushBack(dag)
//}
//
//func Exec(txs []*types.Transaction) error {
//	fmt.Printf("start exec txs, toplevel length is:%v\n", TopLevel.Len())
//	for {
//		if TopLevel.Len() == 0 {
//			break
//		}
//		execIds, err := WaitPop()
//		if err != nil {
//			return err
//		}
//		fmt.Printf("length of execIds is :%v\n", len(execIds))
//		if len(execIds) == 0 {
//			log.Info("transaction execute finished!")
//			return nil
//		}
//		var wg sync.WaitGroup
//		wg.Add(len(execIds))
//		for _, id := range execIds {
//			go execTx(txs[id], &wg)
//		}
//		wg.Wait()
//	}
//	return nil
//}
//
//func execTx(tx *types.Transaction, wg *sync.WaitGroup) {
//	//fromAddr := types.GetAddrNotUseSign(tx)
//	fmt.Printf("tx finished, txHash: %v, to: %v\n", tx.Hash().String(),  tx.To().String())
//	wg.Done()
//}
