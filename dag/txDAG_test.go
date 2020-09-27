package dag
//
//import (
//	"github.com/simplechain-org/go-simplechain/common"
//	"github.com/simplechain-org/go-simplechain/core/types"
//	"math/big"
//	"math/rand"
//	"sync"
//	"testing"
//)
//
//var SourceKey = []string{
//	"88eb79cc8cc05c8040936168f03fc46def08ba40165224790c072b4ddca42636", //accounts.0
//	"61c91c58c058656546113e54b21a01abaee2a83b3deb0b4b2635187f885021a4",
//	"31a5ae562b483964a20578aa5e24c6c89168541ecdb718fcc4030bb9e18744b8",
//	"6380bf86a3288f5f5fea180e1f1c7d83cb14190a00ccaab8c0c941cd3387c7d5",
//	"8e8ea6d72fe39a74911cebc4b862f893934573cf934f8a35b3c1815298c8f655",
//	"680d854a24fdf7de7d0a35bd3c81affcd1125e620edd6ca12c892e8c2f4593d8",
//	"ce0e1aaa7fa21af0dea2814802b797ed5838317ed847bdc59e810108e8501b10",
//	"d341b41a6c1c8224f87645660b1d6523db5c5b2696d3b4697be9755dffab91c6",
//	"7846643ede267141509d73cda2cc81254479cf7a78509c624fb9486a7d9f8e82",
//	"5f3ad6c34b8d128bdbad0d3e9fa9064717b2776893c21769f1145c88a64ac239", //10
//	"ad701a72f912dd64c5d8a9168ee0d4776095fcda4564527351347f3cab615740",
//	"3de8ec2615f5f2676c2d447ecb715b93744e5698a44cb365771195f346b58eaf",
//	"319da3aad9ba6c02a01d342221766f56b25e6c220e9de4b92a842e31557e8bac",
//	"309f4824ba85b8aed7c91ab8c09ae15ce0b6833f1825d98f64e034d769150ee8",
//	"4bf5fb969e69b82e19eb9fe7d3630afc87430df9f385d8ec78feebf9da126cc1",
//	"e2e42dd6ec2902d43f792d81ab11e833e8bc9efbc15fbaccec692dc93f9b438d",
//	"3b048d1e78a08283922563701dfc256f00f5db886ec28fa47a4c5a2ee622ae6f",
//	"05f25b345193a210819f5cf03b1a3e247cb20483c2e0ad531608eca75b765bf2",
//	"1b3bac2c7eafdd9ea36b96d21bbb67364abd86ea861fe5f1e3ca8a8b030fd7e1",
//	"a2ffa6b712fe55c3c7a3e8f0f2639c0600a1a7f1c7ca486cd15ddce46343867f", //20
//	"8b235abe9f0ef912d8f4dbf5ce317250dd03aad7bf354225919282a98c3c5b5a",
//	"776290c3a1e44524d31dd4751073a03b70e64247a8b59db0e1debe73a4c51ffc",
//	"feadb665ace9f3281e5668957687b5ee451834adcd5704ed4d692f25f8a24b73",
//	"fd7ad0817cfb54bc0317c604c0de477607727aa85680f412b69b6154e9d2d293",
//	"6f0f5eb5671357c0d6a577489e4bb4312a77c0bde8d4ed54b51da24b3334db64",
//	"77d10252d44e19d36846951f2b6b75b0b44c28dbf587d88a1eaa7ef6cd908573",
//	"767ba7ea10acbc9d98c1afad661d651a5cc4715ec9557f304026d990fd968523",
//	"f22a4daf841db5f3bb496a0a9afa2339e5ef155f254965f2429776242744a554",
//	"a913d5e03467c0096570f1300131dd21c868375cc6e48f547d58cf1540fe3ff8",
//	"b8db2da7157a6f4c34523bc41320a78af59959498ed207602a34f9dff3e52299", //30
//	"602304633a4b7b6f77a8dce663dbc3269e70c485a1bc5850daafd3847d571485",
//	"a39622346d1c6ac9670dd18a82c624f1ad96dd54fb036bd00a5efa104292dc6e",
//	"97a055ab235fef73e5215dcd450031b64f9c73c506c3165f07725ec910c015c3",
//	"d27f2f775499e505e1f23581ad6ed36d4afbd651c1141d0275e327b59a2b2b89",
//	"8aed374bd877a3da1b8811b8365c09ee4a48b99eeacfc5d4272f72f9648193c6",
//	"b73b07ad4fd604716d6ebd719e382908cbb34901631dbaffce991a1b760dd246",
//	"9d8a547b41b5286818ca1efc72bd3e632d96cf7d0978db28e685d16ccb217542",
//	"0a3e97f35ee78998217ca5824f43d34c86f44bac5033b9a797a99fa04143b6c6",
//	"20ee87bae03a74a47aad20cc53f6410f3b37c77c2027a857944dae5d1e5c1d38",
//	"6b61e819e8715b7ebb248c6a4e0fb883b983d602c84a75f03ffb7f34102d0210", //40
//}
//
//var mu sync.Mutex
//var txs []*types.Transaction
//var gasLimit = uint64(21000 + (20+64)*68) // in units
//
//func TestGraph_InitGraph(t *testing.T) {
//	var wg = sync.WaitGroup{}
//	var addrCount = 20
//	var txCount = 500
//	wg.Add(addrCount)
//	for i := 0; i < addrCount; i++ {
//		go getTxs(txCount, &wg)
//	}
//	wg.Wait()
//	t.Log("length of txs", len(txs))
//	var graph Graph
//	graph.InitGraph(txs)
//	Exec(txs)
//}
//
//func TestEasyDAG(t *testing.T) {
//
//	for i := 0; i < 2; i++ {
//		txs = make([]*types.Transaction, 0)
//		makeTxs()
//		t.Log("length of txs", len(txs))
//		var graph Graph
//		graph.InitGraph(txs)
//		Exec(txs)
//		Clear()
//	}
//}
//
//func makeTxs() {
//	var data [20 + 64]byte
//	fromAddr := common.HexToAddress(SourceKey[0])
//	toAddr := common.HexToAddress(SourceKey[1])
//	copy(data[:], fromAddr.Bytes())
//	tx1 := types.NewTransaction(0, toAddr, big.NewInt(10), gasLimit, big.NewInt(10), data[:])
//	txs = append(txs, tx1)
//
//	fromAddr = common.HexToAddress(SourceKey[2])
//	toAddr = common.HexToAddress(SourceKey[3])
//	copy(data[:], fromAddr.Bytes())
//	tx2 := types.NewTransaction(0, toAddr, big.NewInt(10), gasLimit, big.NewInt(10), data[:])
//	txs = append(txs, tx2)
//
//	fromAddr = common.HexToAddress(SourceKey[4])
//	toAddr = common.HexToAddress(SourceKey[5])
//	copy(data[:], fromAddr.Bytes())
//	tx3 := types.NewTransaction(0, toAddr, big.NewInt(10), gasLimit, big.NewInt(10), data[:])
//	txs = append(txs, tx3)
//
//	fromAddr = common.HexToAddress(SourceKey[3])
//	toAddr = common.HexToAddress(SourceKey[4])
//	copy(data[:], fromAddr.Bytes())
//	tx4 := types.NewTransaction(0, toAddr, big.NewInt(10), gasLimit, big.NewInt(10), data[:])
//	txs = append(txs, tx4)
//
//	fromAddr = common.HexToAddress(SourceKey[0])
//	toAddr = common.HexToAddress(SourceKey[5])
//	copy(data[:], fromAddr.Bytes())
//	tx5 := types.NewTransaction(0, toAddr, big.NewInt(10), gasLimit, big.NewInt(10), data[:])
//	txs = append(txs, tx5)
//
//	fromAddr = common.HexToAddress(SourceKey[4])
//	toAddr = common.HexToAddress(SourceKey[0])
//	copy(data[:], fromAddr.Bytes())
//	tx6 := types.NewTransaction(0, toAddr, big.NewInt(10), gasLimit, big.NewInt(10), data[:])
//	txs = append(txs, tx6)
//}
//
//func getTxs(txCount int, wg *sync.WaitGroup) {
//	var data [20 + 64]byte
//	for i := 0; i < txCount; i++ {
//		mu.Lock()
//		//random get from and to
//		start := rand.Intn(len(SourceKey))
//		end := rand.Intn(len(SourceKey))
//		if start == end {
//			end++
//		}
//		if end >= len(SourceKey) {
//			end -= 2
//		}
//		fromAddr := common.HexToAddress(SourceKey[start])
//		toAddr := common.HexToAddress(SourceKey[end])
//		copy(data[:], fromAddr.Bytes())
//		tmp := int64(rand.Intn(30))
//		tx := types.NewTransaction(uint64(i), toAddr, big.NewInt(tmp), gasLimit, big.NewInt(10), data[:])
//		txs = append(txs, tx)
//		mu.Unlock()
//	}
//	wg.Done()
//}
