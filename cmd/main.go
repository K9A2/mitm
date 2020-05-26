package main

import (
	"crypto/tls"
	"crypto/x509"
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
	rootCA, err := getRootCA(certFilePath, keyFilePath)
	if err != nil {
		log.Fatalln("error in loading root ca certificate:", err.Error())
	}

	p := &mitm.Proxy{
		RootCA: rootCA,
		Wrap:   cloudToButt,
	}

	log.Fatal(http.ListenAndServe(":8080", p))
}

// getRootCA 从证书文件加载根证书，或者在运行时生成根证书
func getRootCA(certFilePath, keyFilePath string) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certFilePath, keyFilePath)
	if err != nil {
		log.Println()
		return nil, err
	}
	// 只使用首个证书
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	return &cert, err
}

type cloudToButtResponse struct {
	http.ResponseWriter

	sub         bool
	wroteHeader bool
}

func (w *cloudToButtResponse) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *cloudToButtResponse) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(p)
}

func cloudToButt(upstream http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("wrapping: %s\n", r.RequestURI)
		r.Header.Set("Accept-Encoding", "")
		upstream.ServeHTTP(&cloudToButtResponse{ResponseWriter: w}, r)
	})
}
