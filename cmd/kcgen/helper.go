package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Helper function that creates the client object from the specified kubeconfig
func genkubeclient() *kubernetes.Clientset {
	kcpath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", kcpath)
	if err != nil {
		log.Fatalf("error loading kubeconfig file: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		log.Fatalf("unable to generate the kubernetes client: %v", err)
	}

	return clientset
}

// Determine if the cluster we are deploying into is healthy.
// Print out what we are doing and have the user validate
func validateCluster() bool {
	reader := bufio.NewReader(os.Stdin)
	kubeconfigPath := os.Getenv("HOME") + "/.kube/config"

	// Load the kubeconfig file
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		log.Fatalf("Failed to load kubeconfig file: %v", err)
	}

	// Get the current context
	currentContext := config.CurrentContext

	// Get the server associated with the current context
	clusterDetails, exists := config.Clusters[config.Contexts[currentContext].Cluster]
	if !exists {
		log.Fatalf("Cluster %s not found in kubeconfig", config.Contexts[currentContext].Cluster)
	}

	fmt.Printf("The current context in $HOME/.kube/config is set to %s with a server of %s\n", currentContext, clusterDetails.Server)
	fmt.Print("Is this OK? (y/N) ")
	contextVerified, _ := reader.ReadString('\n')
	contextVerified = strings.TrimSpace(contextVerified)

	if contextVerified == "yes" || contextVerified == "y" {
		// We still need to validate if server is alive before continuing
		serverAddress := strings.TrimPrefix(clusterDetails.Server, "https://")
		serverAddress = strings.TrimSuffix(serverAddress, "/")

		conn, err := net.DialTimeout("tcp", serverAddress, 5*time.Second)
		if err != nil {
			log.Fatalf("Kubernetes appears to be offline! Failed to connect to %s: %v", serverAddress, err)
		}
		_ = conn.Close()

		// if TCP connect test to kubernetes API is good - continue
		return true
	} else {
		return false
	}
}
