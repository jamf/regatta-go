// Copyright JAMF Software, LLC

package test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type RegattaVersion string

const (
	Regatta_v0_2_1 RegattaVersion = "v0.2.1"
	Regatta_master RegattaVersion = "master"
)

type testOptions struct {
	tables []string
}

func defaultOptions() *testOptions {
	return &testOptions{tables: []string{"test"}}
}

func WithTable(name string) RegattaOption {
	return func(options *testOptions) {
		options.tables = append(options.tables, name)
	}
}

type RegattaOption func(options *testOptions)

func testWithRegatta(t *testing.T, version RegattaVersion, option ...RegattaOption) (string, error) {
	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	t.Cleanup(func() {
		os.Unsetenv("TESTCONTAINERS_RYUK_DISABLED")
	})
	testcontainers.SkipIfProviderIsNotHealthy(t)

	opts := defaultOptions()
	for _, ro := range option {
		ro(opts)
	}

	ctx := context.Background()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return "", err
	}
	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	keyPath := filepath.Join(t.TempDir(), "server.key")
	err = os.WriteFile(keyPath, keyPem, 0o777)
	if err != nil {
		return "", err
	}
	tml := x509.Certificate{
		// you can add any attr that you need
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(5, 0, 0),
		// you have to generate a different serial number each execution
		SerialNumber: big.NewInt(123123),
		Subject: pkix.Name{
			CommonName:   "New Name",
			Organization: []string{"New Org."},
		},
		BasicConstraintsValid: true,
	}
	cert, err := x509.CreateCertificate(rand.Reader, &tml, &tml, &key.PublicKey, key)
	if err != nil {
		return "", err
	}
	certPath := filepath.Join(t.TempDir(), "server.crt")
	certPem := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	})
	err = os.WriteFile(certPath, certPem, 0o777)
	if err != nil {
		return "", err
	}
	regattaC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        fmt.Sprintf("ghcr.io/jamf/regatta:%s", version),
			ExposedPorts: []string{"8443/tcp"},
			Files: []testcontainers.ContainerFile{
				{
					ContainerFilePath: "/hack/server.crt",
					HostFilePath:      certPath,
				},
				{
					ContainerFilePath: "/hack/server.key",
					HostFilePath:      keyPath,
				},
			},
			Cmd: []string{
				"leader",
				"--dev-mode",
				"--api.reflection-api",
				"--raft.address=127.0.0.1:5012",
				"--raft.initial-members=1=127.0.0.1:5012",
				"--replication.enabled=false",
				"--maintenance.enabled=false",
				fmt.Sprintf("--tables.names=%s", strings.Join(opts.tables, ",")),
			},
			WaitingFor: wait.ForExposedPort(),
		},
		Started: true,
	})
	if err != nil {
		return "", err
	}

	t.Cleanup(func() {
		if regattaC != nil {
			if err := regattaC.Terminate(ctx); err != nil {
				t.Fatalf("failed to terminate container: %s", err.Error())
			}
		}
	})
	return regattaC.Endpoint(ctx, "")
}
