package mitm

import (
  "crypto/tls"
  "crypto/x509"
  "encoding/base64"
  "log"
  "math/rand"
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
