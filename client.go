package mitm

import (
  "crypto/tls"
  "errors"
  "log"
  "net"
  "net/http"
  "net/http/httputil"
)

const defaultMessage = "welcome my friend"

var (
  errNoAvailableConn = errors.New("no available conn")
)

// ClientMain 作为在以代理客户端形式运行时的主程序
func ClientMain(config *Config) error {
  rootCA, err := GetRootCA(config.Client.CertPath, config.Client.KeyPath) // 读取指定的根证书
  if err != nil {
    return errors.New("error in loading root ca certificate: " + err.Error())
  }

  localProxy := localProxyHandler{
    RemoteProxyAddr: config.Client.RemoteProxyAddr,
    RootCA:          rootCA,
  }

  log.Printf("client listen at <%s>", config.Client.ListenAddr)
  // 在给定端口上监听客户端请求
  return http.ListenAndServe(config.Client.ListenAddr, localProxy)
}

// localProxyHandler 负责将客户端的代理请求转发至远程代理服务器
type localProxyHandler struct {
  RemoteProxyAddr string           // 远程服务器地址，所有 HTTP 请求的 Host 字段都会被设置为此地址
  RootCA          *tls.Certificate // 用于加密下行链路数据流的证书
}

// httpsDirector 将给定的请求指向远程代理服务器
func (lph localProxyHandler) httpsDirector(r *http.Request) {
  //r.Host = lph.RemoteProxyAddr
  //r.URL.Scheme = DefaultScheme
}

func (lph localProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  var targetHostname = getHostName(r.Host) // 获取目标主机名
  if targetHostname == "" {
    log.Println("error in getting target hostname:", r.Host)
    http.Error(w, "bad request", http.StatusBadRequest)
    return
  }

  // 用于加密发送给客户的数据流的 TLS 证书
  downstreamTLSCert, err := generateTLSCert(lph.RootCA, []string{targetHostname})
  if err != nil {
    log.Println("error in creating downstream TLS cert:", err.Error())
    http.Error(w, "internal server error", http.StatusInternalServerError)
    return
  }
  downstreamTLSConfig := &tls.Config{
    Certificates: []tls.Certificate{*downstreamTLSCert},
  }
  downstreamTLSConn, err := handshake(w, downstreamTLSConfig)
  if err != nil {
    log.Println("error in performing handshake with client:", err.Error())
    http.Error(w, "internal server error", http.StatusInternalServerError)
    return
  }
  defer downstreamTLSConn.Close()

  rp := &httputil.ReverseProxy{
    Director: lph.httpsDirector,
    Transport: &hybridRoundTripper{
      RemoteProxyAddr: lph.RemoteProxyAddr,
      Scheme:          DefaultScheme,
      Host:            r.Host,
    },
  }

  //osh := outSendingHandler{
  //  RemoteProxyAddr: lph.RemoteProxyAddr,
  //  Scheme:          DefaultScheme,
  //  Host:            r.Host,
  //}
  onCloseChan := make(chan int)
  wc := &onCloseConn{downstreamTLSConn, func() {
    // 完成传输后触发结束事件
    onCloseChan <- 0
  }}
  // 设置一个临时服务器以监听来自下游连接的请求，并由反向代理执行。
  // 在处理完一个请求后会退出此临时服务器。
  err = http.Serve(&oneShotListener{wc}, rp)
  if err != nil && err != errNoAvailableConn {
    log.Println("error in serving incoming request:", err.Error())
    return
  }
  <-onCloseChan
  log.Println("closed")
}

// oneShotListener 负责监听来自客户端的请求
type oneShotListener struct {
  conn net.Conn
}

// Accept 接受来自客户端的请求
func (l *oneShotListener) Accept() (net.Conn, error) {
  if l.conn == nil {
    return nil, errNoAvailableConn
  }
  // 只在这条连接上接受一个请求，避免因无限接受连接而出现内存泄露问题
  conn := l.conn
  l.conn = nil
  return conn, nil
}

// Close 关闭与客户端的连接
func (l *oneShotListener) Close() error {
  // 不做任何操作，在 Proxy 的 ServeHTTP 方法中关闭
  return nil
}

// Addr 返回 listener 实例所监听的地址
func (l *oneShotListener) Addr() net.Addr {
  return l.conn.LocalAddr()
}

// A onCloseConn implements net.Conn and calls its f on Close.
type onCloseConn struct {
  net.Conn
  f func()
}

func (c *onCloseConn) Close() error {
  if c.f != nil {
    c.f()
    c.f = nil
  }
  return c.Conn.Close()
}
