package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// statically link the kubeconfig template file to make binary portable

//go:embed kc-template.yaml
var kctmplt string

type kubeuser struct {
	Saname       string   `yaml:"saname"`
	Existing     bool     `yaml:"existing"`
	Isadmin      bool     `yaml:"isadmin"`
	Namespaces   []string `yaml:"namespaces"`
	Roles        []string `yaml:"roles"`
	Clusterroles []string `yaml:"clusterroles"`
}

func (kuser *kubeuser) parseConfigYaml(configpath string) {
	userconfig, err := os.Open(configpath)
	if err != nil {
		log.Fatalf("Could not access file for reading: %v", err)
		os.Exit(1)
	}
	defer userconfig.Close()

	decoder := yaml.NewDecoder(userconfig)
	err = decoder.Decode(kuser)
	if err != nil {
		log.Fatalf("Could not parse yaml - please validate syntax: %v", err)
		os.Exit(1)
	}
}

func (kuser *kubeuser) createNewUser(kubeclient *kubernetes.Clientset) {
	// create sa,rb/crb on cluster
	// kubectl create serviceaccount my-service-account --namespace my-namespace
	// kubectl create rolebinding my-rolebinding --role=my-role --serviceaccount=my-namespace:my-service-account --namespace=my-namespace
	// kubectl create clusterrolebinding my-clusterrolebinding --clusterrole=my-clusterrole --serviceaccount=my-namespace:my-service-account
}

func (kuser *kubeuser) genKubeconfig() {
	if !kuser.Existing {

	}
	// generate kubeconfig
	// kubectl get secret $(kubectl get serviceaccount my-service-account -n my-namespace -o jsonpath='{.secrets[0].name}') -n my-namespace -o jsonpath='{.data.token}' | base64 --decode
	// kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}'
	// kubectl get secret $(kubectl get serviceaccount my-service-account -n my-namespace -o jsonpath='{.secrets[0].name}') -n my-namespace -o jsonpath='{.data.ca\.crt}' | base64 --decode
}
func main() {
	var configpath string
	var kuser kubeuser
	kubeclient := genkubeclient()

	if len(os.Args) > 2 {
		log.Fatalf("Usage: either run as standalone or with path to config yaml")
		os.Exit(1)
	}

	if len(os.Args) == 2 {
		if fileExists(os.Args[1]) {
			// configuration file exist set it here
			configpath = os.Args[1]
			kuser.parseConfigYaml(configpath)
			kuser.createNewUser(kubeclient)
		}
	} else {
		promptUser(&configpath, &kuser, kubeclient)
	}

	kuser.genKubeconfig()

	print(kctmplt)
}

// painful helper function
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
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

func promptUser(configPath *string, kuser *kubeuser, kubeclient *kubernetes.Clientset) {
	reader := bufio.NewReader(os.Stdin)

	// Prompt the user to create a new user or use an existing one
	fmt.Print("Would you like to create a new user? (yes/no): ")
	createNewUser, _ := reader.ReadString('\n')
	createNewUser = strings.TrimSpace(strings.ToLower(createNewUser))

	if createNewUser == "yes" {
		// Ask for the path to the config file
		fmt.Print("Please enter the path to the config file: ")
		path, _ := reader.ReadString('\n')
		*configPath = strings.TrimSpace(path)
		fmt.Printf("Config path set to: %s\n", *configPath)
		kuser.parseConfigYaml(*configPath)
		kuser.createNewUser(kubeclient)

	} else {
		// Prompt for existing user configuration
		fmt.Print("Would you like to get the config file for an existing user? (yes/no): ")
		getExistingConfig, _ := reader.ReadString('\n')
		getExistingConfig = strings.TrimSpace(strings.ToLower(getExistingConfig))

		if getExistingConfig == "yes" {
			// Ask for the service account name
			fmt.Print("What is the service account name? ")
			serviceAccountName, _ := reader.ReadString('\n')
			serviceAccountName = strings.TrimSpace(serviceAccountName)

			// Ask for the namespace of the service account
			fmt.Print("What is the namespace of the service account? ")
			namespace, _ := reader.ReadString('\n')
			namespace = strings.TrimSpace(namespace)

			// convert namespace to slice
			ns := []string{namespace}

			// set up a existing user with genKubeconfig
			kuser.Saname = serviceAccountName
			kuser.Existing = true
			kuser.Namespaces = ns
		}
	}
}
