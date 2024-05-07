package rpcbalancer

import (
	"log"
	"sync"

	"github.com/valyala/fasthttp"
)

func RunWebServer(wg *sync.WaitGroup, port string, pool *Pool) {
	defer wg.Done()

	handler := func(ctx *fasthttp.RequestCtx) {
		pool.HandleRequest(ctx)
	}

	// Start the server
	log.Print(":" + port)
	log.Fatal(fasthttp.ListenAndServe(":"+port, handler))
}
