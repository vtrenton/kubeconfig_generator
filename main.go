package main

import (
	_ "embed"
	"log"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// statically link the kubeconfig template file to make binary portable

//go:embed kc-template.yaml
var kctmplt string

func main() {

	print(kctmplt)
	//kubeclient := genkubeclient()

	// prompt user for key information

	// create sa,rb/crb on cluster
	// kubectl create serviceaccount my-service-account --namespace my-namespace
	// kubectl create rolebinding my-rolebinding --role=my-role --serviceaccount=my-namespace:my-service-account --namespace=my-namespace
	// kubectl create clusterrolebinding my-clusterrolebinding --clusterrole=my-clusterrole --serviceaccount=my-namespace:my-service-account

	// generate kubeconfig
	// kubectl get secret $(kubectl get serviceaccount my-service-account -n my-namespace -o jsonpath='{.secrets[0].name}') -n my-namespace -o jsonpath='{.data.token}' | base64 --decode
	// kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}'
	// kubectl get secret $(kubectl get serviceaccount my-service-account -n my-namespace -o jsonpath='{.secrets[0].name}') -n my-namespace -o jsonpath='{.data.ca\.crt}' | base64 --decode

	// fill out kc-tmplt.yaml
}

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
