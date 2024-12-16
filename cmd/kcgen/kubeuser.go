package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
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

// Create needed Resources and binding on the cluster
func (kuser kubeuser) createNewUser(kubeclient *kubernetes.Clientset) {
	ctx := context.Background()
	// Namespaces
	for _, ns := range kuser.Namespaces {
		_, err := kubeclient.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if err != nil {
			createNS(ns, ctx, kubeclient)
		}
		// Roles
		for _, rb := range kuser.Roles {
			_, err := kubeclient.RbacV1().Roles(ns).Get(ctx, rb, metav1.GetOptions{})
			if err != nil {
				createRole(rb, ns, kuser.Username, ctx, kubeclient)
			}
			// Role Bindings
			createRB(rb, ns, kuser.Username, ctx, kubeclient)
		}
	}
	// Cluster Roles
	for _, crb := range kuser.Clusterroles {
		_, err := kubeclient.RbacV1().ClusterRoles().Get(ctx, crb, metav1.GetOptions{})
		if err != nil {
			createCR(crb, ctx, kubeclient)
		}
		// Cluster Role Bindings
		clusterRoleBindingName := fmt.Sprintf("%s-clusterrolebinding", crb)
		_, err = kubeclient.RbacV1().ClusterRoleBindings().Get(ctx, clusterRoleBindingName, metav1.GetOptions{})
		if err != nil {
			createCRB(crb, kuser.Username, ctx, kubeclient)
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
