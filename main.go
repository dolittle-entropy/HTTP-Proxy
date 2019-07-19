package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/rpc"
	"net/url"
	"time"
)

type registerArgs struct {
	Key, TenantID string
}

type handler struct {
	proxy  *httputil.ReverseProxy
	client *rpc.Client
}

func newHandler(target *url.URL, client *rpc.Client) *handler {
	return &handler{
		proxy:  httputil.NewSingleHostReverseProxy(target),
		client: client,
	}
}

func (h handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	fmt.Println(req.Header)
	if tenantID, ok := req.Header["Tenant"]; ok {
		fmt.Println("Tenant:", tenantID)
		if correlationID, ok := req.Header["Correlation"]; ok {
			var reply string
			err := h.client.Call("Manager.Register", registerArgs{Key: correlationID[0], TenantID: tenantID[0]}, &reply)
			if err != nil {
				http.Error(rw, err.Error(), 500)
				return
			}
			fmt.Println("Reply from Mongo", reply)

			h.proxy.ServeHTTP(rw, req)
			err = h.client.Call("Manager.Unregister", correlationID[0], &reply)
		} else {
			http.Error(rw, "Missing Correlation", 400)
		}
	} else {
		http.Error(rw, "Missing Tenant", 400)
	}
}

func main() {
	var listenPort string
	var backendHost string
	var mongoProxyRPC string

	flag.StringVar(&listenPort, "port", "8888", "The port to listen to for incoming requests")
	flag.StringVar(&backendHost, "backend-host", "http://localhost:5000", "The backend server to forrward the requests to")
	flag.StringVar(&mongoProxyRPC, "mongo-proxy-rpc", "localhost:5557", "The MongoProxy RPC server")
	flag.Parse()

	fmt.Println("Connecting to MongoDB proxy")

	client, err := rpc.DialHTTP("tcp", mongoProxyRPC)
	for err != nil {
		log.Print(err)
		time.Sleep(time.Second * 2)
		fmt.Println("Reconnecting to MongoDB proxy...")
	}

	fmt.Println("Starting HTTP proxy")

	target, err := url.Parse(backendHost)
	if err != nil {
		log.Fatal(err)
	}

	proxy := newHandler(target, client)

	err = http.ListenAndServe(":"+listenPort, proxy)
	log.Fatal(err)
}
