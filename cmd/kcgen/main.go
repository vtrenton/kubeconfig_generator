package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var kuser kubeuser
	kubeclient := genkubeclient()

	if len(os.Args) != 2 {
		fmt.Println("Usage: pass a path to a config yaml file as an argument to this program to generate a Kubeconfig for a user and the assoiated Rolebindings/ClusterRoleBindings for the user on the cluster itself.")
		fmt.Println("Please make sure you have a valid admin kubeconfig at $HOME/.kube/config as well")
		os.Exit(0)
	}

	info, err := os.Stat(os.Args[1])
	if os.IsNotExist(err) || info.IsDir() {
		log.Fatalf("Invalid filepath specified")
	}
	// configuration file exist set it here
	configpath := os.Args[1]
	kuser.parseConfigYaml(configpath)

	// give the user a chance to stop before any changes are made
	if validateCluster() {
		kuser.createNewUser(kubeclient)
		kubeconfig, err := kuser.genKubeconfig(kubeclient)
		if err != nil {
			log.Fatalf("Failed to generate kubeconfig: %v", err)
		}

		// Convert kubeconfig to YAML format
		kubeconfigYAML, err := clientcmd.Write(*kubeconfig)
		if err != nil {
			log.Fatalf("Failed to convert kubeconfig to YAML: %v", err)
		}

		// Define the file path to write the kubeconfig file
		kubeconfigPath := filepath.Join(homedir.HomeDir(), ".kube", fmt.Sprintf("%s-kubeconfig.yaml", kuser.Username))

		// Write the YAML data to a file
		err = os.WriteFile(kubeconfigPath, kubeconfigYAML, 0644)
		if err != nil {
			log.Fatalf("Failed to write kubeconfig to file: %v", err)
		}

		log.Printf("Successfully wrote kubeconfig to %s", kubeconfigPath)
	} else {
		fmt.Println("No changes will be made!")
	}
}
