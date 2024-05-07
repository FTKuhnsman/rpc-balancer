package rpcbalancer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

type node struct {
	URI     string
	Client  *fasthttp.Client
	Healthy bool

	mu sync.RWMutex
}

func NewNode(addr string) *node {

	// parsedURL, err := url.Parse(addr)
	// if err != nil {
	// 	log.Fatal("Failed to parse URL: ", err)
	// }

	//host := parsedURL.Host
	return &node{
		Client: &fasthttp.Client{
			// Addr:                host,
			// IsTLS:               true,
			// MaxIdleConnDuration: fasthttp.DefaultMaxIdleConnDuration,
		},
		Healthy: true,
		URI:     addr,
	}
}

func (n *node) SetHealthy(healthy bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.Healthy = healthy
}

type Pool struct {
	Nodes    map[int]*node
	numNodes int
	Fallback *node
	mu       sync.RWMutex
}

func NewPool() *Pool {
	return &Pool{
		Nodes: make(map[int]*node),
		mu:    sync.RWMutex{},
	}
}

func (p *Pool) AddNode(n *node, id int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Nodes[id] = n
	p.numNodes++
}

func (p *Pool) getHealthyNode() (*node, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for i := 0; i < len(p.Nodes); i++ {
		log.Print(p.Nodes[i].URI, p.Nodes[i].Healthy)
		if p.Nodes[i].Healthy {
			return p.Nodes[i], false
		}
	}

	return p.Fallback, true
}

func (p *Pool) HandleRequest(ctx *fasthttp.RequestCtx) {
	// Update the request for the backend server
	req := &ctx.Request

	// Unmarshal the request body into a struct for caching
	var jRPCReq jsonRPCPayload
	err := json.Unmarshal(ctx.PostBody(), &jRPCReq)
	if err != nil {
		ctx.Error("Failed to unmarshal request: "+err.Error(), 520)
		log.Print("Failed to unmarshal request.")
		// update request metrics
		go metrics.IncrementInvalidRequests()
		return
	}

	// DEBUG
	//log.Print(string(ctx.PostBody()))

	// update request metrics
	go metrics.IncrementTotalRequests(jRPCReq.Method)

	// Get the client to use for the request
	for {
		node, fallback := p.getHealthyNode()
		if fallback && node == nil {
			ctx.Error("No healthy nodes available", 521)
			log.Printf("No healthy nodes available")
			return
		}

		log.Printf("Node tried: %s", node.URI)
		log.Printf("Fallback:%v", fallback)

		// Set the request host to the backend server
		//req.Header.SetHost(node.Client.Addr)
		req.Header.Set("X-Forwarded-For", ctx.RemoteIP().String())
		ctx.Request.SetRequestURI(node.URI)

		ctx.Request.Header.VisitAll(func(key, value []byte) {
			req.Header.SetBytesKV(key, value)
		})

		// Copy the request body, if any
		if len(ctx.PostBody()) > 0 {
			req.SetBody(ctx.PostBody())
		}

		// Make a request to the backend server
		resp := fasthttp.AcquireResponse()
		err := node.Client.Do(req, resp)
		if err != nil {
			switch fallback {
			case true:
				ctx.Error("Request failed at fallback: ", 521)
				log.Printf("Request failed at fallback")
				return
			default:
				node.SetHealthy(false)
				log.Printf("Request failed. Node set to unhealthy.")
				continue
			}
		}

		if resp.StatusCode() != 200 {
			switch fallback {
			case true:
				ctx.Error("Request failed at fallback: ", 521)
				log.Printf("Request failed at fallback")
				return
			default:
				node.SetHealthy(false)
				log.Printf("Request failed. Node set to unhealthy.")
				continue
			}
		}

		// Write the response back to the client
		ctx.Response.SetStatusCode(resp.StatusCode())
		ctx.Response.SetBody(resp.Body())

		resp.Header.VisitAll(func(key, value []byte) {
			ctx.Response.Header.SetBytesKV(key, value)
		})
		log.Printf("Request succeeded")
		return
	}
}

func (p *Pool) StartHealthCheckLoop(frequency int) {
	for {
		for _, n := range p.Nodes {
			if !n.Healthy {
				height, err := GetBlockHeight(n.URI)
				if err != nil {
					log.Printf("Error getting block height: %v", err)
					n.SetHealthy(false)
				} else {
					n.SetHealthy(true)
					metrics.BlockHeight.Set(float64(height))
				}
			}
		}
		time.Sleep(time.Duration(frequency) * time.Second)
	}
}

type jsonRPCPayload struct {
	Jsonrpc string   `json:"jsonrpc"`
	ID      int      `json:"id"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
}

func GetBlockHeight(target string) (int64, error) {
	request := jsonRPCPayload{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "eth_blockNumber",
		Params:  []string{},
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest("POST", target, bytes.NewBuffer(requestBody))

	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode == 429 {
		return 0, fmt.Errorf("rate limited")
	}

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("error getting block height")
	}

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return 0, err
	}

	blockHeight, err := hexStringToInt64(result["result"].(string))
	if err != nil {
		return 0, fmt.Errorf("error getting block height")
	}

	return blockHeight, nil
}
