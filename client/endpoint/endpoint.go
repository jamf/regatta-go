package endpoint

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"
)

type CredsRequirement int

const (
	// CredsRequire - Credentials/certificate required for thi type of connection.
	CredsRequire CredsRequirement = iota
	// CredsDrop - Credentials/certificate not needed and should get ignored.
	CredsDrop
	// CredsOptional - Credentials/certificate might be used if supplied
	CredsOptional
)

func extractHostFromHostPort(ep string) string {
	host, _, err := net.SplitHostPort(ep)
	if err != nil {
		return ep
	}
	return host
}

func extractHostFromPath(pathStr string) string {
	return extractHostFromHostPort(path.Base(pathStr))
}

// mustSplit2 returns the values from strings.SplitN(s, sep, 2).
// If sep is not found, it returns ("", "", false) instead.
func mustSplit2(s, sep string) (string, string) {
	spl := strings.SplitN(s, sep, 2)
	if len(spl) < 2 {
		panic(fmt.Errorf("token '%v' expected to have separator sep: `%v`", s, sep))
	}
	return spl[0], spl[1]
}

func schemeToCredsRequirement(schema string) CredsRequirement {
	switch schema {
	case "https", "unixs":
		return CredsRequire
	case "http":
		return CredsDrop
	case "unix":
		return CredsOptional
	case "":
		return CredsOptional
	default:
		return CredsOptional
	}
}

// This function translates endpoints names supported by regatta server into
// endpoints as supported by grpc with additional information
// (server_name for cert validation, requireCreds - whether certs are needed).
// The main differences:
//   - regatta supports unixs & https names as opposed to unix & http to
//     distinguish need to configure certificates.
//   - regatta support http(s) names as opposed to tcp supported by grpc/dial method.
//   - regatta supports unix(s)://local-file naming schema
//     (as opposed to unix:local-file canonical name used by grpc for current dir files).
//   - Within the unix(s) schemas, the last segment (filename) without 'port' (content after colon)
//     is considered serverName - to allow local testing of cert-protected communication.
//
// See more:
//   - https://github.com/grpc/grpc-go/blob/26c143bd5f59344a4b8a1e491e0f5e18aa97abc7/internal/grpcutil/target.go#L47
//   - https://golang.org/pkg/net/#Dial
//   - https://github.com/grpc/grpc/blob/master/doc/naming.md
func translateEndpoint(ep string) (addr string, serverName string, requireCreds CredsRequirement) {
	if strings.HasPrefix(ep, "unix:") || strings.HasPrefix(ep, "unixs:") {
		if strings.HasPrefix(ep, "unix:///") || strings.HasPrefix(ep, "unixs:///") {
			// absolute path case
			schema, absolutePath := mustSplit2(ep, "://")
			return "unix://" + absolutePath, extractHostFromPath(absolutePath), schemeToCredsRequirement(schema)
		}
		if strings.HasPrefix(ep, "unix://") || strings.HasPrefix(ep, "unixs://") {
			// legacy regatta local path
			schema, localPath := mustSplit2(ep, "://")
			return "unix:" + localPath, extractHostFromPath(localPath), schemeToCredsRequirement(schema)
		}
		schema, localPath := mustSplit2(ep, ":")
		return "unix:" + localPath, extractHostFromPath(localPath), schemeToCredsRequirement(schema)
	}

	if strings.Contains(ep, "://") {
		url, err := url.Parse(ep)
		if err != nil {
			return ep, extractHostFromHostPort(ep), CredsOptional
		}
		if url.Scheme == "http" || url.Scheme == "https" {
			return url.Host, url.Hostname(), schemeToCredsRequirement(url.Scheme)
		}
		return ep, url.Hostname(), schemeToCredsRequirement(url.Scheme)
	}
	// Handles plain addresses like 10.0.0.44:437.
	return ep, extractHostFromHostPort(ep), CredsOptional
}

// RequiresCredentials returns whether given endpoint requires
// credentials/certificates for connection.
func RequiresCredentials(ep string) CredsRequirement {
	_, _, requireCreds := translateEndpoint(ep)
	return requireCreds
}

// Interpret endpoint parses an endpoint of the form
// (http|https)://<host>*|(unix|unixs)://<path>)
// and returns low-level address (supported by 'net') to connect to,
// and a server name used for x509 certificate matching.
func Interpret(ep string) (address string, serverName string) {
	addr, serverName, _ := translateEndpoint(ep)
	return addr, serverName
}
