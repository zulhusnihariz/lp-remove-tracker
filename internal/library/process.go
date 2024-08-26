package bot

import (
	"database/sql"
	"errors"
	"log"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/coder"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/config"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/generators"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/liquidity"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/storage"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/types"
)

var (
	JitoTipAccounts []string
	tipAccount      = []string{"jito", "bloxroute"}
	latestBlockhash string
)

func ProcessResponse(response generators.GeyserResponse) {
	latestBlockhash = response.MempoolTxns.RecentBlockhash

	var (
		isProcess    bool
		ix           generators.TxInstruction
		res          generators.GeyserResponse
		computeLimit uint32
		computePrice uint32
		tipAmount    int64
		tip          string
		status       = "success"
	)

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
				isProcess = true
				ix = ins
				res = response
			case coder.SwapBaseOut:
			default:
				log.Println("Unknown instruction type")
			}
		}

		if programId == config.COMPUTE_PROGRAM.String() {
			computeDecoded, err := c.DecodeCompute(ins.Data)
			if err != nil {
				continue
			}

			if computeDecoded.Instruction == 2 {
				computeLimit = computeDecoded.Value
			}

			if computeDecoded.Instruction == 3 {
				computePrice = computeDecoded.Value
			}
		}

		if programId == config.TRANSFER_PROGRAM.String() {
			transfer, err := c.DecodeTransfer(ins.Data)

			if err != nil {
				continue
			}

			var accounts []string

			for _, idx := range ins.Accounts {
				accountKeysLength := len(response.MempoolTxns.AccountKeys)

				if idx >= uint8(accountKeysLength) {
					accounts = append(accounts, response.MempoolTxns.AccountKeys[accountKeysLength-1])
				} else {
					accounts = append(accounts, response.MempoolTxns.AccountKeys[idx])

				}
			}

			destination := accounts[1]

			if destination == "" {
				continue
			}

			isJitoTipAccount := slices.Contains(JitoTipAccounts, destination)
			isBloxRouteTipAccount := strings.EqualFold(destination, config.BLOXROUTE_TIP.String())

			if isJitoTipAccount {
				tip = tipAccount[0]
				tipAmount = transfer.Amount
			} else if isBloxRouteTipAccount {
				tip = tipAccount[1]
				tipAmount = transfer.Amount
			}
		}

	}

	if response.MempoolTxns.Error != "" {
		status = "failed"
	}

	if isProcess {
		processSwapBaseIn(ix, res, computeLimit, computePrice, tip, tipAmount, status)
	}
}

func getPublicKeyFromTx(pos int, tx generators.MempoolTxn, instruction generators.TxInstruction) (*solana.PublicKey, error) {
	accountIndexes := instruction.Accounts
	if len(accountIndexes) == 0 {
		return nil, errors.New("no account indexes provided")
	}

	lookupsForAccountKeyIndex := GenerateTableLookup(tx.AddressTableLookups)
	var ammId *solana.PublicKey
	accountIndex := int(accountIndexes[pos])

	if accountIndex >= len(tx.AccountKeys) {
		lookupIndex := accountIndex - len(tx.AccountKeys)
		lookup := lookupsForAccountKeyIndex[lookupIndex]
		table, err := GetLookupTable(solana.MustPublicKeyFromBase58(lookup.LookupTableKey))
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

	tracker, err := GetAmmTrackingStatus(ammId)
	if err != nil {
		log.Print(err)
		return
	}

	if tracker.Status == storage.TRACKED_TRIGGER_ONLY || tracker.Status == storage.TRACKED_BOTH {
		log.Printf("%s | Untracked because of initialize2", ammId)
		PauseAmmTracking(ammId)
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

	TrackedAmm(ammId)
}

/**
* Process swap base in instruction
 */
func processSwapBaseIn(ins generators.TxInstruction, tx generators.GeyserResponse, computeLimit uint32, computePrice uint32, tip string, tipAmount int64, status string) {
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

	sourceTokenAccount, _ = getPublicKeyFromTx(sourceAccountIndex, tx.MempoolTxns, ins)
	destinationTokenAccount, _ = getPublicKeyFromTx(destinationAccountIndex, tx.MempoolTxns, ins)
	signerPublicKey, _ = getPublicKeyFromTx(signerAccountIndex, tx.MempoolTxns, ins)

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

	tracker, _ := GetAmmTrackingStatus(ammId)

	if tracker.Status != storage.TRACKED_TRIGGER_ONLY {
		return
	}

	amount := GetBalanceFromTransaction(tx.MempoolTxns.PreTokenBalances, tx.MempoolTxns.PostTokenBalances, mint)
	// amountSol := GetBalanceFromTransaction(tx.MempoolTxns.PreTokenBalances, tx.MempoolTxns.PostTokenBalances, config.WRAPPED_SOL)

	var action string = "SELL"

	if amount.Cmp(big.NewInt(0)) != 0 {
		if amount.Sign() == 1 {
			action = "BUY"
		}
	}

	if tip == "" {
		tip = sql.NullString{}.String
	}

	trade := &types.Trade{
		AmmId:        ammId,
		Mint:         &mint,
		Action:       action,
		ComputeLimit: uint64(computeLimit),
		ComputePrice: uint64(computePrice),
		Amount:       amount.String(),
		Signature:    tx.MempoolTxns.Signature,
		Tip:          tip,
		TipAmount:    tipAmount,
		Status:       status,
		Signer:       signerPublicKey.String(),
	}

	err = SetTrade(trade)

	if err != nil {
		log.Print(err)
	}

	log.Printf("%s | %s | %s | %d | %d | %d | %s", ammId, tx.MempoolTxns.Signature, action, computeLimit, computePrice, amount, tip)

	/* 	if amount.Sign() == 1 {
	   		if amountSol.Cmp(big.NewInt(0)) == 1 {
	   			log.Printf("%s | %s | Potential entry %d SOL (Slot %d) | %s", pKey.ID, tx.MempoolTxns.Source, amountSol, tx.MempoolTxns.Slot, tx.MempoolTxns.Signature)

	   			bot.SetTrade(&types.Trade{
	   				AmmId:     ammId,
	   				Mint:      &mint,
	   				Action:    "BUY",
	   				Amount:    big.NewInt(0).Abs(amount).String(),
	   				Signature: tx.MempoolTxns.Signature,
	   			})
	   		}
	   	}
	*/
	// Only proceed if the amount is greater than 0.011 SOL and amount of SOL is a negative number (represent buy action)
	// log.Printf("%s | %d | %s | %s", ammId, amount.Sign(), amountSol, tx.MempoolTxns.Signature)
	// sniper(amount *big.Int, amountSol *big.Int, pKey *types.RaydiumPoolKeys, tx generators.GeyserResponse)

	// Machine gun technique
	// Sniper technique
}
