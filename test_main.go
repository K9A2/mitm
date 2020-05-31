package mitm

import (
  "crypto/tls"
  "encoding/base64"
  "fmt"
  "github.com/stormlin/mitm/quic/http3"
  "io/ioutil"
  "log"
  "net/http"
)

// 测试远程代理服务器是否能获取目标资源
func TestMain(config *Config, targetURL string) error {
  roundTripper := &http3.RoundTripper{
    TLSClientConfig: &tls.Config{
      InsecureSkipVerify: true,
    },
  }
  defer roundTripper.Close()

  h3Client := &http.Client{
    //Transport: roundTripper,
    Transport: &http.Transport{
      TLSClientConfig: &tls.Config{
        InsecureSkipVerify: true,
      },
    },
  }

  encodedTarget := base64.StdEncoding.EncodeToString([]byte(targetURL))
  urlString := fmt.Sprintf("https://%s/proxy?url=%s",
    config.Client.RemoteProxyAddr, encodedTarget)
  log.Printf("send as: <%s>", urlString)
  resp, err := h3Client.Get(urlString)
  if err != nil {
    return err
  }
  data, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    return err
  }
  log.Println("status:", resp.Status)
  log.Println(string(data))
  return nil
}
