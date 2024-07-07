// Package eksclient creates EKS client.
package eksclient

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

// Options define config for client.
type Options struct {
	// ClusterName is required.
	ClusterName string

	// ClusterCAData holds PEM-encoded bytes (typically read from a root
	// certificates bundle).
	ClusterCAData string

	// ClusterEndpoint must be a host string, a host:port pair, or a URL
	// to the base of the apiserver. If a URL is given then the (optional)
	// Path of that URL represents a prefix that must be appended to all
	// request URIs used to access the apiserver. This allows a frontend
	// proxy to easily relocate all of the apiserver endpoints.
	ClusterEndpoint string

	// DebugLog optionally activates debug logs.
	DebugLog bool

	// Logf optionally defines logging fuction. If unspecified defaults to log.Printf.
	Logf func(format string, v ...any)

	// Source optionally defines a custom token source.
	Source TokenSource
}

// TokenSource generates tokens.
type TokenSource interface {
	Get() (token.Token, error)
}

type tokenGenerator struct {
	generator      token.Generator
	options        *token.GetTokenOptions
	last           token.Token
	refreshEarlier time.Duration
}

func newTokenGenerator(options Options) (*tokenGenerator, error) {
	gen, err := token.NewGenerator(true, false)
	if err != nil {
		return nil, err
	}
	opts := &token.GetTokenOptions{
		ClusterID: options.ClusterName,
	}
	return &tokenGenerator{
		generator:      gen,
		options:        opts,
		refreshEarlier: 10 * time.Second,
	}, nil
}

func (g *tokenGenerator) Get() (token.Token, error) {
	if needsRefresh(g.refreshEarlier, g.last.Expiration) {
		tok, err := g.generator.GetWithOptions(g.options)
		g.last = tok
		return tok, err
	}
	return g.last, nil
}

func needsRefresh(refreshEarlier time.Duration, expiration time.Time) bool {
	return time.Now().Add(refreshEarlier).After(expiration)
}

// New creates kubernetes client.
func New(options Options) (*kubernetes.Clientset, error) {

	const me = "eksclient.New"

	if options.Logf == nil {
		options.Logf = log.Printf
	}

	if options.Source == nil {
		source, err := newTokenGenerator(options)
		if err != nil {
			return nil, err
		}
		options.Source = source
	}

	debugf := func(format string, v ...any) {
		if options.DebugLog {
			options.Logf(fmt.Sprintf("DEBUG: %s: ", me)+format, v...)
		}
	}

	return newClientset(debugf, options.Source, options.ClusterName, options.ClusterCAData, options.ClusterEndpoint)
}

// newClientset creates kubernetes client.
// FIXME WRITEME TODO XXX Refresh/renew token automatically.
func newClientset(debugf func(format string, v ...any), source TokenSource,
	clusterName, clusterCAData, clusterEndpoint string) (*kubernetes.Clientset, error) {

	debugf("newClientset: clusterName=%s endpoint=%s CA=%s",
		clusterName, clusterEndpoint, clusterCAData)

	/*
		tok, err := source.Get()
		if err != nil {
			return nil, err
		}
	*/

	ca, err := base64.StdEncoding.DecodeString(clusterCAData)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(
		&rest.Config{
			Host: clusterEndpoint,

			//BearerToken: tok.Token,

			TLSClientConfig: rest.TLSClientConfig{
				CAData: ca,
			},

			WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
				return &tokenTransport{
					source:    source,
					transport: rt,
				}
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

type tokenTransport struct {
	source    TokenSource
	transport http.RoundTripper
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tok, err := t.source.Get()
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tok.Token))

	return t.transport.RoundTrip(req)
}
