# Regatta Go client
[![GoDoc](https://godoc.org/github.com/jamf/regatta-go?status.svg)](https://godoc.org/github.com/jamf/regatta-go)
[![tag](https://img.shields.io/github/tag/jamf/regatta-go.svg)](https://github.com/jamf/regatta-go/releases)
![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.20-%23007d9c)
![Build Status](https://github.com/jamf/regatta-go/actions/workflows/test.yml/badge.svg)
[![Coverage Status](https://coveralls.io/repos/github/jamf/regatta-go/badge.svg?branch=main)](https://coveralls.io/github/jamf/regatta-go?branch=main)
[![Go report](https://goreportcard.com/badge/github.com/jamf/regatta-go)](https://goreportcard.com/report/github.com/jamf/regatta-go)
[![Contributors](https://img.shields.io/github/contributors/jamf/regatta-go)](https://github.com/jamf/regatta-go/graphs/contributors)
[![License](https://img.shields.io/github/license/jamf/regatta-go)](LICENSE)

This repository hosts the code of **Regatta** client for Go language. For documentation and examples check the [godocs](https://godoc.org/github.com/jamf/regatta-go) page.

## Example use

```go
package main

import (
	"context"
	"fmt"
	"time"

	client "github.com/jamf/regatta-go"
)

func main() {
	// Create the configuration
	cc, err := client.NewClientConfig(&client.ConfigSpec{
		Logger:      client.PrintLogger{}, // Logger override
		Endpoints:   []string{"127.0.0.1:8443"}, // Seed of Regatta servers (other server will be discovered during initial connection)
		DialTimeout: 5 * time.Second, // Dial timeout
		Secure: &client.SecureConfig{
			InsecureSkipVerify: true, // Skip verification of self-signed certificate
		},
	})
	if err != nil {
		panic(err)
	}
	c, err := client.New(cc) // Create the client out of the provided config
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Provide operation timeout
	defer cancel()
	
	put, err := c.Table("regatta-test").Put(ctx, "foo", "bar")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", put)
}
```

## Regatta Documentation

For guidance on installation, deployment, and administration,
see the [documentation page](https://engineering.jamf.com/regatta).

## Contributing

Regatta is in active development and contributors are welcome! For guidance on development, see the page
[Contributing](https://engineering.jamf.com/regatta/contributing).
Feel free to ask questions and engage in [GitHub Discussions](https://github.com/jamf/regatta/discussions)!
