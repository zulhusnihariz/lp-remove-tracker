package main

import (
	"errors"
	"fmt"
	"log"
	"math/big"
	"runtime"
	"sync"
	"time"

	_ "go.uber.org/automaxprocs"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/adapter"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/coder"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/config"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/generators"
	bot "github.com/iqbalbaharum/lp-remove-tracker/internal/library"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/liquidity"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/storage"
)

func loadAdapter() {
	adapter.GetRedisClient(0)
}

var (
	grpcs            []*generators.GrpcClient
	latestBlockhash  string
	wsolTokenAccount solana.PublicKey
	wg               sync.WaitGroup
	txChannel        chan generators.GeyserResponse
)

func main() {
	numCPU := runtime.NumCPU() * 2
	maxProcs := runtime.GOMAXPROCS(0)
	log.Printf("Number of logical CPUs available: %d", numCPU)
	log.Printf("Number of CPUs being used: %d", maxProcs)

	runtime.GOMAXPROCS(runtime.NumCPU())

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	err := config.InitEnv()
	if err != nil {
		log.Print(err)
		return
	}

	err = adapter.InitRedisClients(config.RedisAddr, config.RedisPassword)
	if err != nil {
		log.Fatalf(fmt.Sprintf("Failed to initialize Redis clients: %v", err))
		return
	}

	err = adapter.InitSqlClient(config.MySqlDsn)
	if err != nil {
		log.Fatalf(fmt.Sprintf("Failed to initialize SQL client: %v", err))
		return
	}

	log.Print("Initialized ENVIRONMENT successfully")

	client, err := generators.GrpcConnect(config.GRPC1.Addr, config.GRPC1.InsecureConnection)
	client2, err := generators.GrpcConnect(config.GRPC2.Addr, config.GRPC2.InsecureConnection)

	grpcs = append(grpcs, client, client2)

	if err != nil {
		log.Fatalf("Error in GRPC connection: %s ", err)
		return
	}

	txChannel = make(chan generators.GeyserResponse)

	var processed sync.Map

	// Create a worker pool
	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for response := range txChannel {
				if _, exists := processed.Load(response.MempoolTxns.Signature); !exists {
					processed.Store(response.MempoolTxns.Signature, true)
					processResponse(response)

					time.AfterFunc(1*time.Minute, func() {
						processed.Delete(response.MempoolTxns.Signature)
					})
				}
			}
		}()
	}

	// wg.Add(1)
	listenFor(
		grpcs[0],
		"triton",
		[]string{
			config.RAYDIUM_AMM_V4.String(),
		}, txChannel, &wg)

	listenFor(
		grpcs[1],
		"solana-tracker",
		[]string{
			config.RAYDIUM_AMM_V4.String(),
		}, txChannel, &wg)

	wg.Wait()

	for i := 0; i < len(grpcs); i++ {
		grpc := grpcs[i]
		defer func() {
			if err := grpc.CloseConnection(); err != nil {
				log.Printf("Error closing gRPC connection: %v", err)
			}
		}()
	}
}

// Listening geyser for new addresses
func listenFor(client *generators.GrpcClient, name string, addresses []string, txChannel chan generators.GeyserResponse, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := client.GrpcSubscribeByAddresses(
			name,
			config.GrpcToken,
			addresses,
			[]string{}, txChannel)
		if err != nil {
			log.Printf("Error in first gRPC subscription: %v", err)
		}
	}()
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
				log.Printf("Initialize2 | %s | %s", response.MempoolTxns.Source, response.MempoolTxns.Signature)
				processInitialize2(ins, response)
			case coder.Withdraw:
				log.Printf("Withdraw | %s | %s", response.MempoolTxns.Source, response.MempoolTxns.Signature)
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

	time.Sleep(time.Duration(500) * time.Millisecond)

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

	// buyToken(pKey, 100000, 0, ammId, compute, false, config.BUY_METHOD)
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

	if amount.Sign() == -1 {
		if amountSol.Cmp(big.NewInt(10000000)) == 1 {
			log.Printf("%s | %s | Potential entry %d SOL (Slot %d) | %s", pKey.ID, tx.MempoolTxns.Source, amountSol, tx.MempoolTxns.Slot, tx.MempoolTxns.Signature)
		}
	}

	// Only proceed if the amount is greater than 0.011 SOL and amount of SOL is a negative number (represent buy action)
	// log.Printf("%s | %d | %s | %s", ammId, amount.Sign(), amountSol, tx.MempoolTxns.Signature)
	// sniper(amount *big.Int, amountSol *big.Int, pKey *types.RaydiumPoolKeys, tx generators.GeyserResponse)

	// Machine gun technique
	// Sniper technique
}
