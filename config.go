package mitm

import (
  "github.com/jessevdk/go-flags"
  "gopkg.in/yaml.v3"
  "io/ioutil"
  "os"
)

// 命令行参数定义
type Options struct {
  Role       string `short:"r" long:"role" description:"set the role of this program" choice:"test" choice:"client" choice:"server"`
  ConfigPath string `short:"c" long:"config" description:"set the path to YAML config path"`
  URL        string `short:"u" long:"url" description:"set the url of a test request"`
}

// ParseCommandlineOptions 解析并返回命令行参数
func ParseCommandlineOptions(args []string) (*Options, error) {
  var options Options
  args, err := flags.ParseArgs(&options, args)
  if err != nil {
    return nil, err
  }
  return &options, nil
}

type ClientConfig struct {
  ListenAddr      string `yaml:"listenAddr"`
  RemoteProxyAddr string `yaml:"remoteProxyAddr"`
  CertPath        string `yaml:"certPath"`
  KeyPath         string `yaml:"keyPath"`
}

type ServerConfig struct {
  ListenAddr string `yaml:"listenAddr"`
  CertPath   string `yaml:"certPath"`
  KeyPath    string `yaml:"keyPath"`
}

type TestConfig struct {
  Target string `yaml:"target"`
}

type Config struct {
  Client     *ClientConfig
  Server     *ServerConfig
  TestConfig *TestConfig
}

func getDefaultConfig() *Config {
  config := &Config{
    Client: &ClientConfig{
      ListenAddr:      ListenAddr,
      RemoteProxyAddr: RemoteProxyAddr,
      CertPath:        ClientCertPath,
      KeyPath:         ClientKeyPath,
    },
    Server: &ServerConfig{
      ListenAddr: ListenAddr,
      CertPath:   ServerCertPath,
      KeyPath:    ServerKeyPath,
    },
    TestConfig: &TestConfig{
      Target: DefaultTarget,
    },
  }
  return config
}

// LoadConfig 加载指定的配置文件，没有声明的配置项会被设为默认值
func LoadConfig(configPath string) (*Config, error) {
  config := getDefaultConfig()

  configFile, err := os.Open(configPath)
  if err != nil {
    return nil, err
  }
  defer configFile.Close()

  content, err := ioutil.ReadAll(configFile)
  if err != nil {
    return nil, err
  }
  if err = yaml.Unmarshal(content, &config); err != nil {
    return nil, err
  }

  return config, nil
}
