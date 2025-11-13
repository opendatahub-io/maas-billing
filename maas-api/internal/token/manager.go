package token

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
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

// GenerateToken creates a Service Account token in the namespace bound to the tier the user belongs to
func (m *Manager) GenerateToken(ctx context.Context, user *UserContext, expiration time.Duration, name string) (*Token, error) {

	userTier, err := m.tierMapper.GetTierForGroups(ctx, user.Groups...)
	if err != nil {
		log.Printf("Failed to determine user tier for %s: %v", user.Username, err)
		return nil, fmt.Errorf("failed to determine user tier for %s: %w", user.Username, err)
	}

	namespace, errNs := m.ensureTierNamespace(ctx, userTier)
	if errNs != nil {
		log.Printf("Failed to ensure tier namespace for user %s: %v", userTier, errNs)
		return nil, fmt.Errorf("failed to ensure tier namespace for user %s: %w", userTier, errNs)
	}

	saName, errSA := m.ensureServiceAccount(ctx, namespace, user.Username, userTier)
	if errSA != nil {
		log.Printf("Failed to ensure service account for user %s in namespace %s: %v", user.Username, namespace, errSA)
		return nil, fmt.Errorf("failed to ensure service account for user %s in namespace %s: %w", user.Username, namespace, errSA)
	}

	token, errToken := m.createServiceAccountToken(ctx, namespace, saName, int(expiration.Seconds()))
	if errToken != nil {
		log.Printf("Failed to create token for service account %s in namespace %s: %v", saName, namespace, errToken)
		return nil, fmt.Errorf("failed to create token for service account %s in namespace %s: %w", saName, namespace, errToken)
	}

	result := &Token{
		Token:      token.Status.Token,
		Expiration: Duration{expiration},
		ExpiresAt:  token.Status.ExpirationTimestamp.Unix(),
	}

	// If name is provided, create a metadata secret to track the token
	if name != "" {
		if err := m.createTokenMetadataSecret(ctx, namespace, user.Username, name, result.ExpiresAt); err != nil {
			log.Printf("Failed to create metadata secret for token %s: %v", name, err)
			// Log error but don't fail token generation
		}
	}

	return result, nil
}

// RevokeTokens revokes all tokens for a user by recreating their Service Account
// and marking all their token metadata secrets as expired
func (m *Manager) RevokeTokens(ctx context.Context, user *UserContext) error {
	userTier, err := m.tierMapper.GetTierForGroups(ctx, user.Groups...)
	if err != nil {
		return fmt.Errorf("failed to determine user tier for %s: %w", user.Username, err)
	}

	namespace, errNS := m.tierMapper.Namespace(ctx, userTier)
	if errNS != nil {
		return fmt.Errorf("failed to determine namespace for user %s: %w", user.Username, errNS)
	}

	saName, errName := m.sanitizeServiceAccountName(user.Username)
	if errName != nil {
		return fmt.Errorf("failed to sanitize service account name for user %s: %w", user.Username, errName)
	}

	_, err = m.serviceAccountLister.ServiceAccounts(namespace).Get(saName)
	if errors.IsNotFound(err) {
		log.Printf("Service account %s not found in namespace %s, nothing to revoke", saName, namespace)
		// Still try to mark secrets as expired even if SA doesn't exist
		if err := m.markUserSecretsAsExpired(ctx, namespace, user.Username); err != nil {
			log.Printf("Failed to mark secrets as expired for user %s: %v", user.Username, err)
		}
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to check service account %s in namespace %s: %w", saName, namespace, err)
	}

	// Mark all token metadata secrets as expired before revoking tokens
	if err := m.markUserSecretsAsExpired(ctx, namespace, user.Username); err != nil {
		log.Printf("Failed to mark secrets as expired for user %s: %v", user.Username, err)
		// Don't fail the revocation if secret updates fail
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

// ensureTierNamespace creates a tier-based namespace if it doesn't exist.
// It takes a tier name, formats it as {instance}-tier-{tier}, and returns the namespace name.
func (m *Manager) ensureTierNamespace(ctx context.Context, tier string) (string, error) {
	namespace, errNs := m.tierMapper.Namespace(ctx, tier)
	if errNs != nil {
		return "", fmt.Errorf("failed to determine namespace for tier %q: %w", tier, errNs)
	}

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
			Labels: namespaceLabels(m.tenantName, tier),
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
	saName, errName := m.sanitizeServiceAccountName(username)
	if errName != nil {
		return "", fmt.Errorf("failed to sanitize service account name for user %s: %w", username, errName)
	}

	_, err := m.serviceAccountLister.ServiceAccounts(namespace).Get(saName)
	if err == nil {
		return saName, nil
	}

	if !errors.IsNotFound(err) {
		return "", fmt.Errorf("failed to check service account %s in namespace %s: %w", saName, namespace, err)
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
			Labels:    serviceAccountLabels(m.tenantName, userTier),
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
func (m *Manager) createServiceAccountToken(ctx context.Context, namespace, saName string, ttl int) (*authv1.TokenRequest, error) {
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
		return nil, fmt.Errorf("failed to create token for service account %s: %w", saName, err)
	}

	return result, nil
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

// createTokenMetadataSecret creates a Kubernetes secret to track token metadata (NOT the token itself)
func (m *Manager) createTokenMetadataSecret(ctx context.Context, namespace, username, tokenName string, expiresAt int64) error {
	// Sanitize the token name for use in a Kubernetes secret name
	sanitizedName, err := m.sanitizeSecretName(username, tokenName)
	if err != nil {
		return fmt.Errorf("failed to sanitize secret name: %w", err)
	}

	// Get the user's tier for labels
	userTier := "unknown"
	// Try to extract tier from namespace if it follows the pattern
	if strings.HasPrefix(namespace, m.tenantName+"-tier-") {
		userTier = strings.TrimPrefix(namespace, m.tenantName+"-tier-")
	}

	creationTime := time.Now().Unix()
	expirationTime := time.Unix(expiresAt, 0).Format(time.RFC3339)
	creationTimeStr := time.Unix(creationTime, 0).Format(time.RFC3339)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sanitizedName,
			Namespace: namespace,
			Labels:    tokenMetadataSecretLabels(m.tenantName, userTier, username),
			Annotations: map[string]string{
				"maas.opendatahub.io/token-name": tokenName,
			},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"username":       username,
			"creationDate":   creationTimeStr,
			"expirationDate": expirationTime,
			"name":           tokenName,
			"status":         "active",
		},
	}

	_, err = m.clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			// Update existing secret
			return m.updateTokenMetadataSecret(ctx, namespace, sanitizedName, map[string]string{
				"username":       username,
				"creationDate":   creationTimeStr,
				"expirationDate": expirationTime,
				"name":           tokenName,
				"status":         "active",
			})
		}
		return fmt.Errorf("failed to create metadata secret %s: %w", sanitizedName, err)
	}

	log.Printf("Created token metadata secret %s for user %s in namespace %s", sanitizedName, username, namespace)
	return nil
}

// updateTokenMetadataSecret updates an existing token metadata secret
func (m *Manager) updateTokenMetadataSecret(ctx context.Context, namespace, secretName string, data map[string]string) error {
	secret, err := m.clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	if secret.StringData == nil {
		secret.StringData = make(map[string]string)
	}

	for key, value := range data {
		secret.StringData[key] = value
	}

	_, err = m.clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret %s: %w", secretName, err)
	}

	log.Printf("Updated token metadata secret %s in namespace %s", secretName, namespace)
	return nil
}

// markUserSecretsAsExpired marks all token metadata secrets for a user as expired
func (m *Manager) markUserSecretsAsExpired(ctx context.Context, namespace, username string) error {
	// List all secrets in the namespace with the token-secret label and matching username
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("maas.opendatahub.io/token-secret=true,maas.opendatahub.io/token-owner=%s", username),
	}

	secrets, err := m.clientset.CoreV1().Secrets(namespace).List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("failed to list secrets for user %s: %w", username, err)
	}

	for _, secret := range secrets.Items {
		// Update the status field to "expired" and add expiredAt timestamp
		// Note: When retrieving secrets, they have Data (base64), not StringData
		// StringData is write-only and must be nil when updating
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}

		// Clear StringData (it's write-only and can interfere with updates)
		secret.StringData = nil

		secret.Data["status"] = []byte("expired")
		secret.Data["expiredAt"] = []byte(time.Now().Format(time.RFC3339))

		_, err := m.clientset.CoreV1().Secrets(namespace).Update(ctx, &secret, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("Failed to update secret %s status to expired: %v", secret.Name, err)
			continue
		}
		log.Printf("Marked secret %s as expired for user %s at %s", secret.Name, username, time.Now().Format(time.RFC3339))
	}

	return nil
}

// sanitizeSecretName creates a sanitized secret name for token metadata from username and token name
func (m *Manager) sanitizeSecretName(username, tokenName string) (string, error) {
	// Kubernetes Secret names must be valid DNS-1123 subdomain:
	// [a-z0-9-], 1-253 chars, start/end alphanumeric.
	combined := username + "-" + tokenName
	name := strings.ToLower(combined)

	// Replace any invalid runes with '-'
	reInvalid := regexp.MustCompile(`[^a-z0-9-]+`)
	name = reInvalid.ReplaceAllString(name, "-")

	// Collapse consecutive dashes
	reDash := regexp.MustCompile(`-+`)
	name = reDash.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "", fmt.Errorf("invalid username %q or token name %q", username, tokenName)
	}

	// Append a stable short hash to ensure uniqueness
	sum := sha1.Sum([]byte(combined))
	suffix := hex.EncodeToString(sum[:])[:8]

	// Prefix for identification
	prefix := "token-"

	// Ensure total length <= 63 including prefix, hyphen and suffix
	const maxLen = 63
	baseMax := maxLen - len(prefix) - 1 - len(suffix)
	if len(name) > baseMax {
		name = name[:baseMax]
		name = strings.Trim(name, "-")
	}

	return prefix + name + "-" + suffix, nil
}

// sanitizeServiceAccountName ensures the service account name follows Kubernetes naming conventions.
// While ideally usernames should be pre-validated, Kubernetes TokenReview can return usernames
// in various formats (OIDC emails, LDAP DNs, etc.) that need sanitization for use as SA names.
func (m *Manager) sanitizeServiceAccountName(username string) (string, error) {
	// Kubernetes ServiceAccount names must be valid DNS-1123 labels:
	// [a-z0-9-], 1-63 chars, start/end alphanumeric.
	name := strings.ToLower(username)

	// Replace any invalid runes with '-'
	reInvalid := regexp.MustCompile(`[^a-z0-9-]+`)
	name = reInvalid.ReplaceAllString(name, "-")

	// Collapse consecutive dashes
	reDash := regexp.MustCompile(`-+`)
	name = reDash.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "", fmt.Errorf("invalid username %q", username)
	}

	// Append a stable short hash to reduce collisions
	sum := sha1.Sum([]byte(username))
	suffix := hex.EncodeToString(sum[:])[:8]

	// Ensure total length <= 63 including hyphen and suffix
	const maxLen = 63
	baseMax := maxLen - 1 - len(suffix)
	if len(name) > baseMax {
		name = name[:baseMax]
		name = strings.Trim(name, "-")
	}

	return name + "-" + suffix, nil
}
