package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	QNG_RPC        = "/qng"
	BUNDLER_RPC    = "/bundler"
	EXPORT_RPC     = "/export"
	//RPC_URL        = "http://146.196.54.208:1234"
	RPC_URL        = "http://127.0.0.1:18545"
	BUNDLER_URL    = "http://127.0.0.1:3000/rpc"
	CROSS_CONTRACT = "xxxxxxxx"
)

type JsonRpc struct {
	JsonRpc string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}
type ExportTx struct {
	Txid string `json:"txid"`
	Idx  uint32 `json:"idx"`
	Fee  uint64 `json:"fee"`
	Sig  string `json:"sig"`
}

func ConvertStrToInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

func Export4337(e ExportTx) string {
	ctx := context.Background()
	eclient, _ := ethclient.Dial(RPC_URL)
	chainId, _ := eclient.ChainID(ctx)
	privKey, _ := crypto.HexToECDSA("e15169d9025c023afb125f2e671b2718c23c13cf13302f159d9155e2fcc7eded")
	exportClient, _ := NewMeerchange(common.HexToAddress(CROSS_CONTRACT), eclient)
	auth, _ := bind.NewKeyedTransactorWithChainID(privKey, chainId)
	b, _ := hex.DecodeString(e.Txid)
	txidBytes := common.BytesToHash(b)
	fmt.Println(txidBytes, "txidBytes")
	fmt.Println(e.Txid, e.Idx, e.Fee, e.Sig)
	tx, err := exportClient.Export4337(auth, txidBytes, e.Idx, e.Fee, e.Sig)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	fmt.Println("send succ", tx.Hash().Hex())
	return tx.Hash().Hex()
}

func ProxyHandler(w http.ResponseWriter, r *http.Request) {
	// log.Println("Request Headers:")
	// for name, values := range r.Header {
	// 	for _, value := range values {
	// 		log.Printf("%s: %s", name, value)
	// 	}
	// }
	log.Println("Request Uri:", r.RequestURI)
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if EXPORT_RPC == r.RequestURI {
		b, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		var req JsonRpc
		json.Unmarshal(b, &req)
		params := ExportTx{}
		txid := ""
		// 将响应头的 Content-Type 设置为 application/json
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Connection", "keep-alive") // 保持连接
		w.WriteHeader(http.StatusOK)
		if len(req.Params) == 4 {
			params.Txid = req.Params[0].(string)
			params.Idx = uint32(ConvertStrToInt(req.Params[1].(string)))
			params.Fee = uint64(ConvertStrToInt(req.Params[2].(string)))
			params.Sig = req.Params[3].(string)
			txid = Export4337(params)
		} else {
			w.Write([]byte(`{"code":500,"message":"params error","result":""}`))
			return
		}
		w.Write([]byte(fmt.Sprintf(`{"code":0,"message":"OK","result":%s}`, txid)))
		return
	}
	rurl := RPC_URL
	if r.RequestURI == BUNDLER_RPC {
		rurl = BUNDLER_URL
	}
	b1, _ := io.ReadAll(r.Body)
	fmt.Println("------------", rurl, string(b1))
	proxyReq, err := http.NewRequest(r.Method, rurl, bytes.NewReader(b1))
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	proxyReq.Header = r.Header
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Connection", "keep-alive") // 保持连接
	client := &http.Client{}
	// transport := &http.Transport{
	// 	Proxy: func(req *http.Request) (*url.URL, error) {
	// 		return url.Parse("http://127.0.0.1:1080")
	// 	},
	// }
	// client := &http.Client{Transport: transport}

	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Failed to get response from backend", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		for _, value := range values {
			if name == "Access-Control-Allow-Origin" {
				continue
			}
			if name == "Access-Control-Allow-Methods" {
				continue
			}
			if name == "Access-Control-Allow-Headers" {
				continue
			}
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
}

func main() {
	http.HandleFunc("/", ProxyHandler)
	log.Println("Proxy server listening on :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
