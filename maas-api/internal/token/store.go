package token

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NamedToken represents metadata for a single token
type NamedToken struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	CreationDate   string `json:"creationDate"`
	ExpirationDate string `json:"expirationDate"`
	Status         string `json:"status"` // "active", "expired"
	ExpiredAt      string `json:"expiredAt,omitempty"`
}

// UserTokenData represents the JSON structure stored in the user's secret
type UserTokenData struct {
	Username string       `json:"username"`
	Tokens   []NamedToken `json:"tokens"`
}

// Store handles the persistence of token metadata
type Store struct {
	clientset  kubernetes.Interface
	tenantName string
}

// NewStore creates a new TokenStore
func NewStore(clientset kubernetes.Interface, tenantName string) *Store {
	return &Store{
		clientset:  clientset,
		tenantName: tenantName,
	}
}

// AddTokenMetadata adds a new token to the user's metadata secret
func (s *Store) AddTokenMetadata(ctx context.Context, namespace, username, tokenName string, expiresAt int64) error {
	secretName, err := s.getUserSecretName(username)
	if err != nil {
		return fmt.Errorf("failed to generate secret name: %w", err)
	}

	// Retry loop for optimistic locking
	for i := 0; i < 3; i++ {
		secret, err := s.clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
		
		if errors.IsNotFound(err) {
			// Create new secret
			return s.createNewUserSecret(ctx, namespace, secretName, username, tokenName, expiresAt)
		} else if err != nil {
			return fmt.Errorf("failed to get secret %s: %w", secretName, err)
		}

		// Update existing secret
		if err := s.updateExistingUserSecret(ctx, secret, username, tokenName, expiresAt); err != nil {
			if errors.IsConflict(err) {
				continue // Retry on conflict
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("failed to update token metadata after retries")
}

// createNewUserSecret creates a new secret with the first token
func (s *Store) createNewUserSecret(ctx context.Context, namespace, secretName, username, tokenName string, expiresAt int64) error {
	newToken := s.createNamedTokenObj(tokenName, expiresAt)
	
	userData := UserTokenData{
		Username: username,
		Tokens:   []NamedToken{newToken},
	}

	jsonData, err := json.Marshal(userData)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %w", err)
	}

	// Extract tier from namespace for labels if possible
	tier := "unknown"
	if strings.HasPrefix(namespace, s.tenantName+"-tier-") {
		tier = strings.TrimPrefix(namespace, s.tenantName+"-tier-")
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/component":      "token-metadata",
				"app.kubernetes.io/part-of":        "maas-api",
				"maas.opendatahub.io/instance":     s.tenantName,
				"maas.opendatahub.io/tier":         tier,
				"maas.opendatahub.io/token-owner":  username,
				"maas.opendatahub.io/token-secret": "true",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tokens.json": jsonData,
		},
	}

	_, err = s.clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	return err
}

// updateExistingUserSecret appends a token to an existing secret
func (s *Store) updateExistingUserSecret(ctx context.Context, secret *corev1.Secret, username, tokenName string, expiresAt int64) error {
	var userData UserTokenData
	
	if data, ok := secret.Data["tokens.json"]; ok {
		if err := json.Unmarshal(data, &userData); err != nil {
			log.Printf("Warning: failed to unmarshal existing token data for %s, resetting: %v", username, err)
			userData = UserTokenData{Username: username}
		}
	} else {
		userData = UserTokenData{Username: username}
	}

	// Add new token
	newToken := s.createNamedTokenObj(tokenName, expiresAt)
	userData.Tokens = append(userData.Tokens, newToken)

	jsonData, err := json.Marshal(userData)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %w", err)
	}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data["tokens.json"] = jsonData

	_, err = s.clientset.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

// MarkTokensAsExpired marks all active tokens for a user as expired
func (s *Store) MarkTokensAsExpired(ctx context.Context, namespace, username string) error {
	secretName, err := s.getUserSecretName(username)
	if err != nil {
		return fmt.Errorf("failed to generate secret name: %w", err)
	}

	secret, err := s.clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// No metadata secret found, nothing to do
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	var userData UserTokenData
	if data, ok := secret.Data["tokens.json"]; ok {
		if err := json.Unmarshal(data, &userData); err != nil {
			return fmt.Errorf("failed to unmarshal user data: %w", err)
		}
	} else {
		return nil // Empty data
	}

	updated := false
	now := time.Now().Format(time.RFC3339)
	
	for i := range userData.Tokens {
		if userData.Tokens[i].Status == "active" {
			userData.Tokens[i].Status = "expired"
			userData.Tokens[i].ExpiredAt = now
			updated = true
		}
	}

	if !updated {
		return nil
	}

	jsonData, err := json.Marshal(userData)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %w", err)
	}

	secret.Data["tokens.json"] = jsonData
	_, err = s.clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

func (s *Store) createNamedTokenObj(name string, expiresAt int64) NamedToken {
	creationTime := time.Now()
	return NamedToken{
		ID:             s.generateTokenID(name, creationTime),
		Name:           name,
		CreationDate:   creationTime.Format(time.RFC3339),
		ExpirationDate: time.Unix(expiresAt, 0).Format(time.RFC3339),
		Status:         "active",
	}
}

func (s *Store) generateTokenID(name string, t time.Time) string {
	// Create a unique ID for the token entry
	data := fmt.Sprintf("%s-%d", name, t.UnixNano())
	sum := sha1.Sum([]byte(data))
	return hex.EncodeToString(sum[:])[:12]
}

func (s *Store) getUserSecretName(username string) (string, error) {
	// Sanitize username
	name := strings.ToLower(username)
	reInvalid := regexp.MustCompile(`[^a-z0-9-]+`)
	name = reInvalid.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	
	if name == "" {
		return "", fmt.Errorf("invalid username %q", username)
	}

	// Add hash to ensure uniqueness and valid length
	sum := sha1.Sum([]byte(username))
	suffix := hex.EncodeToString(sum[:])[:8]
	
	// Format: maas-tokens-{sanitized-user}-{hash}
	prefix := "maas-tokens-"
	combined := prefix + name + "-" + suffix

	if len(combined) > 63 {
		// Truncate middle if too long, keeping prefix and suffix
		maxBase := 63 - len(suffix) - 1
		if maxBase > len(prefix) {
			combined = combined[:maxBase] + "-" + suffix
		}
	}
	
	return combined, nil
}

