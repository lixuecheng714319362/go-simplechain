package main

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"github.com/Beyond-simplechain/foundation/asio"
	"log"
	"math/big"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/simplechain-org/go-simplechain/common"
	"github.com/simplechain-org/go-simplechain/core/types"
	"github.com/simplechain-org/go-simplechain/crypto"
	"github.com/simplechain-org/go-simplechain/ethclient"
)

const (
	warnPrefix = "\x1b[93mwarn:\x1b[0m"
	errPrefix  = "\x1b[91merror:\x1b[0m"
)

var txsCount = int64(0)

var sourceKey = "5aedb85503128685e4f92b0cc95e9e1185db99339f9b85125c1e2ddc0f7c4c48"

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

const SENDS = 1000000

func initNonce(seed uint64, count int) []uint64 {
	ret := make([]uint64, count)
	bigseed := seed * 1e10
	for i := 0; i < count; i++ {
		ret[i] = bigseed
		bigseed++
	}
	return ret
}

var parallel = asio.NewParallel(1000, 100)

var (
	chainId *uint64
	tps     *int
)

func main() {
	url := flag.String("url", "ws://127.0.0.1:8546", "websocket url")
	chainId = flag.Uint64("chainid", 1, "chainId")
	tps = flag.Int("tps", -1, "send tps limit, negative is limitless")

	sendTx := flag.Bool("sendtx", false, "enable only send tx")
	senderCount := flag.Int("accounts", 4, "the number of sender")
	callcode := flag.Bool("callcode", false, "enable call contract code")

	seed := flag.Uint64("seed", 1, "hash seed")

	flag.Parse()

	var cancels []context.CancelFunc

	if *callcode {

	}

	if *sendTx {
		log.Printf("start send tx: %d accounts", *senderCount)

		privateKey, err := crypto.HexToECDSA(sourceKey)
		if err != nil {
			log.Fatalf(errPrefix+" parse private key: %v", err)
		}
		publicKey := privateKey.Public()
		publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
		if !ok {
			log.Fatalf(errPrefix + " cannot assert type: publicKey is not of type *ecdsa.PublicKey")
		}
		fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

		nonces := initNonce(*seed, SENDS*(*senderCount))
		for i := 0; i < *senderCount; i++ {
			client, err := ethclient.Dial(*url)
			if err != nil {
				log.Fatalf(errPrefix+" connect %s: %v", *url, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancels = append(cancels, cancel)

			go throughputs(ctx, client, i, privateKey, fromAddress, nonces[i*SENDS:(i+1)*SENDS])
		}
	}

	go func() {
		http.ListenAndServe("127.0.0.1:6789", nil)
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(interrupt)
	<-interrupt
	parallel.Stop()
	for _, cancel := range cancels {
		cancel()
	}

	time.Sleep(time.Second)
	log.Printf("txsCount=%v", txsCount)
}

func getBlockLimit(ctx context.Context, client *ethclient.Client) uint64 {
	block, err := client.BlockByNumber(ctx, nil)
	if err != nil {
		return 60
	}
	return block.NumberU64() + 60
}

var big1 = big.NewInt(1)

func throughputs(ctx context.Context, client *ethclient.Client, index int, privateKey *ecdsa.PrivateKey, fromAddress common.Address, nonces []uint64) {
	gasLimit := uint64(21000 + (20+64)*68) // in units
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatalf(errPrefix+" get gas price: %v", err)
	}
	toAddress := common.HexToAddress("0xffd79941b7085805f48ded97298694c6bb950e2c")
	signer := types.NewEIP155Signer(new(big.Int).SetUint64(*chainId))

	var (
		data       [20 + 64]byte
		blockLimit = getBlockLimit(ctx, client)
		meterCount = 0
		i          int
	)

	copy(data[:], fromAddress.Bytes())

	start := time.Now()
	timer := time.NewTimer(0)
	<-timer.C
	timer.Reset(10 * time.Minute)

	tpsInterval := 10 * time.Minute
	if *tps > 0 {
		tpsInterval = time.Second
	}
	tpsTicker := time.NewTicker(tpsInterval)
	defer tpsTicker.Stop()

	for {
		if i >= len(nonces) {
			break
		}

		select {
		case <-ctx.Done():
			seconds := time.Since(start).Seconds()
			log.Printf("throughputs:%v return (total %v in %v s, %v txs/s)", index, meterCount, seconds, float64(meterCount)/seconds)
			atomic.AddInt64(&txsCount, int64(meterCount))
			return

		case <-tpsTicker.C:
			atomic.AddInt64(&txsCount, int64(meterCount))
			// statistics throughputs
			if *tps > 0 && meterCount > *tps {
				// sleep to cut down throughputs if higher than limit tps
				time.Sleep(time.Duration(meterCount / *tps) * time.Second)
			}

			meterCount = 0

		case <-time.After(10 * time.Second):
			blockLimit += 10

		default:
			nonce := nonces[i]

			copy(data[20:], new(big.Int).SetUint64(nonce).Bytes())
			//parallel.Put(func() error {
			//	sendTransaction(ctx, signer, privateKey, nonce, blockLimit, toAddress, big1, gasLimit, gasPrice, data[:], client)
			//	return nil
			//})
			sendTransaction(ctx, signer, privateKey, nonce, blockLimit, toAddress, big1, gasLimit, gasPrice, data[:], client)

			i++
			//switch {
			if i%10000 == 0 {
				blockLimit = getBlockLimit(ctx, client)
			}
			meterCount++
		}
	}
}

func sendTransaction(ctx context.Context, signer types.Signer, key *ecdsa.PrivateKey, nonce, limit uint64,
	toAddress common.Address, value *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, client *ethclient.Client) {

	tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)
	tx.SetBlockLimit(limit)

	signature, err := crypto.Sign(signer.Hash(tx).Bytes(), key)
	if err != nil {
		log.Printf(warnPrefix+" send tx[hash:%s, nonce:%d]: %v", tx.Hash().String(), tx.Nonce(), err)
		return
	}
	signed, err := tx.WithSignature(signer, signature)
	if err != nil {
		log.Printf(warnPrefix+" send tx[hash:%s, nonce:%d]: %v", tx.Hash().String(), tx.Nonce(), err)
		return
	}
	err = client.SendTransaction(ctx, signed)
	if err != nil {
		log.Printf(warnPrefix+" send tx[hash:%s, nonce:%d]: %v", tx.Hash().String(), tx.Nonce(), err)
		return
	}
}
