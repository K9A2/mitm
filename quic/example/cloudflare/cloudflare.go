package main

import (
	"crypto/md5"
	"crypto/tls"
	"github.com/stormlin/mitm/quic/http3"
	"github.com/stormlin/mitm/quic/internal/testdata"
	"io"
	"log"
	"net/http"
	"time"
)

const targetURL = "https://www.youtube.com/yts/jsbin/desktop_polymer_inlined_html_polymer_flags_v2-vfly6dPky/desktop_polymer_inlined_html_polymer_flags_v2.js"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	pool := testdata.GetRootCA()
	testdata.AddRootCA(pool)

	roundTripper := &http3.RoundTripper{
		TLSClientConfig: &tls.Config{
			RootCAs:            pool,
			InsecureSkipVerify: true,
		},
	}
	defer roundTripper.Close()

	//var timeSum int64
	for i := 0; i < 1; i++ {
		timeStart := time.Now()

		h3Client := &http.Client{
			//Transport: roundTripper,
		}
		resp, err := h3Client.Get(targetURL)
		if err != nil {
			log.Printf("error in getting target resource: %s\n", err.Error())
			resp.Body.Close()
			return
		}
		h := md5.New()
		io.Copy(h, resp.Body)
		resp.Body.Close()

		timeEnd := time.Now()
		//timeSum += timeEnd.Sub(timeStart).Nanoseconds()
		log.Printf("consumed time: %d, content-length: %v, md5: %x\n",
			timeEnd.Sub(timeStart).Milliseconds(), resp.ContentLength, h.Sum(nil))
	}

	//log.Printf("average time: %f\n", float64(timeSum)/1/10e6)
}
