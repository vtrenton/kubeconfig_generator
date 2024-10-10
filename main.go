package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
)

type kubeuser struct {
	Username     string   `yaml:"username"`
	Namespaces   []string `yaml:"namespaces"`
	Roles        []string `yaml:"roles"`
	Clusterroles []string `yaml:"clusterroles"`
	Clientcert   string   `yaml:"clientcert"`
	Clientkey    string   `yaml:"clientkey"`
}

func (kuser *kubeuser) parseConfigYaml(configpath string) {
	userconfig, err := os.Open(configpath)
	if err != nil {
		log.Fatalf("Could not access file for reading: %v", err)
	}
	defer userconfig.Close()

	decoder := yaml.NewDecoder(userconfig)
	err = decoder.Decode(kuser)
	if err != nil {
		log.Fatalf("Could not parse yaml - please validate syntax: %v", err)
	}
}

// TODO: REFCATOR
func (kuser kubeuser) createNewUser(kubeclient *kubernetes.Clientset) {
	ctx := context.Background()

	// Loop through each namespace associated with the user
	for _, ns := range kuser.Namespaces {
		// Check if the namespace exists
		_, err := kubeclient.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if err != nil {
			// If the namespace doesn't exist, create it
			namespace := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			}
			_, err = kubeclient.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
			if err != nil {
				log.Printf("Failed to create namespace %s: %v", ns, err)
			} else {
				log.Printf("Created namespace %s", ns)
			}
		}

		// Loop through each role to create role bindings
		for _, rb := range kuser.Roles {
			// Check if the role exists
			_, err := kubeclient.RbacV1().Roles(ns).Get(ctx, rb, metav1.GetOptions{})
			if err != nil {
				// If the role doesn't exist, create it
				role := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      rb,
						Namespace: ns,
					},
				}
				_, err = kubeclient.RbacV1().Roles(ns).Create(ctx, role, metav1.CreateOptions{})
				if err != nil {
					log.Printf("Failed to create role %s in namespace %s: %v", rb, ns, err)
				} else {
					log.Printf("Created role %s in namespace %s", rb, ns)
				}
			}

			// Create the role binding for the user
			roleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-rolebinding", rb),
					Namespace: ns,
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "User",
						Name:      kuser.Username,
						Namespace: ns,
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind:     "Role",
					Name:     rb,
					APIGroup: "rbac.authorization.k8s.io",
				},
			}
			_, err = kubeclient.RbacV1().RoleBindings(ns).Create(ctx, roleBinding, metav1.CreateOptions{})
			if err != nil {
				log.Printf("Failed to create role binding for role %s in namespace %s: %v", rb, ns, err)
			} else {
				log.Printf("Created role binding for role %s in namespace %s", rb, ns)
			}
		}

		// Loop through each cluster role to create cluster role bindings
		for _, crb := range kuser.Clusterroles {
			// Check if the cluster role exists
			_, err := kubeclient.RbacV1().ClusterRoles().Get(ctx, crb, metav1.GetOptions{})
			if err != nil {
				// If the cluster role doesn't exist, create it
				clusterRole := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: crb,
					},
				}
				_, err = kubeclient.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
				if err != nil {
					log.Printf("Failed to create cluster role %s: %v", crb, err)
				} else {
					log.Printf("Created cluster role %s", crb)
				}
			}
			// Check if the cluster role binding already exists
			clusterRoleBindingName := fmt.Sprintf("%s-clusterrolebinding", crb)
			_, err = kubeclient.RbacV1().ClusterRoleBindings().Get(ctx, clusterRoleBindingName, metav1.GetOptions{})
			if err == nil {
				// ClusterRoleBinding already exists
				log.Printf("ClusterRoleBinding %s already exists, skipping creation.", clusterRoleBindingName)
				continue
			}

			// If an error occurred, check if it's a NotFound error
			if !apierrors.IsNotFound(err) {
				log.Printf("Error checking existence of ClusterRoleBinding %s: %v", clusterRoleBindingName, err)
				continue
			}

			// Create the cluster role binding for the user
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-clusterrolebinding", crb),
				},
				Subjects: []rbacv1.Subject{
					{
						Kind: "User",
						Name: kuser.Username,
						//Namespace: ns,
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind:     "ClusterRole",
					Name:     crb,
					APIGroup: "rbac.authorization.k8s.io",
				},
			}
			_, err = kubeclient.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{})
			if err != nil {
				log.Printf("Failed to create cluster role binding for cluster role %s: %v", crb, err)
			} else {
				log.Printf("Created cluster role binding for cluster role %s", crb)
			}
		}
	}
}

// genKubeconfig generates a kubeconfig using the service account's token and cluster information
func (kuser kubeuser) genKubeconfig(kubeclient *kubernetes.Clientset) (*api.Config, error) {
	ctx := context.Background()
	kcname := kuser.Username
	clientkey := []byte(kuser.Clientkey)
	clientcert := []byte(kuser.Clientcert)

	// Retrieve the CA certificate from the kube-root-ca.crt ConfigMap in the kube-system namespace
	configMap, err := kubeclient.CoreV1().ConfigMaps("kube-system").Get(ctx, "kube-root-ca.crt", metav1.GetOptions{})
	if err != nil {
		log.Printf("Failed to retrieve ConfigMap containing CA certificate: %v", err)
		return nil, err
	}

	caCert, ok := configMap.Data["ca.crt"]
	if !ok {
		log.Printf("CA certificate not found in ConfigMap kube-root-ca.crt")
		return nil, fmt.Errorf("CA certificate not found in ConfigMap kube-root-ca.crt")
	}

	// Load the existing kubeconfig file to get the cluster server endpoint
	// A bit of a hack - might be a better way to accomplish this in the future
	kubeconfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	admkubeconfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		log.Printf("Failed to load existing kubeconfig: %v", err)
		return nil, err
	}

	// Extract the first cluster server endpoint from the kubeconfig
	var clusterServer string
	for _, cluster := range admkubeconfig.Clusters {
		clusterServer = cluster.Server
		break
	}

	// Construct the kubeconfig object using the Client certificate, Client key and CA certificate
	kubeconfig := &api.Config{
		Clusters: map[string]*api.Cluster{
			"kubernetes": {
				Server:                   clusterServer,
				CertificateAuthorityData: []byte(caCert),
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			kcname: {
				ClientKeyData:         clientkey,
				ClientCertificateData: clientcert,
			},
		},
		Contexts: map[string]*api.Context{
			"kubernetes": {
				Cluster:  "kubernetes",
				AuthInfo: kcname,
			},
		},
		CurrentContext: "kubernetes",
	}

	log.Printf("Successfully generated kubeconfig for %s\n", kcname)
	return kubeconfig, nil
}

func main() {
	var kuser kubeuser
	kubeclient := genkubeclient()

	if len(os.Args) != 2 {
		fmt.Println("Usage: pass a path to a config yaml file as an argument to this program to generate a Kubeconfig for a user and the assoiated Rolebindings/ClusterRoleBindings for the user on the cluster itself.")
		fmt.Println("Please make sure you have a valid admin kubeconfig at $HOME/.kube/config as well")
		os.Exit(0)
	}

	if !fileExists(os.Args[1]) {
		log.Fatalf("Invalid config file")
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
