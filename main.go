package main

import (
	"errors"
	"log"
	"math/big"
	"runtime"
	"sync"
	"time"

	_ "go.uber.org/automaxprocs"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/go-arbi-bot/internal/adapter"
	"github.com/iqbalbaharum/go-arbi-bot/internal/coder"
	"github.com/iqbalbaharum/go-arbi-bot/internal/config"
	"github.com/iqbalbaharum/go-arbi-bot/internal/generators"
	instructions "github.com/iqbalbaharum/go-arbi-bot/internal/instructions"
	bot "github.com/iqbalbaharum/go-arbi-bot/internal/library"
	"github.com/iqbalbaharum/go-arbi-bot/internal/liquidity"
	"github.com/iqbalbaharum/go-arbi-bot/internal/rpc"
	"github.com/iqbalbaharum/go-arbi-bot/internal/storage"
	"github.com/iqbalbaharum/go-arbi-bot/internal/types"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

func loadAdapter() {
	adapter.GetRedisClient(0)
}

var (
	client           *pb.GeyserClient
	latestBlockhash  string
	wsolTokenAccount solana.PublicKey
)

func main() {

	numCPU := runtime.NumCPU() * 2
	maxProcs := runtime.GOMAXPROCS(0)
	log.Printf("Number of logical CPUs available: %d", numCPU)
	log.Printf("Number of CPUs being used: %d", maxProcs)

	runtime.GOMAXPROCS(runtime.NumCPU())

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Printf("Initialized .env")
	err := config.InitEnv()
	if err != nil {
		log.Print(err)
		return
	}

	log.Printf("Wallet: %s", config.Payer.PublicKey())
	log.Printf("SELL Method: %s", config.SELL_METHOD)

	ata, err := getOrCreateAssociatedTokenAccount()
	if err != nil {
		log.Print(err)
		return
	}

	log.Printf("WSOL Associated Token Account %s", ata)
	wsolTokenAccount = *ata

	generators.GrpcConnect(config.GrpcAddr, config.InsecureConnection)

	txChannel := make(chan generators.GeyserResponse)

	var wg sync.WaitGroup

	// Create a worker pool
	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for response := range txChannel {
				processResponse(response)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		runBatchTransactionThread()
	}()

	generators.GrpcSubscribeByAddresses(
		config.GrpcToken,
		[]string{config.RAYDIUM_AMM_V4.String()},
		[]string{}, txChannel)

	wg.Wait()

	defer func() {
		if err := generators.CloseConnection(); err != nil {
			log.Printf("Error closing gRPC connection: %v", err)
		}
	}()
}

func runBatchTransactionThread() {
	ticker := time.NewTicker(time.Duration(config.TxInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			runBatchTransactionProcess()
		}
	}
}

func runBatchTransactionProcess() {
	if len(latestBlockhash) <= 0 {
		return
	}

	trackedAMMs, err := bot.GetAllTrackedAmm()
	if err != nil {
		log.Printf("Error fetching tracked AMMs: %v", err)
		return
	}

	var transactions []*solana.Transaction

	for _, tracker := range *trackedAMMs {
		if tracker.Status == storage.TRACKED_BOTH {
			if tracker.LastUpdated < time.Now().Add(-10*time.Minute).Unix() {
				log.Printf("%s| Remove from tracking", tracker.AmmId)
				go bot.TrackedAmm(tracker.AmmId, true)
			} else {
				txs, err := generateInstructions(tracker.AmmId, "bloxroute")
				if err != nil {
					log.Print(err)
				}

				transactions = append(transactions, txs...)
			}
		}
	}

	if len(transactions) > 0 {
		if err := rpc.SendBatchTransactions(transactions); err != nil {
			log.Printf("Error sending batch transactions: %v", err)
		}
	}
}

func processResponse(response generators.GeyserResponse) {
	latestBlockhash = response.MempoolTxns.RecentBlockhash

	c := coder.NewRaydiumAmmInstructionCoder()
	for _, ins := range response.MempoolTxns.Instructions {
		programId := response.MempoolTxns.AccountKeys[ins.ProgramIdIndex]

		if programId == config.RAYDIUM_AMM_V4.String() {
			decodedIx, err := c.Decode(ins.Data)
			if err != nil {
				continue
			}

			switch decodedIx.(type) {
			case coder.Initialize2:
				log.Println("Initialize2", response.MempoolTxns.Signature)
				processInitialize2(ins, response)
			case coder.Withdraw:
				log.Println("Withdraw", response.MempoolTxns.Signature)
				processWithdraw(ins, response)
			case coder.SwapBaseIn:
				processSwapBaseIn(ins, response)
			case coder.SwapBaseOut:
			default:
				log.Println("Unknown instruction type")
			}
		}
	}
}

func getPublicKeyFromTx(pos int, tx generators.MempoolTxn, instruction generators.TxInstruction) (*solana.PublicKey, error) {
	accountIndexes := instruction.Accounts
	if len(accountIndexes) == 0 {
		return nil, errors.New("no account indexes provided")
	}

	lookupsForAccountKeyIndex := bot.GenerateTableLookup(tx.AddressTableLookups)
	var ammId *solana.PublicKey
	accountIndex := int(accountIndexes[pos])

	if accountIndex >= len(tx.AccountKeys) {
		lookupIndex := accountIndex - len(tx.AccountKeys)
		lookup := lookupsForAccountKeyIndex[lookupIndex]
		table, err := bot.GetLookupTable(solana.MustPublicKeyFromBase58(lookup.LookupTableKey))
		if err != nil {
			return nil, err
		}

		if int(lookup.LookupTableIndex) >= len(table.Addresses) {
			return nil, errors.New("lookup table index out of range")
		}

		ammId = &table.Addresses[lookup.LookupTableIndex]

	} else {
		key := solana.MustPublicKeyFromBase58(tx.AccountKeys[accountIndex])
		ammId = &key
	}

	return ammId, nil
}

func processInitialize2(ins generators.TxInstruction, tx generators.GeyserResponse) {
	ammId, err := getPublicKeyFromTx(4, tx.MempoolTxns, ins)
	if err != nil {
		return
	}

	if ammId == nil {
		log.Print("Unable to retrieve AMM ID")
		return
	}

	tracker, err := bot.GetAmmTrackingStatus(ammId)
	if err != nil {
		log.Print(err)
		return
	}

	if tracker.Status == storage.TRACKED_TRIGGER_ONLY || tracker.Status == storage.TRACKED_BOTH {
		log.Printf("%s | Untracked because of initialize2", ammId)
		bot.PauseAmmTracking(ammId)
	}
}

func processWithdraw(ins generators.TxInstruction, tx generators.GeyserResponse) {
	ammId, err := getPublicKeyFromTx(1, tx.MempoolTxns, ins)
	if err != nil {
		return
	}

	if ammId == nil {
		log.Print("Unable to retrieve AMM ID")
		return
	}

	pKey, err := liquidity.GetPoolKeys(ammId)
	if err != nil {
		log.Printf("%s | %s", ammId, err)
		return
	}

	time.Sleep(time.Duration(config.BuyDelay) * time.Millisecond)

	reserve, err := liquidity.GetPoolSolBalance(pKey)
	if err != nil {
		log.Printf("%s | %s", ammId, err)
		return
	}

	if reserve > uint64(config.LAMPORTS_PER_SOL) {
		log.Printf("%s | Pool still have high balance", ammId)
		return
	}

	tracker, err := bot.GetAmmTrackingStatus(ammId)
	if err != nil {
		log.Print(err)
		return
	}

	if tracker.Status == storage.PAUSE {
		log.Printf("%s | UNPAUSED tracking", ammId)
		bot.TrackedAmm(ammId, false)
		return
	}

	compute := instructions.ComputeUnit{
		MicroLamports: 500000,
		Units:         85000,
		Tip:           0,
	}

	buyToken(pKey, 100000, 0, ammId, compute, false, config.BUY_METHOD)
}

/**
* Process swap base in instruction
 */
func processSwapBaseIn(ins generators.TxInstruction, tx generators.GeyserResponse) {
	var ammId *solana.PublicKey
	var openbookId *solana.PublicKey
	var sourceTokenAccount *solana.PublicKey
	var destinationTokenAccount *solana.PublicKey
	var signerPublicKey *solana.PublicKey

	var err error
	ammId, err = getPublicKeyFromTx(1, tx.MempoolTxns, ins)
	if err != nil {
		return
	}

	if ammId == nil {
		return
	}

	openbookId, err = getPublicKeyFromTx(7, tx.MempoolTxns, ins)
	if err != nil {
		return
	}

	var sourceAccountIndex int
	var destinationAccountIndex int
	var signerAccountIndex int

	if openbookId.String() == config.OPENBOOK_ID.String() {
		sourceAccountIndex = 15
		destinationAccountIndex = 16
		signerAccountIndex = 17
	} else {
		sourceAccountIndex = 14
		destinationAccountIndex = 15
		signerAccountIndex = 16
	}

	if sourceAccountIndex >= len(ins.Accounts) || destinationAccountIndex >= len(ins.Accounts) || signerAccountIndex >= len(ins.Accounts) {
		log.Printf("%s | Invalid data length (%d)", ammId, len(ins.Accounts))
		return
	}

	sourceTokenAccount, err = getPublicKeyFromTx(sourceAccountIndex, tx.MempoolTxns, ins)
	destinationTokenAccount, err = getPublicKeyFromTx(destinationAccountIndex, tx.MempoolTxns, ins)
	signerPublicKey, err = getPublicKeyFromTx(signerAccountIndex, tx.MempoolTxns, ins)

	if sourceTokenAccount == nil || destinationTokenAccount == nil || signerPublicKey == nil {
		return
	}

	tracker, err := bot.GetAmmTrackingStatus(ammId)

	if !signerPublicKey.Equals(config.Payer.PublicKey()) {

		if err != nil {
			log.Print(err)
			return
		}

		if tracker.Status != storage.TRACKED_TRIGGER_ONLY && tracker.Status != storage.TRACKED_BOTH {
			return
		}

	}

	pKey, err := liquidity.GetPoolKeys(ammId)
	if err != nil {
		return
	}

	mint, _, err := liquidity.GetMint(pKey)
	if err != nil {
		return
	}

	amount := bot.GetBalanceFromTransaction(tx.MempoolTxns.PreTokenBalances, tx.MempoolTxns.PostTokenBalances, mint)
	amountSol := bot.GetBalanceFromTransaction(tx.MempoolTxns.PreTokenBalances, tx.MempoolTxns.PostTokenBalances, config.WRAPPED_SOL)

	if signerPublicKey.Equals(config.Payer.PublicKey()) {
		chunk, err := bot.GetTokenChunk(ammId)
		if err != nil {
			if err.Error() == "key not found" {
				bot.SetTokenChunk(ammId, types.TokenChunk{
					Total:     amount,
					Remaining: amount,
					Chunk:     new(big.Int).Div(amount, big.NewInt(config.ChunkSplitter)),
				})

				bot.TrackedAmm(ammId, true)
				log.Printf("%s | Tracked", ammId)
			}
			return
		}

		if chunk.Remaining.Uint64() == 0 {
			bot.UntrackedAmm(ammId)
			log.Printf("%s | Cant deduct more since token remaining is out", ammId)
			return
		} else {
			if tx.MempoolTxns.Error == "" {
				chunk.Remaining = new(big.Int).Sub(chunk.Remaining, amount)
				bot.SetTokenChunk(ammId, chunk)
			}
		}

		return
	}

	// Only proceed if the amount is greater than 0.011 SOL and amount of SOL is a negative number (represent buy action)
	// log.Printf("%s | %d | %s | %s", ammId, amount.Sign(), amountSol, tx.MempoolTxns.Signature)
	// sniper(amount *big.Int, amountSol *big.Int, pKey *types.RaydiumPoolKeys, tx generators.GeyserResponse)

	// Machine gun technique
	startMachineGun(amount, amountSol, tracker, ammId)
	sniper(amount, amountSol, pKey, tx)
}

func startMachineGun(amount *big.Int, amountSol *big.Int, tracker *types.Tracker, ammId *solana.PublicKey) {
	if amount.Sign() == -1 {
		if amountSol.Cmp(big.NewInt(config.MachineGunMinTrigger)) == 1 {
			if tracker.Status != storage.TRACKED_BOTH {
				log.Printf("%s | Set Burst", ammId)
				bot.TrackedAmm(ammId, false)
			}
		}
	}
}

func sniper(amount *big.Int, amountSol *big.Int, pKey *types.RaydiumPoolKeys, tx generators.GeyserResponse) {
	if amount.Sign() == -1 {
		if amountSol.Cmp(big.NewInt(10000000)) == 1 {
			log.Printf("%s | Potential entry %d SOL (Slot %d) | %s", pKey.ID, amountSol, tx.MempoolTxns.Slot, tx.MempoolTxns.Signature)

			compute := instructions.ComputeUnit{
				MicroLamports: 1500,
				Units:         45000,
				Tip:           0,
			}

			var minAmountOut uint64
			var useStakedRPCFlag bool = false
			if amountSol.Uint64() > 10000000 {
				tipBigInt := new(big.Int).Mul(amountSol, big.NewInt(87))
				tipBigInt.Div(tipBigInt, big.NewInt(100))
				compute.Tip = tipBigInt.Uint64()

				mAmount := new(big.Int).Mul(amountSol, big.NewInt(92))
				mAmount.Div(tipBigInt, big.NewInt(100))

				minAmountOut = mAmount.Uint64()

				useStakedRPCFlag = true
			} else {
				compute.Tip = 0
				minAmountOut = 50000
			}

			chunk, err := bot.GetTokenChunk(&pKey.ID)
			if err != nil {
				log.Printf("%s | %s", pKey.ID, err)
				return
			}

			if (chunk.Remaining).Uint64() == 0 {
				log.Printf("%s | Juice out", pKey.ID)
				return
			}

			go sellToken(pKey, chunk, minAmountOut, &pKey.ID, compute, useStakedRPCFlag, "bloxroute")
		}
	}
}

func buyToken(
	pKey *types.RaydiumPoolKeys,
	amount uint64,
	minAmountOut uint64,
	ammId *solana.PublicKey,
	compute instructions.ComputeUnit,
	useStakedRPCFlag bool,
	method string) {

	blockhash, err := solana.HashFromBase58(latestBlockhash)
	if err != nil {
		log.Print(err)
		return
	}

	options := instructions.TxOption{
		Blockhash: blockhash,
	}

	signatures, transaction, err := instructions.MakeSwapInstructions(
		pKey,
		wsolTokenAccount,
		compute,
		options,
		amount,
		minAmountOut,
		"buy",
		config.BUY_METHOD,
	)

	if err != nil {
		log.Printf("%s | %s", ammId, err)
		return
	}

	switch method {
	case "bloxroute":
		rpc.SubmitBloxRouteTransaction(transaction, useStakedRPCFlag)
		break
	case "jito":
		_, err := rpc.SendJitoTransaction(transaction)
		if err != nil {
			log.Printf("%s | %s", ammId, err)
			return
		}
		break
	}

	rpc.SendTransaction(transaction)

	log.Printf("%s | BUY | %s", ammId, signatures)
}

func sellToken(
	pKey *types.RaydiumPoolKeys,
	chunk types.TokenChunk,
	minAmountOut uint64,
	ammId *solana.PublicKey,
	compute instructions.ComputeUnit,
	useStakedRPCFlag bool,
	method string) {

	blockhash, err := solana.HashFromBase58(latestBlockhash)
	if err != nil {
		log.Print(err)
		return
	}

	options := instructions.TxOption{
		Blockhash: blockhash,
	}

	signatures, transaction, err := instructions.MakeSwapInstructions(
		pKey,
		wsolTokenAccount,
		compute,
		options,
		chunk.Chunk.Uint64(),
		minAmountOut,
		"sell",
		method,
	)

	if err != nil {
		log.Printf("%s | %s", ammId, err)
		return
	}

	switch method {
	case "bloxroute":
		rpc.SubmitBloxRouteTransaction(transaction, useStakedRPCFlag)
		break
	case "jito":
		_, err := rpc.SendJitoTransaction(transaction)
		if err != nil {
			log.Printf("%s | %s", ammId, err)
			return
		}
		break
	}

	rpc.SendTransaction(transaction)

	log.Printf("%s | SELL | %s", ammId, signatures)
}

func generateInstructions(ammId *solana.PublicKey, method string) ([]*solana.Transaction, error) {

	var txs []*solana.Transaction = []*solana.Transaction{}

	pKey, err := liquidity.GetPoolKeys(ammId)
	if err != nil {
		return nil, err
	}

	blockhash, err := solana.HashFromBase58(latestBlockhash)
	if err != nil {
		return nil, err
	}

	options := instructions.TxOption{
		Blockhash: blockhash,
	}

	compute := instructions.ComputeUnit{
		MicroLamports: 1005,
		Units:         45000,
		Tip:           0,
	}

	chunk, err := bot.GetTokenChunk(ammId)
	if err != nil {
		log.Printf("%s | %s", ammId, err)
		return nil, err
	}

	if (chunk.Remaining).Uint64() == 0 {
		log.Printf("%s | No more juice", ammId)
		return nil, err
	}

	_, transaction, err := instructions.MakeSwapInstructions(
		pKey,
		wsolTokenAccount,
		compute,
		options,
		chunk.Chunk.Uint64(),
		50000,
		"sell",
		method,
	)

	txs = append(txs, transaction)

	if err != nil {
		log.Printf("%s | %s", ammId, err)
		return nil, err
	}

	return txs, nil
}

func getOrCreateAssociatedTokenAccount() (*solana.PublicKey, error) {

	ata, tx, err := instructions.ValidatedAssociatedTokenAccount(&config.WRAPPED_SOL)
	if err != nil {
		return nil, err
	}

	if tx != nil {
		log.Print("Creating WSOL associated token account")
		rpc.SendTransaction(tx)
	}

	return &ata, nil
}
