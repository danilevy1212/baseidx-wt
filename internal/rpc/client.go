package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/danilevy1212/baseidx-wt/internal/data"
)

type Client struct {
	BaseURL      string
	DebugBaseURL string
	client       *http.Client
}

func NewClient(baseURL, debugBaseURL string) Client {
	return Client{
		BaseURL:      baseURL,
		DebugBaseURL: debugBaseURL,
		client:       &http.Client{},
	}
}

func (c *Client) post(method string, params []any, target any) error {
	body := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal rpc body: %w", err)
	}

	resp, err := c.client.Post(c.BaseURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("rpc post failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rpc error: status code %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("unmarshal rpc response: %w", err)
	}
	return nil
}

func (c *Client) GetBlockByNumber(block data.Hex, full bool) (*BlockDTO, error) {
	var res BlockDTO
	err := c.post("eth_getBlockByNumber", []any{block.String(), full}, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) GetBalance(addr string) (*BalanceDTO, error) {
	var res BalanceDTO
	err := c.post("eth_getBalance", []any{addr, "latest"}, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) GetBlockReceipts(block data.Hex) (*BlockReceiptsDTO, error) {
	var res BlockReceiptsDTO
	err := c.post("eth_getBlockReceipts", []any{block.String()}, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) GetLastestBlock() (*LatestBlockDTO, error) {
	var res LatestBlockDTO
	err := c.post("eth_blockNumber", nil, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) GetTransactionCallTrace(transactionHash string) (*GetTransactionCallTraceDTO, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "debug_traceTransaction",
		"params": []any{
			transactionHash,
			map[string]any{
				"tracer":       "callTracer",
				"tracerConfig": map[string]any{"onlyTopLevel": false},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(
		c.DebugBaseURL,
		"application/json",
		bytes.NewReader(jsonData),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var traceDTO GetTransactionCallTraceDTO
	if err := json.NewDecoder(resp.Body).Decode(&traceDTO); err != nil {
		return nil, err
	}

	return &traceDTO, nil
}
