package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"path/filepath"
)

var (
	serverKeyFile string
	serverCrtFile string
	demoKeyPair   *tls.Certificate
	demoCertPool  *x509.CertPool
)

func initCerts() error {
	serverKeyFile = filepath.Join(srvCfg.cfgDir, "certs", "tls.key")
	serverKey, err := ioutil.ReadFile(serverKeyFile)
	if err != nil {
		return err
	}

	serverCrtFile = filepath.Join(srvCfg.cfgDir, "certs", "tls.crt")
	serverCrt, err := ioutil.ReadFile(serverCrtFile)
	if err != nil {
		return err
	}

	pair, err := tls.X509KeyPair(serverCrt, serverKey)
	if err != nil {
		panic(err)
	}
	demoKeyPair = &pair
	demoCertPool = x509.NewCertPool()
	ok := demoCertPool.AppendCertsFromPEM(serverCrt)
	if !ok {
		panic("bad certs")
	}
	return nil
}
