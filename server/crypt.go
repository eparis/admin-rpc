package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

var (
	demoKeyPair *tls.Certificate
	caCertPool  *x509.CertPool
)

func initCerts() error {
	serverKeyFile := filepath.Join(srvCfg.cfgDir, "certs", "tls.key")
	serverKey, err := ioutil.ReadFile(serverKeyFile)
	if err != nil {
		return err
	}

	serverCrtFile := filepath.Join(srvCfg.cfgDir, "certs", "tls.crt")
	serverCrt, err := ioutil.ReadFile(serverCrtFile)
	if err != nil {
		return err
	}

	pair, err := tls.X509KeyPair(serverCrt, serverKey)
	if err != nil {
		return (err)
	}
	demoKeyPair = &pair

	// Load the System CA
	caCertPool, err = x509.SystemCertPool()
	if err != nil {
		return err
	}

	// The CA.crt in the config/certs/ directory
	caCrtFile := filepath.Join(srvCfg.cfgDir, "certs", "CA.crt")
	caCrt, err := ioutil.ReadFile(caCrtFile)
	if err == nil {
		ok := caCertPool.AppendCertsFromPEM(caCrt)
		if !ok {
			return fmt.Errorf("bad certs")
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	// And the CA provided by openshift for services
	caCrtFile = "/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt"
	caCrt, err = ioutil.ReadFile(caCrtFile)
	if err == nil {
		ok := caCertPool.AppendCertsFromPEM(caCrt)
		if !ok {
			return fmt.Errorf("bad certs")
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	return nil
}
