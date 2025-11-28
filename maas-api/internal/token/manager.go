package token

import (
	"context"
	"crypto/sha256" //nolint:gosec // SHA1 used for non-cryptographic hashing of usernames, not for security
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/opendatahub-io/maas-billing/maas-api/internal/tier"
)

type Manager struct {
	tenantName           string
	tierMapper           *tier.Mapper
	clientset            kubernetes.Interface
	namespaceLister      corelistersv1.NamespaceLister
	serviceAccountLister corelistersv1.ServiceAccountLister
	store                *Store
}

func NewManager(
	tenantName string,
	tierMapper *tier.Mapper,
	clientset kubernetes.Interface,
	namespaceLister corelistersv1.NamespaceLister,
	serviceAccountLister corelistersv1.ServiceAccountLister,
	store *Store,
) *Manager {
	return &Manager{
		tenantName:           tenantName,
		tierMapper:           tierMapper,
		clientset:            clientset,
		namespaceLister:      namespaceLister,
		serviceAccountLister: serviceAccountLister,
		store:                store,
	}
}

// GenerateToken creates a Service Account token in the namespace bound to the tier the user belongs to.
// The name parameter is optional - if provided, the token is tracked in the database for individual revocation
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

	claims, err := extractClaims(token.Status.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to extract claims from new token: %w", err)
	}
	jti, ok := claims["jti"].(string)
	if !ok {
		return nil, fmt.Errorf("jti claim not found or not a string in new token")
	}
	expFloat, ok := claims["exp"].(float64)
	if !ok {
		return nil, fmt.Errorf("exp claim not found or not a number in new token")
	}
	exp := int64(expFloat)

	result := &Token{
		Token:      token.Status.Token,
		Expiration: Duration{expiration},
		ExpiresAt:  exp,
	}

	// If name is provided, add to the database for tracking
	if name != "" {
		if err := m.store.AddTokenMetadata(ctx, namespace, user.Username, name, jti, result.ExpiresAt); err != nil {
			log.Printf("Failed to update metadata for token %s: %v", name, err)
			// Log error but don't fail token generation
		}
	}

	return result, nil
}

// RevokeTokens revokes all tokens for a user by recreating their Service Account.
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
		// Still try to delete token metadata even if SA doesn't exist
		if err := m.store.DeleteTokensForUser(ctx, namespace, user.Username); err != nil {
			log.Printf("Failed to delete tokens for user %s: %v", user.Username, err)
		}
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to check service account %s in namespace %s: %w", saName, namespace, err)
	}

	// Delete all token metadata before revoking tokens
	if err := m.store.DeleteTokensForUser(ctx, namespace, user.Username); err != nil {
		log.Printf("Failed to delete tokens for user %s: %v", user.Username, err)
		// Don't fail the revocation if metadata deletion fails
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

// ValidateToken verifies the token with K8s
func (m *Manager) ValidateToken(ctx context.Context, token string, reviewer *Reviewer) (*UserContext, error) {
	// 1. Check K8s validity
	userCtx, err := reviewer.ExtractUserInfo(ctx, token)
	if err != nil {
		log.Printf("TokenReview error: %v", err)
		return nil, err
	}

	if !userCtx.IsAuthenticated {
		log.Printf("TokenReview returned IsAuthenticated=false, username: '%s'", userCtx.Username)
		return userCtx, nil
	}

	log.Printf("TokenReview successful for user: %s", userCtx.Username)

	// 2. Check user type
	// If it is a User token (not SA), we should allow it (Bootstrap/Admin access)
	if !strings.HasPrefix(userCtx.Username, "system:serviceaccount:") {
		log.Printf("Allowing non-SA token for user: %s", userCtx.Username)
		return userCtx, nil
	}

	log.Printf("Token validation successful for user: %s", userCtx.Username)
	return userCtx, nil
}

// GetTokens returns all tokens for a user
func (m *Manager) GetTokens(ctx context.Context, user *UserContext) ([]NamedToken, error) {
	return m.store.GetTokensForUser(ctx, user.Username)
}

// GetToken returns a single token by its JTI
func (m *Manager) GetToken(ctx context.Context, user *UserContext, jti string) (*NamedToken, error) {
	return m.store.GetToken(ctx, user.Username, jti)
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

// createServiceAccountToken creates a token for the service account using TokenRequest.
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

// deleteServiceAccount deletes a service account.
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
	sum := sha256.Sum256([]byte(username)) //nolint:gosec // SHA1 used for non-cryptographic hashing, not for security
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
