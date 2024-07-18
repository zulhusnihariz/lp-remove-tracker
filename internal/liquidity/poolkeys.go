package liquidity

import (
	"errors"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/config"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/rpc"
)

type RaydiumPoolKeys struct {
	ID                 string
	BaseMint           solana.PublicKey
	QuoteMint          solana.PublicKey
	LpMint             solana.PublicKey
	BaseDecimals       int
	QuoteDecimals      int
	LpDecimals         int
	Version            int
	ProgramID          solana.PublicKey
	Authority          solana.PublicKey
	OpenOrders         solana.PublicKey
	TargetOrders       solana.PublicKey
	BaseVault          solana.PublicKey
	QuoteVault         solana.PublicKey
	WithdrawQueue      solana.PublicKey
	LpVault            solana.PublicKey
	MarketProgramID    solana.PublicKey
	MarketID           solana.PublicKey
	MarketAuthority    solana.PublicKey
	MarketBaseVault    solana.PublicKey
	MarketQuoteVault   solana.PublicKey
	MarketBids         solana.PublicKey
	MarketAsks         solana.PublicKey
	MarketEventQueue   solana.PublicKey
	LookupTableAccount solana.PublicKey
}

func GetPoolKeys(ammId *solana.PublicKey) (*RaydiumPoolKeys, error) {

	state, err := rpc.GetLiquidityState(ammId)
	if err != nil {
		return &RaydiumPoolKeys{}, err
	}

	authority, err := getAssociatedAuthority(config.RAYDIUM_AMM_V4.PublicKey())
	if err != nil {
		return &RaydiumPoolKeys{}, err
	}

	pKey := &RaydiumPoolKeys{
		ID:                 ammId.String(),
		BaseMint:           state.BaseMint,
		QuoteMint:          state.QuoteMint,
		LpMint:             state.LpMint,
		BaseDecimals:       int(state.BaseDecimal),
		QuoteDecimals:      int(state.QuoteDecimal),
		Authority:          authority,
		OpenOrders:         state.OpenOrders,
		TargetOrders:       state.TargetOrders,
		BaseVault:          state.BaseVault,
		QuoteVault:         state.QuoteVault,
		Version:            3,
		MarketProgramID:    state.MarketProgramId,
		MarketID:           state.MarketId,
		MarketAuthority:    authority,
		WithdrawQueue:      state.WithdrawQueue,
		LpVault:            state.LpVault,
		LookupTableAccount: solana.PublicKey{},
	}

	marketInfo, err := rpc.GetMarketState(&state.MarketId)

	if err != nil {
		return &RaydiumPoolKeys{}, err
	}

	pKey.MarketBaseVault = marketInfo.BaseVault
	pKey.MarketQuoteVault = marketInfo.QuoteVault
	pKey.MarketBids = marketInfo.Bids
	pKey.MarketAsks = marketInfo.Asks
	pKey.MarketEventQueue = marketInfo.EventQueue

	return pKey, nil
}

func GetMint(pKey *RaydiumPoolKeys) (solana.PublicKey, bool, error) {

	var mint solana.PublicKey = solana.PublicKey{}
	var swap = false
	if pKey.BaseMint == config.WRAPPED_SOL {
		mint = pKey.QuoteMint
		swap = true
	} else {
		if pKey.QuoteMint == config.WRAPPED_SOL {
			mint = pKey.BaseMint
		} else {
			return solana.PublicKey{}, false, errors.New("neither BaseMint nor QuoteMint is WRAPPED_SOL")
		}
	}

	return mint, swap, nil
}

func GetPoolSolBalance(pKey *RaydiumPoolKeys) (uint64, error) {

	_, swap, err := GetMint(pKey)
	if err != nil {
		return 0, err
	}

	var value uint64
	if !swap {
		value, err = rpc.GetBalance(pKey.QuoteVault)
	} else {
		value, err = rpc.GetBalance(pKey.BaseVault)
	}

	if err != nil {
		return 0, err
	}

	return value, nil
}

func getAssociatedAuthority(programId solana.PublicKey) (solana.PublicKey, error) {
	seed := []byte{97, 109, 109, 32, 97, 117, 116, 104, 111, 114, 105, 116, 121}
	programAddress, _, err := solana.FindProgramAddress([][]byte{seed}, programId)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return programAddress, nil
}
