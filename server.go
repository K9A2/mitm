package mitm

import (
  "encoding/base64"
  "github.com/lithammer/shortuuid"
  "io"
  "log"
  "net/http"
)

const welcomeMsg = "hi, my friend"

type welcomeMsgHandler struct{}

func (h *welcomeMsgHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  log.Println("test request received")
  w.WriteHeader(http.StatusOK)
  w.Write([]byte(welcomeMsg))
}

func ServerMain(config *Config) error {
  log.Printf("server listen at <%s>", config.Server.ListenAddr)
  handler := &remoteProxyHandler{}
  http.Handle("/proxy", handler)
  return http.ListenAndServeTLS(config.Server.ListenAddr, config.Server.CertPath,
   config.Server.KeyPath, nil)
}

// remoteProxyHandler 负责将受到的请求转发到真正的目标服务器
type remoteProxyHandler struct{}

func (rph remoteProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  requestID := shortuuid.New()

  q := r.URL.Query()
  encodedTarget := q.Get("url")
  decodedTarget, err := base64.StdEncoding.DecodeString(encodedTarget)
  if err != nil {
    log.Printf("requestID = <%s>, error in decoding target url: %s", requestID, err.Error())
    http.Error(w, "internal server error", http.StatusInternalServerError)
    return
  }

  outReq, err := http.NewRequest(r.Method, string(decodedTarget), nil)
  if err != nil {
    log.Printf("requestID = <%s>, error in creating out request: %s", requestID, err.Error())
    http.Error(w, "internal server error", http.StatusInternalServerError)
    return
  }
  outReq.Header = r.Header

  log.Printf("requestID = <%s>, encoded url = <%s>, decoded url = <%s>, accept-encoding = <%s>",
    requestID, encodedTarget, decodedTarget, outReq.Header.Get("accept-encoding"))

  h3Client := &http.Client{}

  resp, err := h3Client.Do(outReq)
  if err != nil {
    log.Printf("requestID = <%s>, error in fetching resource from source: %s", requestID, err.Error())
    http.Error(w, "error in fetching resource from source", http.StatusServiceUnavailable)
    return
  }

  copyHeader(w.Header(), resp.Header)
  written, err := io.Copy(w, resp.Body)
  if err != nil {
    log.Printf("requestID = <%s>, error in returning data: %s", requestID, r.RemoteAddr)
    http.Error(w, "server internal error", http.StatusInternalServerError)
    return
  } else {
    w.WriteHeader(http.StatusOK)
    log.Printf("requestID = <%s>, %d bytes written for %s, content-type = <%s>, content-encoding = <%s>",
      requestID, written, decodedTarget, resp.Header.Get("content-type"), resp.Header.Get("content-encoding"))
  }
}
