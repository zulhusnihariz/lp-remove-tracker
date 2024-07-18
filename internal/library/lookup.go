package bot

import (
	"log"

	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/adapter"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/generators"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/rpc"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/storage"
)

type LookupIndex struct {
	LookupTableIndex uint8
	LookupTableKey   string
}

func GetLookupTable(addr solana.PublicKey) (addresslookuptable.AddressLookupTableState, error) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get LookupTableStorage instance: %v", err)
	}

	account, err := storage.GetLookup(redisClient, addr.String())

	if err != nil {
		return account, err
	}

	resp, err := rpc.GetLookupTable(addr)

	if err != nil {
		return addresslookuptable.AddressLookupTableState{}, err
	}

	storage.SetLookup(redisClient, addr.String(), resp)

	return resp, nil
}

func GenerateTableLookup(addressTableLookups []generators.TxAddressTableLookup) []LookupIndex {
	var lookupIndexes []LookupIndex

	for _, lookup := range addressTableLookups {
		for _, index := range lookup.WritableIndexes {
			lookupIndexes = append(lookupIndexes, LookupIndex{
				LookupTableIndex: index,
				LookupTableKey:   lookup.AccountKey,
			})
		}
		for _, index := range lookup.ReadonlyIndexes {
			lookupIndexes = append(lookupIndexes, LookupIndex{
				LookupTableIndex: index,
				LookupTableKey:   lookup.AccountKey,
			})
		}
	}

	return lookupIndexes
}
