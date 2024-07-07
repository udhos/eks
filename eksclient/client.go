// Package eksclient creates EKS client.
package eksclient

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

// Options define config for client.
type Options struct {
	// ClusterName is required EKS cluster name.
	ClusterName string

	// ClusterCAData is required PEM-encoded bytes (typically read
	// from a root certificates bundle).
	ClusterCAData string

	// ClusterEndpoint is required host string, a host:port
	// pair, or a URL to the base of the apiserver. If a URL is given then
	// the (optional) Path of that URL represents a prefix that must be
	// appended to all request URIs used to access the apiserver. This
	// allows a frontend proxy to easily relocate all of the apiserver
	// endpoints.
	ClusterEndpoint string

	// DebugLog optionally activates debug logs.
	DebugLog bool

	// Logf optionally defines logging fuction. If unspecified defaults to log.Printf.
	Logf func(format string, v ...any)

	debugf func(format string, v ...any)

	// Source optionally defines a custom token source.
	Source TokenSource

	// RefreshEarlier optionally defines how long before token expiration should the token be refreshed.
	// If unspecified, defaults to 10s.
	RefreshEarlier time.Duration

	// ReuseToken optionally adds layer to force token reuse. Usually redundant.
	ReuseToken bool
}

// TokenSource generates tokens.
type TokenSource interface {
	// Get gets current token or generates a new one if the current one has expired or it is close to expire.
	Get() (token.Token, error)
}

type tokenGenerator struct {
	generator    token.Generator
	tokenOptions *token.GetTokenOptions
	last         token.Token
	options      Options
	lock         sync.Mutex
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
		generator:    gen,
		tokenOptions: opts,
		options:      options,
	}, nil
}

// Get gets current token or generates a new one if the current one has expired or it is close to expire.
func (g *tokenGenerator) Get() (token.Token, error) {
	g.lock.Lock()
	defer g.lock.Unlock()

	now := time.Now()

	g.debugToken(now, "old token")

	if g.needsRefresh(now) {
		tok, err := g.generator.GetWithOptions(g.tokenOptions)
		g.last = tok
		g.debugToken(now, "new token")
		return tok, err
	}

	return g.last, nil
}

func (g *tokenGenerator) needsRefresh(now time.Time) bool {
	if g.options.ReuseToken {
		return now.Add(g.options.RefreshEarlier).After(g.last.Expiration)
	}
	return true
}

func (g *tokenGenerator) debugToken(now time.Time, label string) {
	refresh := g.needsRefresh(now)
	remain := g.last.Expiration.Sub(now)
	if remain < 0 {
		remain = 0
	}

	tk := g.last.Token
	if len(tk) > 20 {
		tk = tk[len(tk)-19:]
	}

	g.options.debugf("Get: %s: reuse=%t expiration=%v remain=%v refreshEarlier=%v needsRefresh=%t token=%s",
		label, g.options.ReuseToken, g.last.Expiration, remain, g.options.RefreshEarlier, refresh, tk)
}

// New creates kubernetes client.
func New(options Options) (*kubernetes.Clientset, error) {

	if options.Logf == nil {
		options.Logf = log.Printf
	}

	if options.RefreshEarlier == 0 {
		options.RefreshEarlier = 10 * time.Second
	}

	options.debugf = func(format string, v ...any) {
		if options.DebugLog {
			options.Logf("DEBUG: eksclient: "+format, v...)
		}
	}

	if options.Source == nil {
		source, err := newTokenGenerator(options)
		if err != nil {
			return nil, err
		}
		options.Source = source
	}

	return newClientset(options.debugf, options.Source, options.ClusterName, options.ClusterCAData, options.ClusterEndpoint)
}

// newClientset creates kubernetes client.
func newClientset(debugf func(format string, v ...any), source TokenSource,
	clusterName, clusterCAData, clusterEndpoint string) (*kubernetes.Clientset, error) {

	debugf("newClientset: clusterName=%s endpoint=%s CA=%s",
		clusterName, clusterEndpoint, clusterCAData)

	ca, err := base64.StdEncoding.DecodeString(clusterCAData)
	if err != nil {
		return nil, err
	}

	config := &rest.Config{
		Host: clusterEndpoint,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: ca,
		},
	}

	// Adds a transport that refreshes the token when needed.
	config.Wrap(func(rt http.RoundTripper) http.RoundTripper {
		return &tokenTransport{
			source:    source,
			transport: rt,
			debugf:    debugf,
		}
	})

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

// tokenTransport is a transport wrapper that refreshes existing token when it has expired or it is close to expire.
type tokenTransport struct {
	source    TokenSource
	transport http.RoundTripper
	debugf    func(format string, v ...any)
}

// RoundTrip refreshes existing token when it has expired or it is close to expire.
func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {

	begin := time.Now()

	tok, err := t.source.Get()
	if err != nil {
		return nil, err
	}

	elap := time.Since(begin)
	t.debugf("RoundTrip: source.Get: %v", elap)

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tok.Token))

	return t.transport.RoundTrip(req)
}
