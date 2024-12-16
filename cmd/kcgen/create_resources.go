package main

import (
	"context"
	"fmt"
	"log"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createNS(ns string, ctx context.Context, kubeclient *kubernetes.Clientset) {
	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	_, err := kubeclient.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create namespace %s: %v", ns, err)
	} else {
		log.Printf("Created namespace %s", ns)
	}
}

func createRole(rb, ns, username string, ctx context.Context, kubeclient *kubernetes.Clientset) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rb,
			Namespace: ns,
		},
	}
	_, err := kubeclient.RbacV1().Roles(ns).Create(ctx, role, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create role %s in namespace %s: %v", rb, ns, err)
	} else {
		log.Printf("Created role %s in namespace %s", rb, ns)
	}
}

func createRB(rb, ns, username string, ctx context.Context, kubeclient *kubernetes.Clientset) {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rolebinding", rb),
			Namespace: ns,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "User",
				Name:      username,
				Namespace: ns,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     rb,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	_, err := kubeclient.RbacV1().RoleBindings(ns).Create(ctx, roleBinding, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create role binding for role %s in namespace %s: %v", rb, ns, err)
	} else {
		log.Printf("Created role binding for role %s in namespace %s", rb, ns)
	}
}

func createCR(crb string, ctx context.Context, kubeclient *kubernetes.Clientset) {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crb,
		},
	}
	_, err := kubeclient.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create cluster role %s: %v", crb, err)
	} else {
		log.Printf("Created cluster role %s", crb)
	}
}

func createCRB(crb, username string, ctx context.Context, kubeclient *kubernetes.Clientset) {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-clusterrolebinding", crb),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: username,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     crb,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	_, err := kubeclient.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create cluster role binding for cluster role %s: %v", crb, err)
	} else {
		log.Printf("Created cluster role binding for cluster role %s", crb)
	}
}
