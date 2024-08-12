package rpc

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gagliardetto/solana-go"
	solanaRpc "github.com/gagliardetto/solana-go/rpc"
	"github.com/iqbalbaharum/go-arbi-bot/internal/config"
	"github.com/mr-tron/base58"
	jito_go "github.com/weeaa/jito-go"
	searcher_client "github.com/weeaa/jito-go/clients/searcher_client"
)

type JitoRequestBody struct {
	Jsonrpc string     `json:"jsonrpc"`
	ID      int        `json:"id"`
	Method  string     `json:"method"`
	Params  [][]string `json:"params"`
}

// ResponseBody represents the structure of the response from the Jito API.
type JitoResponseBody struct {
	Jsonrpc string             `json:"jsonrpc"`
	ID      int                `json:"id"`
	Result  string             `json:"result,omitempty"`
	Error   *JitoErrorResponse `json:"error,omitempty"`
}

type JitoErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type JitoRpc struct {
	client *searcher_client.Client
}

func NewJitoClient() (*JitoRpc, error) {
	ctx := context.Background()

	client, err := searcher_client.New(
		ctx,
		jito_go.Amsterdam.BlockEngineURL,
		solanaRpc.New(jito_go.Amsterdam.BlockEngineURL),
		solanaRpc.New(config.RpcHttpUrl),
		config.JitoAuthPrivateKey.PrivateKey,
		nil)

	if err != nil {
		return nil, err
	}

	return &JitoRpc{
		client: client,
	}, nil
}

func (j *JitoRpc) StreamJitoTransaction(transaction *solana.Transaction, recentBlockhash string) error {
	txns := make([]*solana.Transaction, 0, 1)

	txns = append(txns, transaction)
	_, err := j.client.BroadcastBundle(txns)
	if err != nil {
		return err
	}

	return nil
}

func SendJitoTransaction(transaction *solana.Transaction) (*JitoResponseBody, error) {

	// Encode the transaction to base58
	msg, err := transaction.MarshalBinary()
	if err != nil {
		return nil, err
	}

	var messages []string
	base58Msg := base58.Encode(msg)

	messages = append(messages, base58Msg)

	requestBody := JitoRequestBody{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "sendTransaction",
		Params:  [][]string{messages},
	}

	// Marshal the request body to JSON
	reqBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	// Compress the request body using gzip
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	_, err = gzipWriter.Write(reqBody)
	if err != nil {
		return nil, err
	}
	gzipWriter.Close()

	// Create the HTTP request
	url := fmt.Sprintf("%s/api/v1/transactions", config.BLOCKENGINE_URL)
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var responseBody JitoResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		return nil, err
	}

	if responseBody.Error != nil {
		return nil, err
	}

	return &responseBody, nil
}

func SendJitoBundle(transaction *solana.Transaction) (*JitoResponseBody, error) {

	// Encode the transaction to base58
	msg, err := transaction.MarshalBinary()
	if err != nil {
		return nil, err
	}

	var messages []string

	base58Msg := base58.Encode(msg)

	messages = append(messages, base58Msg)

	requestBody := JitoRequestBody{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "sendBundle",
		Params:  [][]string{messages},
	}

	// Marshal the request body to JSON
	reqBody, err := json.Marshal(requestBody)

	if err != nil {
		return nil, err
	}

	// Create the HTTP request
	url := fmt.Sprintf("%s/api/v1/bundles", config.BLOCKENGINE_URL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))

	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var responseBody JitoResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		return nil, err
	}

	if responseBody.Error != nil {
		return nil, err
	}

	return &responseBody, nil
}
