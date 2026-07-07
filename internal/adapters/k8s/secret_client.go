// Package k8s implements the domain.SecretUpdater port by upserting
// kubernetes.io/tls Secrets via client-go, auto-detecting whether the
// process is running in-cluster or should fall back to a kubeconfig file.
package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// SecretClient implements domain.SecretUpdater.
type SecretClient struct {
	clientset *kubernetes.Clientset
}

// New builds a SecretClient, trying in-cluster config first (when running as
// a pod with a mounted ServiceAccount) and falling back to the kubeconfig at
// kubeconfigPath (or the client-go default loading rules, e.g. KUBECONFIG env
// / ~/.kube/config, when kubeconfigPath is empty).
func New(kubeconfigPath string) (*SecretClient, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		if kubeconfigPath != "" {
			loadingRules.ExplicitPath = kubeconfigPath
		}
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("load kubernetes config (in-cluster and kubeconfig both failed): %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build kubernetes clientset: %w", err)
	}
	return &SecretClient{clientset: clientset}, nil
}

// UpsertTLSSecret creates or updates a kubernetes.io/tls Secret containing
// the given cert/key PEM material in the target namespace.
func (c *SecretClient) UpsertTLSSecret(ctx context.Context, namespace, name string, certPEM, keyPEM []byte) error {
	secrets := c.clientset.CoreV1().Secrets(namespace)

	existing, err := secrets.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get secret %s/%s: %w", namespace, name, err)
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Type:       corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       certPEM,
				corev1.TLSPrivateKeyKey: keyPEM,
			},
		}
		if _, err := secrets.Create(ctx, secret, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create secret %s/%s: %w", namespace, name, err)
		}
		return nil
	}

	existing.Type = corev1.SecretTypeTLS
	if existing.Data == nil {
		existing.Data = map[string][]byte{}
	}
	existing.Data[corev1.TLSCertKey] = certPEM
	existing.Data[corev1.TLSPrivateKeyKey] = keyPEM

	if _, err := secrets.Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update secret %s/%s: %w", namespace, name, err)
	}
	return nil
}
