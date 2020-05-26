package mitm

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"time"
)

var (
	leafExtUsage = []x509.ExtKeyUsage{
		x509.ExtKeyUsageClientAuth,
		x509.ExtKeyUsageServerAuth,
	}
)

const (
	caMaxAge     = 5 * 365 * 24 * time.Hour
	leafMaxAge   = 24 * time.Hour
	leafKeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment
)

// generateTLSCert 使用根证书为指定域名生成临时 TLS 证书
func generateTLSCert(rootCA *tls.Certificate, names []string) (*tls.Certificate, error) {
	log.Printf("generating temp certs for: %v\n", names)

	if !rootCA.Leaf.IsCA {
		log.Println("given rootCA is not a root CA")
		return nil, errors.New("given rootCA is not a root CA")
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Printf("failed to generate serial number: %s\n", err)
		return nil, fmt.Errorf("failed to generate serial number: %s", err)
	}

	now := time.Now().UTC()
	certTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:         names[0],
			Organization:       names,
			OrganizationalUnit: names,
			Country:            []string{"CN"},
			Province:           []string{"GD"},
			Locality:           []string{"GD"},
		},
		NotBefore:             now,
		NotAfter:              now.Add(leafMaxAge),
		BasicConstraintsValid: true,
		IsCA:                  false,
		KeyUsage:              leafKeyUsage,
		ExtKeyUsage:           leafExtUsage,
		DNSNames:              names,
		EmailAddresses:        names,
		IPAddresses:           []net.IP{[]byte{127, 0, 0, 1}},
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Println("error in generating temp private key")
		return nil, err
	}
	tempCert, err := x509.CreateCertificate(rand.Reader, certTemplate, rootCA.Leaf, privateKey.Public(), rootCA.PrivateKey)
	if err != nil {
		log.Println("error in creating certificate")
		return nil, err
	}

	// 生成临时 TLS 证书
	tempTLSCert := new(tls.Certificate)
	tempTLSCert.Certificate = append(tempTLSCert.Certificate, tempCert)
	tempTLSCert.PrivateKey = privateKey
	tempTLSCert.Leaf, err = x509.ParseCertificate(tempCert)
	if err != nil {
		log.Println("error in creating temp tls certificate:", err.Error())
		return nil, err
	}

	return tempTLSCert, nil
}

func generatePrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

func GenCA(name string) (certPEM, keyPEM []byte, err error) {
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             now,
		NotAfter:              now.Add(caMaxAge),
		ExtKeyUsage:           leafExtUsage,
		KeyUsage:              leafKeyUsage,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            2,
	}
	key, err := generatePrivateKey()
	if err != nil {
		return
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return
	}
	keyDER := x509.MarshalPKCS1PrivateKey(key)
	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "ECDSA PRIVATE KEY",
		Bytes: keyDER,
	})
	return
}
