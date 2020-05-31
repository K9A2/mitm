package mitm

import (
  //"context"
  "crypto/tls"
  "github.com/lithammer/shortuuid"
  "log"
  "net"
  "net/http"
  //"net/http/httputil"
  "time"
)

var store *CertificateStore

func init() {
  // 全局初始化临时证书库
  if store == nil {
    store = &CertificateStore{}
  }
}

// Proxy is a forward proxy that substitutes its own certificate
// for incoming TLS connections in place of the upstream server's
// certificate.
type Proxy struct {
  // Wrap specifies a function for optionally wrapping upstream for
  // inspecting the decrypted HTTP request and response.
  Wrap func(upstream http.Handler) http.Handler

  // RootCA specifies the root RootCA for generating leaf certs for each incoming
  // TLS request.
  RootCA *tls.Certificate

  // TLSServerConfig specifies the tls.Config to use when generating leaf
  // cert using RootCA.
  // 用这张证书加密返回下游的数据
  TLSServerConfig *tls.Config

  // TLSClientConfig specifies the tls.Config to use when establishing
  // an upstream connection for proxying.
  // 用这张证书代替客户端的证书，并加密发送给上游的数据
  TLSClientConfig *tls.Config

  // FlushInterval specifies the flush interval
  // to flush to the client while copying the
  // response body.
  // If zero, no periodic flushing is done.
  FlushInterval time.Duration

  RemoteProxyAddr string
}

// ServeHTTP 处理客户端的代理请求
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  if r.Method == "CONNECT" {
    // 处理 https 请求
    p.serveConnect(w, r)
    return
  }
}

// serveConnect 处理客户端的 https 代理请求。从 w 中可以提取底层的 TCP 连接。
//   以 proxy 为中心命名 TLS 连接：
//   客户端 <- 下行链路 <- 本地代理 -> 上行链路 -> 远程代理 -> 目标服务器
func (p *Proxy) serveConnect(w http.ResponseWriter, r *http.Request) {
  var err error
  // 为每一个请求生成一个 uuid 以便于追踪调试
  requestID := shortuuid.New()

  // 上游服务器的主机名
  var upstreamHostName = getHostName(r.Host)
  if upstreamHostName == "" {
    // 无法获取上游服务器的主机名
    log.Println(requestID, "error in getting upstream hostname:", r.Host)
    http.Error(w, "no upstream", http.StatusBadRequest)
    return
  }
  // 生成与上游通信的 TLS 证书
  //upstreamTLSConfig := &tls.Config{
  //  ServerName: upstreamHostName,
  //}
  // 建立与上游主机的 TLS 连接
  //upstreamTLSConn, err := tls.Dial("tcp", r.Host, upstreamTLSConfig)
  //if err != nil {
  //	log.Println(requestID, "error in dialing upstream host:", r.Host, err)
  //	http.Error(w, "no upstream", http.StatusServiceUnavailable)
  //	return
  //}
  //defer upstreamTLSConn.Close()

  // 用配置的根证书和上游服务器的主机名生成新证书，用于加密返回下游的数据
  var downstreamTLSCert *tls.Certificate
  downstreamTLSCert, err = p.cert(upstreamHostName) // 生成临时证书
  if err != nil {
    http.Error(w, "no upstream", http.StatusInternalServerError)
    log.Println(requestID, "cert error:", upstreamHostName, err)
    return
  }
  downstreamTLSConfig := &tls.Config{
    Certificates: []tls.Certificate{*downstreamTLSCert},
  }
  // 使用临时证书完成与下游的 TLS 握手流程
  downstreamTLSConn, err := handshake(w, downstreamTLSConfig)
  if err != nil {
    log.Println("handshake", r.Host, err)
    return
  }
  defer downstreamTLSConn.Close()

  //h2Transport := http.Transport{
  //  DialTLSContext: func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
  //    upstreamTLSConn, err := tls.Dial("tcp", remoteProxyHostname, upstreamTLSConfig)
  //    if err != nil {
  //      return nil, err
  //    }
  //    return upstreamTLSConn, nil
  //  },
  //  ForceAttemptHTTP2: true,
  //}

  //switcher := ProtocolSwitcher{}
  //reverseProxy := &httputil.ReverseProxy{
  //Director: remoteProxyDirector, // 设置要外发的请求的类型和主机名
  //// todo: 暂时用 H2 连接进行测试
  //Transport: &h2Transport,
  //}

  localProxy := localProxyHandler{}

  onCloseChan := make(chan int)
  wc := &onCloseConn{downstreamTLSConn, func() {
    // 完成传输后触发结束事件
    onCloseChan <- 0
  }}
  // 设置一个临时服务器以监听来自下游连接的请求，并由反向代理执行。
  // 在处理完一个请求后会退出此临时服务器。
  err = http.Serve(&oneShotListener{wc}, localProxy)
  if err != nil {
    http.Error(w, "internal server error", http.StatusInternalServerError)
  }
  <-onCloseChan
}

func (p *Proxy) cert(names ...string) (*tls.Certificate, error) {
  return generateTLSCert(p.RootCA, names)
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
