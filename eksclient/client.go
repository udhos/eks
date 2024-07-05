// Package eksclient creates EKS client.
package eksclient

import (
	"encoding/base64"
	"fmt"
	"log"

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
}

// New creates kubernetes client.
func New(options Options) (*kubernetes.Clientset, error) {

	const me = "eksclient.New"

	if options.Logf == nil {
		options.Logf = log.Printf
	}

	debugf := func(format string, v ...any) {
		if options.DebugLog {
			options.Logf(fmt.Sprintf("DEBUG: %s: ", me)+format, v...)
		}
	}

	return newClientset(debugf, options.ClusterName, options.ClusterCAData, options.ClusterEndpoint)
}

// newClientset cria um client para o kubernetes.
// FIXME WRITEME TODO XXX Refresh/renew token automatically.
func newClientset(debugf func(format string, v ...any), clusterName, clusterCAData, clusterEndpoint string) (*kubernetes.Clientset, error) {

	debugf("newClientset: clusterName=%s endpoint=%s CA=%s",
		clusterName, clusterEndpoint, clusterCAData)

	gen, err := token.NewGenerator(true, false)
	if err != nil {
		return nil, err
	}
	opts := &token.GetTokenOptions{
		ClusterID: clusterName,
	}
	tok, err := gen.GetWithOptions(opts)
	if err != nil {
		return nil, err
	}
	ca, err := base64.StdEncoding.DecodeString(clusterCAData)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(
		&rest.Config{
			Host: clusterEndpoint,

			// FIXME WRITEME TODO XXX
			//
			// Refresh/renew token automatically.
			BearerToken: tok.Token,

			TLSClientConfig: rest.TLSClientConfig{
				CAData: ca,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}
