package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
)

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

		// Create the service account in the namespace
		sa := &v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kuser.Saname,
				Namespace: ns,
			},
		}
		_, err = kubeclient.CoreV1().ServiceAccounts(ns).Create(ctx, sa, metav1.CreateOptions{})
		if err != nil {
			log.Printf("Failed to create service account in namespace %s: %v", ns, err)
		} else {
			log.Printf("Created service account in namespace %s", ns)
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

			// Create the role binding for the service account
			roleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-rolebinding", rb),
					Namespace: ns,
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      kuser.Saname,
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

			// Create the cluster role binding for the service account
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-clusterrolebinding", crb),
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      kuser.Saname,
						Namespace: ns,
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
	namespace := kuser.Namespaces[0]
	serviceAccountName := kuser.Saname

	// Retrieve the service account
	sa, err := kubeclient.CoreV1().ServiceAccounts(namespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
	if err != nil {
		log.Printf("Failed to get service account %s in namespace %s: %v", serviceAccountName, namespace, err)
		return nil, err
	}

	// Get the secret name associated with the service account
	if len(sa.Secrets) == 0 {
		return nil, fmt.Errorf("no secrets found for service account %s in namespace %s", serviceAccountName, namespace)
	}
	secretName := sa.Secrets[0].Name

	// Retrieve the secret containing the service account token and CA certificate
	secret, err := kubeclient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		log.Printf("Failed to get secret %s for service account %s in namespace %s: %v", secretName, serviceAccountName, namespace, err)
		return nil, err
	}

	// Extract the token from the secret
	token, ok := secret.Data["token"]
	if !ok {
		return nil, fmt.Errorf("token not found in secret %s for service account %s in namespace %s", secretName, serviceAccountName, namespace)
	}

	// Extract the CA certificate from the secret
	caCert, ok := secret.Data["ca.crt"]
	if !ok {
		return nil, fmt.Errorf("ca.crt not found in secret %s for service account %s in namespace %s", secretName, serviceAccountName, namespace)
	}

	// Get the cluster server endpoint
	clusterInfo, err := kubeclient.CoreV1().ConfigMaps("kube-system").Get(ctx, "kubeadm-config", metav1.GetOptions{})
	if err != nil {
		log.Printf("Failed to get cluster server endpoint: %v", err)
		return nil, err
	}

	clusterServer := clusterInfo.Data["apiServer"]

	// Construct the kubeconfig object
	kubeconfig := &api.Config{
		Clusters: map[string]*api.Cluster{
			"kubernetes": {
				Server:                   clusterServer,
				CertificateAuthorityData: caCert,
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			serviceAccountName: {
				Token: base64.StdEncoding.EncodeToString(token),
			},
		},
		Contexts: map[string]*api.Context{
			"kubernetes": {
				Cluster:  "kubernetes",
				AuthInfo: serviceAccountName,
			},
		},
		CurrentContext: "kubernetes",
	}

	log.Printf("Successfully generated kubeconfig for service account %s in namespace %s", serviceAccountName, namespace)
	return kubeconfig, nil
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

	// Generate the kubeconfig using the genKubeconfig function
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
	kubeconfigPath := filepath.Join(homedir.HomeDir(), ".kube", fmt.Sprintf("%s-kubeconfig.yaml", kuser.Saname))

	// Write the YAML data to a file
	err = os.WriteFile(kubeconfigPath, kubeconfigYAML, 0644)
	if err != nil {
		log.Fatalf("Failed to write kubeconfig to file: %v", err)
	}

	log.Printf("Successfully wrote kubeconfig to %s", kubeconfigPath)
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
		} else {
			fmt.Println("Bye!")
			os.Exit(0)
		}
	}
}
