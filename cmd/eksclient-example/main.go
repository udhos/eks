// Package main implements the tool.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/udhos/boilerplate/awsconfig"
	"github.com/udhos/boilerplate/boilerplate"
	"github.com/udhos/eks/eksclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func main() {
	me := filepath.Base(os.Args[0])
	log.Println(boilerplate.LongVersion(me))

	//
	// parse cmde line
	//

	if len(os.Args) < 2 {
		fmt.Printf("usage:   %s cluster-name\n", me)
		fmt.Printf("example: %s test\n", me)
		os.Exit(1)
	}

	clusterName := os.Args[1]

	const defaultReuse = false
	reuse := defaultReuse
	{
		const envReuse = "REUSE_TOKEN"
		r := os.Getenv(envReuse)
		if r != "" {
			rr, errParse := strconv.ParseBool(r)
			if errParse != nil {
				log.Fatalf("parse error: %s='%s': %v", envReuse, r, errParse)
			}
			reuse = rr
		}
		log.Printf("%s='%s' default=%t reuse=%t",
			envReuse, r, defaultReuse, reuse)
	}

	//
	// create eks client
	//

	options := awsconfig.Options{}
	awsCfg, errCfg := awsconfig.AwsConfig(options)
	if errCfg != nil {
		log.Fatalf("could not get aws config: %v", errCfg)
	}

	clientEks := eks.NewFromConfig(awsCfg.AwsConfig)

	//
	// get cluster data from eks client: CA data, endpoint
	//

	input := eks.DescribeClusterInput{Name: aws.String(clusterName)}

	out, errDesc := clientEks.DescribeCluster(context.TODO(), &input)
	if errDesc != nil {
		log.Fatalf("describe eks cluster error: %v", errDesc)
	}

	clusterCAData := aws.ToString(out.Cluster.CertificateAuthority.Data)
	clusterEndpoint := aws.ToString(out.Cluster.Endpoint)

	log.Printf("clusterName: %s", clusterName)
	log.Printf("clusterCAData: %s", clusterCAData)
	log.Printf("clusterEndpoint: %s", clusterEndpoint)

	//
	// create k8s client (clientset) from cluster data
	//

	eksclientOptions := eksclient.Options{
		ClusterName:     clusterName,
		ClusterCAData:   clusterCAData,
		ClusterEndpoint: clusterEndpoint,
		DebugLog:        true,
		ReuseToken:      reuse,
	}

	clientset, errClientset := eksclient.New(eksclientOptions)
	if errClientset != nil {
		log.Fatalf("clientset: %v", errClientset)
	}

	//
	// use k8s client to list namespaces
	//
	for i := range 3 {
		fmt.Printf("listing namespaces %d:\n", i+1)
		listNamespaces(clientset)
	}
}

func listNamespaces(clientset *kubernetes.Clientset) {
	list, errList := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if errList != nil {
		log.Fatalf("list namespaces: %v", errList)
	}

	log.Printf("found %d namespaces", len(list.Items))

	for _, ns := range list.Items {
		fmt.Println(ns.Name)
	}
}
