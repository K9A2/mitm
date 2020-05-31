package mitm

const (
  // 程序运行目的
  RoleServer = "server"
  RoleClient = "client"
  RoleTest   = "test"

  // 客户端默认配置项
  ClientListenAddr = ":8080"
  RemoteProxyAddr  = "localhost"
  ClientCertPath   = "proxy-client.crt"
  ClientKeyPath    = "proxy-client.key"

  // 服务端默认配置项
  ListenAddr     = ":443"
  ServerCertPath = "proxy-server.crt"
  ServerKeyPath  = "proxy-server.key"

  // 测试客户端配置项
  DefaultTarget = "https://www.baidu.com"

  // 默认请求协议类型
  DefaultScheme = "https"
)
