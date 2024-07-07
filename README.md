[![license](http://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/udhos/eks/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/udhos/eks)](https://goreportcard.com/report/github.com/udhos/eks)
[![Go Reference](https://pkg.go.dev/badge/github.com/udhos/eks.svg)](https://pkg.go.dev/github.com/udhos/eks)

# eks

[eks](https://github.com/udhos/eks) creates kubernetes client (client-go clientset) for EKS api-server from explicit cluster name, CA-data and endpoint.

# Build

```
git clone https://github.com/udhos/eks
cd eks
./build.sh
```

# Test example program

Create an EKS cluster named `test`, then:

```
eksclient-example test
```

# Usage

See example program: [cmd/eksclient-example/main.go](cmd/eksclient-example/main.go)

```golang
import "github.com/udhos/eks/eksclient"

eksclientOptions := eksclient.Options{
    ClusterName:     clusterName,
    ClusterCAData:   clusterCAData,
    ClusterEndpoint: clusterEndpoint,
}

clientset, errClientset := eksclient.New(eksclientOptions)
if errClientset != nil {
    log.Fatalf("clientset: %v", errClientset)
}
```
