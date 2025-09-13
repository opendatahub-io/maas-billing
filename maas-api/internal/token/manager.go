package token

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/tier"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corelistersv1 "k8s.io/client-go/listers/core/v1"
)

type Manager struct {
	tenantName           string
	tierMapper           *tier.Mapper
	clientset            kubernetes.Interface
	namespaceLister      corelistersv1.NamespaceLister
	serviceAccountLister corelistersv1.ServiceAccountLister
}

func NewManager(
	tenantName string,
	tierMapper *tier.Mapper,
	clientset kubernetes.Interface,
	namespaceLister corelistersv1.NamespaceLister,
	serviceAccountLister corelistersv1.ServiceAccountLister,
) *Manager {
	return &Manager{
		tenantName:           tenantName,
		tierMapper:           tierMapper,
		clientset:            clientset,
		namespaceLister:      namespaceLister,
		serviceAccountLister: serviceAccountLister,
	}
}

// GenerateSAToken creates a Service Account token in the namespace bound to the tier the user belong to
func (m *Manager) GenerateSAToken(ctx context.Context, user *UserContext, ttl time.Duration) string {

	userTier, err := m.getUserTier(ctx, user)
	if err != nil {
		log.Printf("Failed to determine user userTier for %s: %v", user.Username, err)
		return ""
	}

	namespace, errNs := m.ensureTierNamespace(ctx, userTier)
	if errNs != nil {
		log.Printf("Failed to ensure tier namespace for userTier %s: %v", userTier, err)
		return ""
	}

	saName, errSA := m.ensureServiceAccount(ctx, namespace, user.Username, "")
	if errSA != nil {
		log.Printf("Failed to ensure service account for user %s in namespace %s: %v", user.Username, namespace, err)
		return ""
	}

	token, errToken := m.createServiceAccountToken(ctx, namespace, saName, int(ttl.Seconds()))
	if errToken != nil {
		log.Printf("Failed to create token for service account %s in namespace %s: %v", saName, namespace, err)
		return ""
	}

	return token
}

// RevokeUserTokens revokes all tokens for a user by recreating their Service Account
func (m *Manager) RevokeUserTokens(ctx context.Context, user *UserContext) error {
	userTier, err := m.getUserTier(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to determine user userTier for %s: %w", user.Username, err)
	}

	namespace := m.tierMapper.Namespaces(ctx)[userTier]

	saName := m.sanitizeServiceAccountName(user.Username)

	_, err = m.serviceAccountLister.ServiceAccounts(namespace).Get(saName)
	if errors.IsNotFound(err) {
		log.Printf("Service account %s not found in namespace %s, nothing to revoke", saName, namespace)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to check service account %s in namespace %s: %w", saName, namespace, err)
	}

	err = m.deleteServiceAccount(ctx, namespace, saName)
	if err != nil {
		return fmt.Errorf("failed to delete service account %s in namespace %s: %w", saName, namespace, err)
	}

	_, err = m.ensureServiceAccount(ctx, namespace, user.Username, userTier)
	if err != nil {
		return fmt.Errorf("failed to recreate service account for user %s in namespace %s: %w", user.Username, namespace, err)
	}

	return nil
}

// getUserTier determines which tier a user belongs to based on their groups
func (m *Manager) getUserTier(ctx context.Context, user *UserContext) (string, error) {
	for _, group := range user.Groups {
		t, err := m.tierMapper.GetTierForGroups(ctx, group)
		if err == nil {
			return t, nil
		}
	}

	log.Printf("No tier found for user %s with groups %v, defaulting to 'free'", user.Username, user.Groups)
	return "free", nil
}

// ensureTierNamespace creates a tier-based namespace if it doesn't exist.
// It takes a tier name, formats it as {instance}-tier-{tier}, and returns the namespace name.
func (m *Manager) ensureTierNamespace(ctx context.Context, userTier string) (string, error) {
	namespace := m.tierMapper.Namespaces(ctx)[userTier]

	_, err := m.namespaceLister.Get(namespace)
	if err == nil {
		return namespace, nil
	}

	if !errors.IsNotFound(err) {
		return "", fmt.Errorf("failed to check namespace %s: %w", namespace, err)
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespace,
			Labels: commonLabels(m.tenantName, userTier),
		},
	}

	_, err = m.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return namespace, nil
		}
		return "", fmt.Errorf("failed to create namespace %s: %w", namespace, err)
	}

	log.Printf("Created namespace %s", namespace)
	return namespace, nil
}

// ensureServiceAccount creates a service account if it doesn't exist.
// It takes a raw username, sanitizes it for Kubernetes naming, and returns the sanitized name.
func (m *Manager) ensureServiceAccount(ctx context.Context, namespace, username, userTier string) (string, error) {
	saName := m.sanitizeServiceAccountName(username)

	_, err := m.serviceAccountLister.ServiceAccounts(namespace).Get(saName)
	if err == nil {
		return saName, nil
	}

	if !errors.IsNotFound(err) {
		return "", fmt.Errorf("failed to check service account %s in namespace %s: %w", saName, namespace, err)
	}

	labels := commonLabels(m.tenantName, userTier)
	labels["maas.opendatahub.io/username"] = username
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
			Labels:    labels,
		},
	}

	_, err = m.clientset.CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return saName, nil
		}
		return "", fmt.Errorf("failed to create service account %s in namespace %s: %w", saName, namespace, err)
	}

	log.Printf("Created service account %s in namespace %s", saName, namespace)
	return saName, nil
}

// createServiceAccountToken creates a token for the service account using TokenRequest
func (m *Manager) createServiceAccountToken(ctx context.Context, namespace, saName string, ttl int) (string, error) {
	expirationSeconds := int64(ttl)

	tokenRequest := &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			ExpirationSeconds: &expirationSeconds,
			Audiences:         []string{m.tenantName + "-sa"},
		},
	}

	result, err := m.clientset.CoreV1().ServiceAccounts(namespace).CreateToken(
		ctx, saName, tokenRequest, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create token for service account %s: %w", saName, err)
	}

	return result.Status.Token, nil
}

// deleteServiceAccount deletes a service account
func (m *Manager) deleteServiceAccount(ctx context.Context, namespace, saName string) error {
	err := m.clientset.CoreV1().ServiceAccounts(namespace).Delete(ctx, saName, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete service account %s in namespace %s: %w", saName, namespace, err)
	}

	log.Printf("Deleted service account %s in namespace %s", saName, namespace)
	return nil
}

// sanitizeServiceAccountName ensures the service account name follows Kubernetes naming conventions.
// While ideally usernames should be pre-validated, Kubernetes TokenReview can return usernames
// in various formats (OIDC emails, LDAP DNs, etc.) that need sanitization for use as SA names.
func (m *Manager) sanitizeServiceAccountName(username string) string {
	// Kubernetes service account names must be valid DNS subdomain names
	name := strings.ToLower(username)
	name = strings.ReplaceAll(name, "@", "-at-")
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.ReplaceAll(name, "_", "-")

	name = strings.Trim(name, "-")

	if len(name) > 63 {
		name = name[:63]
		name = strings.TrimSuffix(name, "-")
	}

	return name
}

func commonLabels(name string, t string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/component":  "token-issuer",
		"app.kubernetes.io/part-of":    "maas-api",
		"maas.opendatahub.io/instance": name,
		"maas.opendatahub.io/tier":     t,
	}
}
