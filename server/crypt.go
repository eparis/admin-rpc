package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"path/filepath"
)

var (
	demoKeyPair  *tls.Certificate
	demoCertPool *x509.CertPool
)

func initCerts() error {
	serverKeyFile := filepath.Join(srvCfg.cfgDir, "server.key")
	serverKey, err := ioutil.ReadFile(serverKeyFile)
	if err != nil {
		return err
	}

	serverPemFile := filepath.Join(srvCfg.cfgDir, "server.pem")
	serverPem, err := ioutil.ReadFile(serverPemFile)
	if err != nil {
		return err
	}

	pair, err := tls.X509KeyPair(serverPem, serverKey)
	if err != nil {
		panic(err)
	}
	demoKeyPair = &pair
	demoCertPool = x509.NewCertPool()
	ok := demoCertPool.AppendCertsFromPEM(serverPem)
	if !ok {
		panic("bad certs")
	}
	return nil
}
