package upstreamauth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
)

type FieldKind string

const (
	KindText     FieldKind = "text"
	KindPassword FieldKind = "password"
	KindSelect   FieldKind = "select"
	KindNumber   FieldKind = "number"
	KindSwitch   FieldKind = "switch"
	KindTextarea FieldKind = "textarea"
)

// FieldSchema defines an input field for the frontend schema-driven form.
type FieldSchema struct {
	Key          string    `json:"key"`
	Label        string    `json:"label"`
	Kind         FieldKind `json:"kind"`
	Options      []string  `json:"options,omitempty"`
	Secret       bool      `json:"secret"`
	Required     bool      `json:"required"`
	DefaultValue any       `json:"default_value,omitempty"`
	Description  string    `json:"description,omitempty"`
}

// AuthTypeSchema defines the schema for a supported upstream authentication strategy.
type AuthTypeSchema struct {
	Type        string        `json:"type"`
	Label       string        `json:"label"`
	Description string        `json:"description"`
	Fields      []FieldSchema `json:"fields"`
}

// UpstreamAuthConfig holds the configured authentication type, non-sensitive params, and encrypted secrets.
type UpstreamAuthConfig struct {
	Type       string            `json:"type"`                 // "none", "api_key", "bearer", "basic", etc.
	Params     map[string]any    `json:"params,omitempty"`     // Non-sensitive configuration, echoed
	Secrets    map[string]string `json:"secrets,omitempty"`    // Write-only secrets, never echoed in responses
	SecretEnc  []byte            `json:"secret_enc,omitempty"` // AES-GCM encrypted ciphertext
	KeyVersion int               `json:"key_version"`
	HasSecret  bool              `json:"has_secret"`           // Indicator whether secrets are configured
	UpdatedAt  int64             `json:"updated_at,omitempty"`
}

// Authenticator defines the interface for an upstream auth strategy.
type Authenticator interface {
	Type() string
	Schema() AuthTypeSchema
	Validate(params map[string]any, secrets map[string]string) error
	Apply(ctx context.Context, req *http.Request, params map[string]any, secrets map[string]string) error
}

var (
	registryLock sync.RWMutex
	registry     = map[string]Authenticator{}
)

// Register registers an authenticator into the global registry.
func Register(a Authenticator) {
	registryLock.Lock()
	defer registryLock.Unlock()
	registry[a.Type()] = a
}

// Get returns the registered authenticator for the given authType.
func Get(authType string) (Authenticator, bool) {
	registryLock.RLock()
	defer registryLock.RUnlock()
	a, ok := registry[authType]
	return a, ok
}

// ListSchemas returns all registered schemas in a consistent priority order.
func ListSchemas() []AuthTypeSchema {
	registryLock.RLock()
	defer registryLock.RUnlock()
	order := []string{
		"none",
		"api_key",
		"bearer",
		"basic",
		"oauth2_client_credentials",
		"hmac_sign",
		"custom_template",
	}
	schemas := make([]AuthTypeSchema, 0, len(registry))
	for _, t := range order {
		if a, ok := registry[t]; ok {
			schemas = append(schemas, a.Schema())
		}
	}
	for t, a := range registry {
		found := false
		for _, o := range order {
			if o == t {
				found = true
				break
			}
		}
		if !found {
			schemas = append(schemas, a.Schema())
		}
	}
	return schemas
}

// ----------------------------------------------------------------------------
// AES-256-GCM Envelope Encryption for Secrets
// ----------------------------------------------------------------------------

var defaultMasterKey = []byte("agentpay-upstream-auth-key-32b!!") // exactly 32 bytes

// EncryptSecrets encrypts the secrets map with AES-256-GCM using masterKey.
func EncryptSecrets(secrets map[string]string, key []byte) ([]byte, error) {
	if len(key) == 0 {
		key = defaultMasterKey
	}
	if len(key) != 32 {
		return nil, errors.New("master key must be exactly 32 bytes for AES-256")
	}
	data, err := json.Marshal(secrets)
	if err != nil {
		return nil, fmt.Errorf("marshal secrets: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// DecryptSecrets decrypts the AES-256-GCM ciphertext back into the secrets map.
func DecryptSecrets(ciphertext []byte, key []byte) (map[string]string, error) {
	if len(ciphertext) == 0 {
		return map[string]string{}, nil
	}
	if len(key) == 0 {
		key = defaultMasterKey
	}
	if len(key) != 32 {
		return nil, errors.New("master key must be exactly 32 bytes for AES-256")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, cipherData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt secrets: %w", err)
	}

	var secrets map[string]string
	if err := json.Unmarshal(plaintext, &secrets); err != nil {
		return nil, fmt.Errorf("unmarshal secrets: %w", err)
	}
	return secrets, nil
}
