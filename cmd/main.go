package main

import (
  "github.com/stormlin/mitm"
  "log"
  "os"
)

func main() {
  // 配置程序的默认全局 logger
  log.SetFlags(log.LstdFlags | log.Lshortfile)

  // 解析命令行参数
  options, err := mitm.ParseCommandlineOptions(os.Args)
  if err != nil {
    log.Fatalln("error in parsing command line options:", err.Error())
  }
  config, err := mitm.LoadConfig(options.ConfigPath)
  if err != nil {
    log.Fatalln("error in loading config file:", err.Error())
  }

  switch options.Role {
  case mitm.RoleClient:
    if err := mitm.ClientMain(config); err != nil {
      log.Fatalln("error in client main:", err.Error())
    }
  case mitm.RoleServer:
    if err := mitm.ServerMain(config); err != nil {
      log.Fatalln("error in server main:", err.Error())
    }
  case mitm.RoleTest:
    if err := mitm.TestMain(config, options.URL); err != nil {
      log.Fatalln("error in test main:", err.Error())
    }
  default:
    log.Fatalln("incorrect program role:", options.Role)
  }
}
