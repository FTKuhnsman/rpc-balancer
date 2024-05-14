package rpcbalancer

import (
	"bytes"
	//"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type node struct {
	URI     string
	Client  *fasthttp.Client
	Healthy bool

	mu sync.RWMutex
}

func NewNode(addr string) *node {

	metrics.SetHealth(addr, true)
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
	metrics.SetHealth(n.URI, healthy)
}

type Pool struct {
	Nodes           map[int]*node
	numNodes        int
	Fallback        *node
	SelectionMethod string
	RoundRobinChan  chan *node
	mu              sync.RWMutex
}

func NewPool(selectionMethod string) *Pool {
	return &Pool{
		Nodes:           make(map[int]*node),
		SelectionMethod: selectionMethod,
		RoundRobinChan:  make(chan *node, 1000), // setting a buffer size of 1000 to avoid blocking. This is a temporary fix and should be replaced with a more robust solution
		mu:              sync.RWMutex{},
	}
}

func (p *Pool) AddNode(n *node, id int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Nodes[id] = n
	p.RoundRobinChan <- n
	p.numNodes++
}

func (p *Pool) getHealthyNode() (*node, bool) {
	switch p.SelectionMethod {
	case "failover":
		return p.getNodeByFailoverOrder()
	case "roundrobin":
		return p.getNodeByRoundRobin()
	case "random":
		return p.getNodeByRandom()
	default:
		return p.getNodeByFailoverOrder()
	}
}

func (p *Pool) getNodeByFailoverOrder() (*node, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for i := 0; i < len(p.Nodes); i++ {
		// DEBUGGING ONLY
		// log.Print(p.Nodes[i].URI, p.Nodes[i].Healthy)
		if p.Nodes[i].Healthy {
			return p.Nodes[i], false
		}
	}

	return p.Fallback, true
}

func (p *Pool) getNodeByRoundRobin() (*node, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for i := 0; i < len(p.Nodes); i++ {
		n := <-p.RoundRobinChan
		p.RoundRobinChan <- n
		if n.Healthy {
			return n, false
		}
	}

	return p.Fallback, true
}

func (p *Pool) getNodeByRandom() (*node, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var keys []int
	for k := range p.Nodes {
		keys = append(keys, k)
	}

	rand.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })

	for _, k := range keys {
		if p.Nodes[k].Healthy {
			return p.Nodes[k], false
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
		log.WithFields(logrus.Fields{
			"error":    err,
			"req_body": string(ctx.PostBody()),
		}).Error("Failed to unmarshal request.")
		// update request metrics
		go metrics.IncrementInvalidRequests()
		return
	}

	// update request metrics
	go metrics.IncrementTotalRequests(jRPCReq.Method)

	// Get the client to use for the request
	for {
		node, fallback := p.getHealthyNode()
		if fallback && node == nil {
			ctx.Error("No healthy nodes available", 521)
			log.Error("No healthy nodes available")
			return
		}

		log.WithFields(logrus.Fields{
			"node":        node.URI,
			"is_fallback": fallback,
		}).Debug("selected node")

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
				log.WithFields(logrus.Fields{
					"error": err,
				}).Error("Request failed at fallback")
				return
			default:
				node.SetHealthy(false)
				log.WithFields(logrus.Fields{
					"node":    node.URI,
					"healthy": node.Healthy,
				}).Error("Request failed. Node set to unhealthy.")
				continue
			}
		}

		if resp.StatusCode() != 200 {
			switch fallback {
			case true:
				ctx.Error("Request failed at fallback: ", 521)
				log.WithFields(logrus.Fields{
					"http_status_code": resp.StatusCode(),
				}).Error("Request failed at fallback")
				return
			default:
				node.SetHealthy(false)
				log.WithFields(logrus.Fields{
					"http_status_code": resp.StatusCode(),
				}).Error("Request failed. Node set to unhealthy.")
				continue
			}
		}

		// Write the response back to the client
		ctx.Response.SetStatusCode(resp.StatusCode())
		ctx.Response.SetBody(resp.Body())

		resp.Header.VisitAll(func(key, value []byte) {
			ctx.Response.Header.SetBytesKV(key, value)
		})
		log.Info("Request succeeded")
		return
	}
}

func (p *Pool) StartHealthCheckLoop(frequency int) {
	for {
		for _, n := range p.Nodes {
			if !n.Healthy {
				height, err := GetBlockHeight(n.URI)
				if err != nil {
					log.WithFields(logrus.Fields{
						"error": err,
						"node":  n.URI,
					}).Error("Error getting block height")
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
	Jsonrpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
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
		return 0, fmt.Errorf("http_status_code: %d", resp.StatusCode)
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
		return 0, err
	}

	return blockHeight, nil
}
