package mitm

import (
  "crypto/tls"
  "crypto/x509"
  "encoding/base64"
  "log"
  "math/rand"
  "net"
  "net/http"
  "time"
)

func init() {
  rand.Seed(time.Now().Unix())
}

func GenerateRequestID() string {
  b := make([]byte, 4)
  if _, err := rand.Read(b); err != nil {
    return ""
  }
  return base64.URLEncoding.EncodeToString(b)
}

// GetRootCA 从证书文件加载根证书，或者在运行时生成根证书
func GetRootCA(certFilePath, keyFilePath string) (*tls.Certificate, error) {
  cert, err := tls.LoadX509KeyPair(certFilePath, keyFilePath)
  if err != nil {
    log.Println()
    return nil, err
  }
  // 只使用首个证书
  cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
  return &cert, err
}

func copyHeader(dst, src http.Header) {
  for k, vv := range src {
    for _, v := range vv {
      dst.Add(k, v)
    }
  }
}

// 向下游返回此 HTTP 消息以表明建立了与上游的连接
var connectedResponse = []byte("HTTP/1.1 200 OK\r\n\r\n")

// handshake hijacks w's underlying net.Conn, responds to the CONNECT request
// and manually performs the TLS handshake. It returns the net.Conn or and
// error if any.
func handshake(w http.ResponseWriter, config *tls.Config) (net.Conn, error) {
  // 获取下游 TCP 链接
  downstreamTCPConn, _, err := w.(http.Hijacker).Hijack()
  if err != nil {
    http.Error(w, "can not get downstream TCP connection", http.StatusServiceUnavailable)
    return nil, err
  }
  // 发送连接成功的 HTTP 消息
  if _, err = downstreamTCPConn.Write(connectedResponse); err != nil {
    downstreamTCPConn.Close()
    return nil, err
  }
  // 在 TCP 连接上建立 TLS 连接
  downstreamTLSConn := tls.Server(downstreamTCPConn, config)
  err = downstreamTLSConn.Handshake() // 手动进行 TLS 握手流程
  if err != nil {
    log.Println("error in tls handshake with", downstreamTLSConn.RemoteAddr())
    downstreamTLSConn.Close()
    downstreamTCPConn.Close()
    return nil, err
  }
  return downstreamTLSConn, nil
}

// getHostName returns the DNS name in addr, if any.
func getHostName(addr string) string {
  hostname, _, err := net.SplitHostPort(addr)
  if err != nil {
    return ""
  }
  return hostname
}
