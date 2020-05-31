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

  //h3RoundTripper := &http3.RoundTripper{
  //  TLSClientConfig: &tls.Config{
  //    InsecureSkipVerify: true,
  //  },
  //}
  //defer h3RoundTripper.Close()

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

  //copyHeader(w.Header(), resp.Header)
  //w.Header().Del("alt-svc")
  //// 向客户端返回数据
  //w.WriteHeader(http.StatusOK)
  //
  //var reader io.ReadCloser
  //switch resp.Header.Get("content-encoding") {
  //case "gzip":
  //  reader, err = gzip.NewReader(resp.Body)
  //  if err != nil {
  //    log.Printf("requestID = <%s>, error in creating gzip reader: %s", requestID, err.Error())
  //    http.Error(w, "internal server error", http.StatusInternalServerError)
  //    return
  //  }
  //default:
  //  // 默认以未编码形式读出数据
  //  reader = resp.Body
  //}
  //// reader 总是必须关闭的
  //defer reader.Close()
  //
  //data, err := ioutil.ReadAll(reader)
  //if err != nil {
  //  log.Printf("requestID = <%s>, error in read all response body: %s", requestID, err.Error())
  //  http.Error(w, "internal server error", http.StatusInternalServerError)
  //  return
  //}
  //
  //log.Println(string(data))
  //
  ////written, err := io.Copy(w, reader)
  //written, err := gzip.NewWriter(w).Write(data)
  //if err != nil {
  //  log.Printf("requestID = <%s>, error in writting response body to client: %s",
  //    requestID, err.Error())
  //  http.Error(w, "internal server error", http.StatusInternalServerError)
  //  return
  //}
  //
  //log.Printf("requestID = <%s>, get response, written = <%d>, content-type = <%s>, content-encoding = <%s>",
  //  requestID, written, w.Header().Get("content-type"), w.Header().Get("content-encoding"))
  //return nil, nil
}

// outSendingHandler 负责实际向远程代理服务器发出请求
type outSendingHandler struct {
  RemoteProxyAddr string // 远程服务器的地址
  Scheme          string // 所代理请求的协议类型
  Host            string // 所代理请求的主机名和端口号
}

func (osh outSendingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}
