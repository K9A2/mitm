package main

import (
	"github.com/stormlin/mitm"
	"log"
	"net/http"
)

const (
	certFilePath = "server.crt"
	keyFilePath  = "server.key"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 读取根证书
	rootCA, err := mitm.GetRootCA(certFilePath, keyFilePath)
	if err != nil {
		log.Fatalln("error in loading root ca certificate:", err.Error())
	}

	p := &mitm.Proxy{
		RootCA: rootCA,
	}

	log.Fatal(http.ListenAndServe(":8080", p))
}
