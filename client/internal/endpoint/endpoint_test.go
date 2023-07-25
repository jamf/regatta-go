// Copyright JAMF Software, LLC

package endpoint

import (
	"testing"
)

func Test_interpret(t *testing.T) {
	tests := []struct {
		endpoint          string
		wantAddress       string
		wantServerName    string
		wantRequiresCreds CredsRequirement
	}{
		{endpoint: "127.0.0.1", wantAddress: "127.0.0.1", wantServerName: "127.0.0.1", wantRequiresCreds: CredsOptional},
		{endpoint: "localhost", wantAddress: "localhost", wantServerName: "localhost", wantRequiresCreds: CredsOptional},
		{endpoint: "localhost:8080", wantAddress: "localhost:8080", wantServerName: "localhost", wantRequiresCreds: CredsOptional},

		{endpoint: "unix:127.0.0.1", wantAddress: "unix:127.0.0.1", wantServerName: "127.0.0.1", wantRequiresCreds: CredsOptional},
		{endpoint: "unix:127.0.0.1:8080", wantAddress: "unix:127.0.0.1:8080", wantServerName: "127.0.0.1", wantRequiresCreds: CredsOptional},

		{endpoint: "unix://127.0.0.1", wantAddress: "unix:127.0.0.1", wantServerName: "127.0.0.1", wantRequiresCreds: CredsOptional},
		{endpoint: "unix://127.0.0.1:8080", wantAddress: "unix:127.0.0.1:8080", wantServerName: "127.0.0.1", wantRequiresCreds: CredsOptional},

		{endpoint: "unixs:127.0.0.1", wantAddress: "unix:127.0.0.1", wantServerName: "127.0.0.1", wantRequiresCreds: CredsRequire},
		{endpoint: "unixs:127.0.0.1:8080", wantAddress: "unix:127.0.0.1:8080", wantServerName: "127.0.0.1", wantRequiresCreds: CredsRequire},
		{endpoint: "unixs://127.0.0.1", wantAddress: "unix:127.0.0.1", wantServerName: "127.0.0.1", wantRequiresCreds: CredsRequire},
		{endpoint: "unixs://127.0.0.1:8080", wantAddress: "unix:127.0.0.1:8080", wantServerName: "127.0.0.1", wantRequiresCreds: CredsRequire},

		{endpoint: "http://127.0.0.1", wantAddress: "127.0.0.1", wantServerName: "127.0.0.1", wantRequiresCreds: CredsDrop},
		{endpoint: "http://127.0.0.1:8080", wantAddress: "127.0.0.1:8080", wantServerName: "127.0.0.1", wantRequiresCreds: CredsDrop},
		{endpoint: "https://127.0.0.1", wantAddress: "127.0.0.1", wantServerName: "127.0.0.1", wantRequiresCreds: CredsRequire},
		{endpoint: "https://127.0.0.1:8080", wantAddress: "127.0.0.1:8080", wantServerName: "127.0.0.1", wantRequiresCreds: CredsRequire},
		{endpoint: "https://localhost:20000", wantAddress: "localhost:20000", wantServerName: "localhost", wantRequiresCreds: CredsRequire},

		{endpoint: "unix:///tmp/abc", wantAddress: "unix:///tmp/abc", wantServerName: "abc", wantRequiresCreds: CredsOptional},
		{endpoint: "unixs:///tmp/abc", wantAddress: "unix:///tmp/abc", wantServerName: "abc", wantRequiresCreds: CredsRequire},
		{endpoint: "unix:///tmp/abc:1234", wantAddress: "unix:///tmp/abc:1234", wantServerName: "abc", wantRequiresCreds: CredsOptional},
		{endpoint: "unixs:///tmp/abc:1234", wantAddress: "unix:///tmp/abc:1234", wantServerName: "abc", wantRequiresCreds: CredsRequire},
		{endpoint: "regatta.io", wantAddress: "regatta.io", wantServerName: "regatta.io", wantRequiresCreds: CredsOptional},
		{endpoint: "http://regatta.io/abc", wantAddress: "regatta.io", wantServerName: "regatta.io", wantRequiresCreds: CredsDrop},
		{endpoint: "dns://something-other", wantAddress: "dns://something-other", wantServerName: "something-other", wantRequiresCreds: CredsOptional},

		{endpoint: "http://[2001:db8:1f70::999:de8:7648:6e8]:100/", wantAddress: "[2001:db8:1f70::999:de8:7648:6e8]:100", wantServerName: "2001:db8:1f70::999:de8:7648:6e8", wantRequiresCreds: CredsDrop},
		{endpoint: "[2001:db8:1f70::999:de8:7648:6e8]:100", wantAddress: "[2001:db8:1f70::999:de8:7648:6e8]:100", wantServerName: "2001:db8:1f70::999:de8:7648:6e8", wantRequiresCreds: CredsOptional},
		{endpoint: "unix:unexpected-file_name#123$456", wantAddress: "unix:unexpected-file_name#123$456", wantServerName: "unexpected-file_name#123$456", wantRequiresCreds: CredsOptional},
	}
	for _, tt := range tests {
		t.Run("interpret "+tt.endpoint, func(t *testing.T) {
			gotAddress, gotServerName := Interpret(tt.endpoint)
			if gotAddress != tt.wantAddress {
				t.Errorf("Interpret() gotAddress = %v, want %v", gotAddress, tt.wantAddress)
			}
			if gotServerName != tt.wantServerName {
				t.Errorf("Interpret() gotServerName = %v, want %v", gotServerName, tt.wantServerName)
			}
		})
		t.Run("requires credentials "+tt.endpoint, func(t *testing.T) {
			requiresCreds := RequiresCredentials(tt.endpoint)
			if requiresCreds != tt.wantRequiresCreds {
				t.Errorf("RequiresCredentials() got = %v, want %v", requiresCreds, tt.wantRequiresCreds)
			}
		})
	}
}

func Test_extractHostFromHostPort(t *testing.T) {
	tests := []struct {
		ep   string
		want string
	}{
		{ep: "localhost", want: "localhost"},
		{ep: "localhost:8080", want: "localhost"},
		{ep: "192.158.7.14:8080", want: "192.158.7.14"},
		{ep: "192.158.7.14:8080", want: "192.158.7.14"},
		{ep: "[2001:db8:1f70::999:de8:7648:6e8]", want: "[2001:db8:1f70::999:de8:7648:6e8]"},
		{ep: "[2001:db8:1f70::999:de8:7648:6e8]:100", want: "2001:db8:1f70::999:de8:7648:6e8"},
	}
	for _, tt := range tests {
		t.Run(tt.ep, func(t *testing.T) {
			if got := extractHostFromHostPort(tt.ep); got != tt.want {
				t.Errorf("extractHostFromHostPort() = %v, want %v", got, tt.want)
			}
		})
	}
}
