package mitm

import (
  "crypto/tls"
  "encoding/base64"
  "fmt"
  "github.com/lithammer/shortuuid"
  "log"
  "net/http"
)

// hybridRoundTripper 负责按照按照给定的调度策略将受到的请求分别用 H2 或者 H3 发送到对端
type hybridRoundTripper struct {
  RemoteProxyAddr string
  Scheme          string
  Host            string
}

func (hr *hybridRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
  // 分配一个唯一的请求 ID 号
  requestID := shortuuid.New()

  // RequestURI 中包含查询串，故无需根据是否含有查询串生成不同的 URL 字符串
  originalTarget := fmt.Sprintf("%s://%s%s", hr.Scheme, hr.Host, r.URL.RequestURI())
  encodedTarget := base64.StdEncoding.EncodeToString([]byte(originalTarget))

  // 构造发现远程代理服务器的请求
  outURL := fmt.Sprintf("%s://%s/proxy?url=%s", "https", hr.RemoteProxyAddr, encodedTarget)
  outReq, err := http.NewRequest(r.Method, outURL, nil)
  if err != nil {
    log.Printf("requestID = <%s>, error in creating out request: %s", requestID, err.Error())
    return nil, err
  }
  outReq.Header = r.Header

  log.Printf("requestID = <%s>, method = <%s>, scheme = <%s>, host = <%s>, requestURI = <%s>, query = <%s>, outURL = <%s>",
    requestID, r.Method, hr.Scheme, hr.Host, r.URL.RequestURI(), r.URL.RawQuery, outURL)

  // 向远程代理服务器发送请求
  h3Client := &http.Client{
    //Transport: h3RoundTripper,
    Transport: &http.Transport{
      TLSClientConfig: &tls.Config{
        InsecureSkipVerify: true,
      },
    },
  }
  resp, err := h3Client.Do(outReq)
  if err != nil {
    log.Printf("requestID = <%s>, error in getting response from remote proxy server: %s",
      requestID, err.Error())
    return nil, err
  }

  log.Printf("requestID = <%s>, go response: status = <%s>", requestID, resp.Status)

  return resp, nil
}
