package mitm

import (
	"crypto/tls"
	"sync"
)

const (
	StatusNonexistent = 1 // 尚未存入该主机的 TLS 证书
	StatusCreating    = 2 // 调用者正在生成该主机的 TLS 证书
	StatusSet         = 3 // 已经存入该主机的 TLS 证书
)

type CertificateStore struct {
	sync.Mutex

	// 以上游的主机名为键存放用于替换的 TLS 证书
	store map[string]*tls.Certificate
	status map[string]*tls.Certificate
}

// Get 获取目标主机的 TLS 证书
// 当目标主机的 TLS 证书已存在时，返回其 TLS 证书和 true
// 当目标主机的 TLS 证书不存在时，返回 nil 和 false
func (s *CertificateStore) Get(hostname string) (*tls.Certificate, bool) {
	s.Lock()
	defer s.Unlock()
	certificate, ok := s.store[hostname]
	return certificate, ok
}

// Put 存入目标主机的 TLS 证书
func (s *CertificateStore) Put(hostname string, certificate *tls.Certificate) bool {
	s.Lock()
	defer s.Unlock()
	_, ok := s.store[hostname]
	if ok {
		// 已有对应名字的 TLS 证书
		return false
	}
	s.store[hostname] = certificate
	return true
}
