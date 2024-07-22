package instructions

import (
	"bytes"
	"encoding/binary"
	"fmt"

	ag_binary "github.com/gagliardetto/binary"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/config"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/types"
)

type RaydiumSwapInstruction struct {
	bin.BaseVariant
	InAmount                uint64
	MinimumOutAmount        uint64
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

type LiquiditySwapFixedInInstructionParams struct {
	InAmount         uint64
	MinimumOutAmount uint64
	PoolKeys         types.RaydiumPoolKeys
	TokenAccountIn   solana.PublicKey
	TokenAccountOut  solana.PublicKey
	Owner            solana.PublicKey
}

func (instruction *RaydiumSwapInstruction) ProgramID() solana.PublicKey {
	return config.RAYDIUM_AMM_V4
}

func (instruction *RaydiumSwapInstruction) Accounts() (out []*solana.AccountMeta) {
	return instruction.GetAccounts()
}

func (instruction *RaydiumSwapInstruction) GetAccounts() []*solana.AccountMeta {
	return instruction.AccountMetaSlice
}

func (instruction *RaydiumSwapInstruction) Data() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := bin.NewBorshEncoder(buf).Encode(instruction); err != nil {
		return nil, fmt.Errorf("unable to encode instruction: %w", err)
	}
	return buf.Bytes(), nil
}

func (instruction *RaydiumSwapInstruction) MarshalWithEncoder(encoder *bin.Encoder) (err error) {
	// Swap instruction is number 9
	err = encoder.WriteUint8(9)
	if err != nil {
		return err
	}
	err = encoder.WriteUint64(instruction.InAmount, binary.LittleEndian)
	if err != nil {
		return err
	}
	err = encoder.WriteUint64(instruction.MinimumOutAmount, binary.LittleEndian)
	if err != nil {
		return err
	}
	return nil
}

func MakeRaydiumSwapFixedInInstruction(params *LiquiditySwapFixedInInstructionParams) *RaydiumSwapInstruction {

	ins := &RaydiumSwapInstruction{
		InAmount:         params.InAmount,
		MinimumOutAmount: params.MinimumOutAmount,
		AccountMetaSlice: make(solana.AccountMetaSlice, 0),
	}

	ins.BaseVariant = bin.BaseVariant{
		Impl:   ins,
		TypeID: ag_binary.TypeIDFromUint32(25, binary.LittleEndian),
	}

	accountMetas := []*solana.AccountMeta{
		solana.Meta(solana.TokenProgramID).WRITE(),        // Token program ID
		solana.Meta(params.PoolKeys.ID).WRITE(),           // Pool ID
		solana.Meta(params.PoolKeys.Authority),            // Pool authority
		solana.Meta(params.PoolKeys.OpenOrders).WRITE(),   // Open orders
		solana.Meta(params.PoolKeys.TargetOrders).WRITE(), // Target orders
	}

	// Add BaseVault and QuoteVault
	accountMetas = append(accountMetas,
		solana.Meta(params.PoolKeys.BaseVault).WRITE(),  // Base vault
		solana.Meta(params.PoolKeys.QuoteVault).WRITE(), // Quote vault
	)

	// Add remaining account metas
	accountMetas = append(accountMetas,
		solana.Meta(params.PoolKeys.MarketProgramID),          // Serum program ID
		solana.Meta(params.PoolKeys.MarketID).WRITE(),         // Serum market ID
		solana.Meta(params.PoolKeys.MarketBids).WRITE(),       // Serum bids
		solana.Meta(params.PoolKeys.MarketAsks).WRITE(),       // Serum asks
		solana.Meta(params.PoolKeys.MarketEventQueue).WRITE(), // Serum event queue
		solana.Meta(params.PoolKeys.MarketBaseVault).WRITE(),  // Serum base vault
		solana.Meta(params.PoolKeys.MarketQuoteVault).WRITE(), // Serum quote vault
		solana.Meta(params.PoolKeys.MarketAuthority),          // Serum authority
		solana.Meta(params.TokenAccountIn).WRITE(),            // User source token account
		solana.Meta(params.TokenAccountOut).WRITE(),           // User destination token account
		solana.Meta(params.Owner).SIGNER(),                    // User owner
	)

	ins.AccountMetaSlice = accountMetas

	return ins
}

func GetAssociatedTokenAccount(mint solana.PublicKey) (solana.PublicKey, error) {

	tokenAccount, _, err := solana.FindAssociatedTokenAddress(config.Payer.PublicKey(), mint)
	if err != nil {
		return solana.PublicKey{}, err
	}

	return tokenAccount, nil
}

func CreateMemoInstruction(from solana.PublicKey, programId solana.PublicKey, memo string) solana.Instruction {
	accountMetas := []*solana.AccountMeta{}
	memoInstruction := solana.NewInstruction(programId, accountMetas, []byte(memo))
	return memoInstruction
}
