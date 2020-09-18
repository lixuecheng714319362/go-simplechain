//+build sub

package types

import (
	"bytes"
	"math/big"
	"runtime"
	"testing"
	"time"

	"github.com/exascience/pargo/parallel"
	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/rlp"
)

var sTx, _ = NewTransaction(
	3,
	common.HexToAddress("b94f5374fce5edbc8e2a8697c15331677e6ebf0b"),
	big.NewInt(10),
	2000,
	big.NewInt(1),
	common.FromHex("5544"),
).WithSignature(
	HomesteadSigner{},
	common.Hex2Bytes("98ff921201554726367d2be8c804a7ff89ccf285ebc57dff8ae4c44b9c19ac4a8887321be575c8095f789dd4c743dfe42c1820f9231f98a962b210e3ac2452a301"),
)

func TestTransactionEncode(t *testing.T) {
	sTx.SetSynced(true)
	sTx.SetImportTime(time.Now().UnixNano())
	txb, err := rlp.EncodeToBytes(sTx)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	should := common.FromHex("f86203018207d094b94f5374fce5edbc8e2a8697c15331677e6ebf0b0a825544801ca098ff921201554726367d2be8c804a7ff89ccf285ebc57dff8ae4c44b9c19ac4aa08887321be575c8095f789dd4c743dfe42c1820f9231f98a962b210e3ac2452a3")
	if !bytes.Equal(txb, should) {
		t.Errorf("encoded RLP mismatch, got %x", txb)
	}
}

func TestTransaction_BlockLimitAndTimestamp(t *testing.T) {
	tx := NewTransaction(1, common.Address{1}, common.Big0, 1, common.Big2, []byte("abcdef"))

	// set blockLimit
	tx1 := NewTransaction(1, common.Address{1}, common.Big0, 1, common.Big2, []byte("abcdef"))
	tx1.SetBlockLimit(100)

	if tx.Hash() == tx1.Hash() {
		t.Errorf("different blockLimit tx requeire diff hash, got same %v", tx.Hash().String())
	}

	// set timestamp
	tx2 := NewTransaction(1, common.Address{1}, common.Big0, 1, common.Big2, []byte("abcdef"))
	tx2.SetImportTime(time.Now().UnixNano())

	if tx.Hash() != tx2.Hash() {
		t.Errorf("different timestamp tx requeire same hash, want %v, got %v", tx.Hash().String(), tx2.Hash().String())
	}
}

func BenchmarkSender10000(b *testing.B) { benchmarkTxs(b, txSender, 10000) }
func BenchmarkSender5000(b *testing.B)  { benchmarkTxs(b, txSender, 5000) }
func BenchmarkSender1000(b *testing.B)  { benchmarkTxs(b, txSender, 1000) }

func BenchmarkAsyncSender10000(b *testing.B) { benchmarkTxs(b, txAsyncSender, 10000) }
func BenchmarkAsyncSender5000(b *testing.B)  { benchmarkTxs(b, txAsyncSender, 5000) }
func BenchmarkAsyncSender1000(b *testing.B)  { benchmarkTxs(b, txAsyncSender, 1000) }

func BenchmarkHash10000(b *testing.B) { benchmarkTxs(b, txHash, 10000) }
func BenchmarkHash5000(b *testing.B)  { benchmarkTxs(b, txHash, 5000) }
func BenchmarkHash1000(b *testing.B)  { benchmarkTxs(b, txHash, 1000) }

func BenchmarkAsyncHash10000(b *testing.B) { benchmarkTxs(b, txAsyncHash, 10000) }
func BenchmarkAsyncHash5000(b *testing.B)  { benchmarkTxs(b, txAsyncHash, 5000) }
func BenchmarkAsyncHash1000(b *testing.B)  { benchmarkTxs(b, txAsyncHash, 1000) }

func BenchmarkEncode10000(b *testing.B) { benchmarkTxs(b, txEncode, 10000) }
func BenchmarkEncode5000(b *testing.B)  { benchmarkTxs(b, txEncode, 5000) }
func BenchmarkEncode1000(b *testing.B)  { benchmarkTxs(b, txEncode, 1000) }

func BenchmarkAsyncEncode10000(b *testing.B) { benchmarkTxs(b, txAsyncEncode, 10000) }
func BenchmarkAsyncEncode5000(b *testing.B)  { benchmarkTxs(b, txAsyncEncode, 5000) }
func BenchmarkAsyncEncode1000(b *testing.B)  { benchmarkTxs(b, txAsyncEncode, 1000) }

func benchmarkTxs(b *testing.B, f func(transactions Transactions), size int) {
	txs := getTransactions(size)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		b.StartTimer()
		f(txs)
	}
}

func txSender(txs Transactions) {
	for _, tx := range txs {
		signer.Sender(tx)
	}
}

func txAsyncSender(txs Transactions) {
	parallel.Range(0, txs.Len(), runtime.NumCPU(), func(low, high int) {
		for i := low; i < high; i++ {
			signer.Sender(txs[i])
		}
	})
}

func txHash(txs Transactions) {
	for i := range txs {
		rlpHash(txs[i])
	}
}

func txAsyncHash(txs Transactions) {
	parallel.Range(0, txs.Len(), runtime.NumCPU(), func(low, high int) {
		for i := low; i < high; i++ {
			rlpHash(txs[i])
		}
	})
}

func txEncode(txs Transactions) {
	for i := range txs {
		rlp.EncodeToBytes(txs[i])
	}
}

func txAsyncEncode(txs Transactions) {
	parallel.Range(0, txs.Len(), runtime.NumCPU(), func(low, high int) {
		for i := low; i < high; i++ {
			rlp.EncodeToBytes(txs[i])
		}
	})
}
