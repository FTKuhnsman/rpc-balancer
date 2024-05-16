package rpcbalancer

import (
	"strings"
	"sync"

	"github.com/valyala/fasthttp"
)

var UrlKey string

func RunWebServer(wg *sync.WaitGroup, port string, pool *Pool, urlKey string) {
	defer wg.Done()

	handler := func(ctx *fasthttp.RequestCtx) {
		pool.HandleRequest(ctx)
	}

	// Start the server
	log.Print(":" + port)
	log.Fatal(fasthttp.ListenAndServe(":"+port, Auth(handler)))
}

func Auth(requestHandler fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		// If no key is set, just pass the request
		if UrlKey == "" {
			requestHandler(ctx)
			return
		}

		// Get the key from the URL
		path := string(ctx.Path())
		keyRaw := strings.Split(path, "/")

		var key []string
		for _, v := range keyRaw {
			if v != "" {
				key = append(key, v)
			}
		}

		// ****uncomment to troubleshoot key index
		//log.Printf("key slice length %v", len(key))
		//log.Printf("key slice: %v", key)

		if len(key) > 0 {
			if key[0] != UrlKey {
				ctx.Error("Unauthorized", fasthttp.StatusUnauthorized)
				return
			}

			requestHandler(ctx)
		}
	}
}
