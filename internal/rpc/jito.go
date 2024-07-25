package rpc

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/config"
	"github.com/mr-tron/base58"
)

type JitoRequestBody struct {
	Jsonrpc string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// ResponseBody represents the structure of the response from the Jito API.
type JitoResponseBody struct {
	Jsonrpc string             `json:"jsonrpc"`
	ID      int                `json:"id"`
	Result  json.RawMessage    `json:"result,omitempty"`
	Error   *JitoErrorResponse `json:"error,omitempty"`
}

type JitoErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func SendJitoTransaction(transaction *solana.Transaction) (*JitoResponseBody, error) {

	// Encode the transaction to base58
	msg, err := transaction.MarshalBinary()
	if err != nil {
		return nil, err
	}

	base58Msg := base58.Encode(msg)
	log.Print(base58Msg)
	requestBody := JitoRequestBody{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "sendTransaction",
		Params:  []interface{}{base58Msg},
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
