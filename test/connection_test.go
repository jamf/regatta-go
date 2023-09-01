// Copyright JAMF Software, LLC

package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/jamf/regatta-go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/connectivity"
)

func Test_ClientConfig(t *testing.T) {
	type args struct {
		config *client.ConfigSpec
	}
	tests := []struct {
		name    string
		version RegattaVersion
		args    args
	}{
		{
			name:    "insecure",
			version: Regatta_v0_2_1,
			args: args{
				config: &client.ConfigSpec{
					Logger:      client.PrintLogger{},
					Endpoints:   []string{"127.0.0.1:8443"},
					DialTimeout: 5 * time.Second,
					Secure: &client.SecureConfig{
						InsecureSkipVerify: true,
					},
				},
			},
		},
		{
			name:    "insecure",
			version: Regatta_master,
			args: args{
				config: &client.ConfigSpec{
					Logger:      client.PrintLogger{},
					Endpoints:   []string{"127.0.0.1:8443"},
					DialTimeout: 5 * time.Second,
					Secure: &client.SecureConfig{
						InsecureSkipVerify: true,
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s_%s", test.version, test.name), func(t *testing.T) {
			endpoint, err := testWithRegatta(t, test.version)
			require.NoError(t, err)
			test.args.config.Endpoints = []string{endpoint}
			cc, err := client.NewClientConfig(test.args.config)
			require.NoError(t, err)
			c, err := client.New(cc)
			require.NoError(t, err)
			defer c.Close()
			require.Eventually(t, func() bool {
				return c.ActiveConnection().GetState() == connectivity.Ready
			}, 10*time.Second, 10*time.Millisecond)

		})
	}
}
