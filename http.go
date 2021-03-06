// Copyright 2013 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package utils

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
)

func init() {
	defaultTransport := http.DefaultTransport.(*http.Transport)
	installHTTPDialShim(defaultTransport)
	registerFileProtocol(defaultTransport)
}

// registerFileProtocol registers support for file:// URLs on the given transport.
func registerFileProtocol(transport *http.Transport) {
	transport.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
}

// SSLHostnameVerification is used as a switch for when a given provider might
// use self-signed credentials and we should not try to verify the hostname on
// the TLS/SSL certificates
type SSLHostnameVerification bool

const (
	// VerifySSLHostnames ensures we verify the hostname on the certificate
	// matches the host we are connecting and is signed
	VerifySSLHostnames = SSLHostnameVerification(true)
	// NoVerifySSLHostnames informs us to skip verifying the hostname
	// matches a valid certificate
	NoVerifySSLHostnames = SSLHostnameVerification(false)
)

// GetHTTPClient returns either a standard http client or
// non validating client depending on the value of verify.
func GetHTTPClient(verify SSLHostnameVerification, certs ...string) *http.Client {
	if len(certs) > 0 {
		return getHTTPClientWithCerts(verify, certs)
	}
	if verify == VerifySSLHostnames {
		return GetValidatingHTTPClient()
	}
	return GetNonValidatingHTTPClient()
}

// getHTTPClientWithCerts returns a new http.Client that verifies the
// server's certificate chain and hostname depending on arguments and
// adds ca certificates to the client. Returns nil if no certificates
// provided.
func getHTTPClientWithCerts(verify SSLHostnameVerification, certs []string) *http.Client {
	if len(certs) == 0 {
		return nil
	}
	pool := x509.NewCertPool()
	for _, cert := range certs {
		pool.AppendCertsFromPEM([]byte(cert))
	}
	tlsConfig := SecureTLSConfig()
	tlsConfig.RootCAs = pool
	if verify == NoVerifySSLHostnames {
		tlsConfig.InsecureSkipVerify = true
	}
	return &http.Client{
		Transport: NewHttpTLSTransport(tlsConfig),
	}
}

// GetValidatingHTTPClient returns a new http.Client that
// verifies the server's certificate chain and hostname.
func GetValidatingHTTPClient() *http.Client {
	return &http.Client{}
}

// GetNonValidatingHTTPClient returns a new http.Client that
// does not verify the server's certificate chain and hostname.
func GetNonValidatingHTTPClient() *http.Client {
	return &http.Client{
		Transport: NewHttpTLSTransport(&tls.Config{
			InsecureSkipVerify: true,
		}),
	}
}

// BasicAuthHeader creates a header that contains just the "Authorization"
// entry.  The implementation was originally taked from net/http but this is
// needed externally from the http request object in order to use this with
// our websockets. See 2 (end of page 4) http://www.ietf.org/rfc/rfc2617.txt
// "To receive authorization, the client sends the userid and password,
// separated by a single colon (":") character, within a base64 encoded string
// in the credentials."
func BasicAuthHeader(username, password string) http.Header {
	auth := username + ":" + password
	encoded := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	return http.Header{
		"Authorization": {encoded},
	}
}

// ParseBasicAuth attempts to find an Authorization header in the supplied
// http.Header and if found parses it as a Basic header. See 2 (end of page 4)
// http://www.ietf.org/rfc/rfc2617.txt "To receive authorization, the client
// sends the userid and password, separated by a single colon (":") character,
// within a base64 encoded string in the credentials."
func ParseBasicAuthHeader(h http.Header) (userid, password string, err error) {
	parts := strings.Fields(h.Get("Authorization"))
	if len(parts) != 2 || parts[0] != "Basic" {
		return "", "", fmt.Errorf("invalid or missing HTTP auth header")
	}
	// Challenge is a base64-encoded "tag:pass" string.
	// See RFC 2617, Section 2.
	challenge, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("invalid HTTP auth encoding")
	}
	tokens := strings.SplitN(string(challenge), ":", 2)
	if len(tokens) != 2 {
		return "", "", fmt.Errorf("invalid HTTP auth contents")
	}
	return tokens[0], tokens[1], nil
}

// OutgoingAccessAllowed determines whether connections other than
// localhost can be dialled.
var OutgoingAccessAllowed = true

func isLocalAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return host == "localhost" || net.ParseIP(host).IsLoopback()
}
