package rpc

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/coder"
)

type AccountInfo struct {
	Value *AccountInfoValue `json:"value"`
}

type AccountInfoValue struct {
	Data       []string `json:"data"`
	Owner      string   `json:"owner"`
	Lamports   uint64   `json:"lamports"`
	Executable bool     `json:"executable"`
}

type RequestBody struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type ResponseBody struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *RPCError       `json:"error"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// var url = os.Getenv("HTTP_RPC_URL")
const url = "https://lineage-ams.rpcpool.com/390cc92f-d182-4400-a829-9524d8a9e23a"

func CallRPC(method string, params interface{}) (*ResponseBody, error) {
	requestBody := RequestBody{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	reqBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))

	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	// req.Header.Set("Accept-Encoding", "gzip")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
	default:
		reader = resp.Body
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var responseBody ResponseBody
	if err := json.Unmarshal(body, &responseBody); err != nil {
		return nil, err
	}

	if responseBody.Error != nil {
		return nil, errors.New(responseBody.Error.Message)
	}

	return &responseBody, nil
}

func GetAccountInfo(publicKey solana.PublicKey, dataSlice *rpc.DataSlice) (*AccountInfo, error) {
	params := map[string]interface{}{
		"encoding":   "base64",
		"commitment": "confirmed",
	}

	if dataSlice != nil {
		params["dataSlice"] = map[string]interface{}{
			"offset": dataSlice.Offset,
			"length": dataSlice.Length,
		}
	}

	reqParams := []interface{}{
		publicKey,
		params,
	}

	response, err := CallRPC("getAccountInfo", reqParams)
	if err != nil {
		return nil, err
	}

	var accountInfo AccountInfo
	if err := json.Unmarshal(response.Result, &accountInfo); err != nil {
		return nil, err
	}

	return &accountInfo, nil
}

func GetBalance(publicKey solana.PublicKey) (uint64, error) {
	params := map[string]interface{}{
		"commitment": "confirmed",
	}

	reqParams := []interface{}{
		publicKey,
		params,
	}

	response, err := CallRPC("getBalance", reqParams)
	if err != nil {
		return 0, err
	}

	var balance rpc.GetBalanceResult
	if err := json.Unmarshal(response.Result, &balance); err != nil {
		return 0, err
	}

	return balance.Value, nil
}

func GetLookupTable(addr solana.PublicKey) (addresslookuptable.AddressLookupTableState, error) {
	resp, err := GetAccountInfo(addr, nil)

	if err != nil {
		return addresslookuptable.AddressLookupTableState{}, err
	}

	if resp == nil || resp.Value == nil {
		return addresslookuptable.AddressLookupTableState{}, nil
	}

	// Decode base64 encoded data
	data, err := base64.StdEncoding.DecodeString(resp.Value.Data[0])

	if err != nil {
		return addresslookuptable.AddressLookupTableState{}, err
	}

	var lookupTableState addresslookuptable.AddressLookupTableState
	err = lookupTableState.UnmarshalWithDecoder(bin.NewBorshDecoder(data))
	if err != nil {
		return addresslookuptable.AddressLookupTableState{}, err
	}

	return lookupTableState, nil
}

// Liquidity State

func GetLiquidityState(ammId *solana.PublicKey) (*coder.LiquidityState, error) {
	resp, err := GetAccountInfo(*ammId, nil)

	c := coder.NewRaydiumLiquidityCoder()

	if resp == nil || resp.Value == nil {
		return &coder.LiquidityState{}, nil
	}

	// Decode base64 encoded data
	data, err := base64.StdEncoding.DecodeString(resp.Value.Data[0])

	if err != nil {
		return &coder.LiquidityState{}, err
	}

	state, err := c.RaydiumLiquidityDecode(data)
	if err != nil {
		return &coder.LiquidityState{}, err
	}

	return &state, nil
}

func GetMarketState(marketId *solana.PublicKey) (*coder.MarketStateLayoutV3, error) {
	resp, err := GetAccountInfo(*marketId, nil)

	c := coder.NewRaydiumMarketCoder()

	if resp == nil || resp.Value == nil {
		return &coder.MarketStateLayoutV3{}, nil
	}

	// Decode base64 encoded data
	data, err := base64.StdEncoding.DecodeString(resp.Value.Data[0])

	if err != nil {
		return &coder.MarketStateLayoutV3{}, err
	}

	state, err := c.RaydiumMarketDecode(data)
	if err != nil {
		return &coder.MarketStateLayoutV3{}, err
	}

	return &state, nil
}
