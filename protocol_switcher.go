package mitm

import "net/http"

// ProtocolSwitcher 是混合了 H2 和 H3 两种协议的调度器。
// 用户通过配置 HostUseH3 来声明需要走 H3 代理的目标主机，调度器会同时使用若干条 H3 连接以
// 同时下载目标资源。对于不在 HostUseH3 中的主机，则使用 H2 代理。
type ProtocolSwitcher struct {
	// 对其中的主机使用 H3 代理
	HostUseH3 map[string]struct{}
}

func (switcher ProtocolSwitcher) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, nil
}
