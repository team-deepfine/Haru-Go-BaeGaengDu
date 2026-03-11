package oauth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const appleKeysURL = "https://appleid.apple.com/auth/keys"
const appleIssuer = "https://appleid.apple.com"

// AppleIDTokenClaims holds the verified claims from an Apple id_token.
type AppleIDTokenClaims struct {
	Sub   string  // Apple user ID
	Email *string // user email (may be nil after first login)
}

// AppleVerifier verifies Apple Sign In id_tokens using Apple's public keys.
type AppleVerifier struct {
	clientID   string
	mu         sync.RWMutex
	keys       map[string]*rsa.PublicKey
	fetchedAt  time.Time
	cacheTTL   time.Duration
	httpClient *http.Client
}

// NewAppleVerifier creates a new Apple id_token verifier.
func NewAppleVerifier(clientID string) *AppleVerifier {
	return &AppleVerifier{
		clientID:   clientID,
		keys:       make(map[string]*rsa.PublicKey),
		cacheTTL:   24 * time.Hour,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Verify validates an Apple id_token JWT and returns the extracted claims.
func (v *AppleVerifier) Verify(ctx context.Context, idToken string) (*AppleIDTokenClaims, error) {
	// Parse the token header to get the kid
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(idToken, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("parse id_token header: %w", err)
	}

	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("missing kid in token header")
	}

	// Get the public key for this kid
	pubKey, err := v.getPublicKey(ctx, kid)
	if err != nil {
		return nil, fmt.Errorf("get apple public key: %w", err)
	}

	// Verify the token
	verified, err := jwt.Parse(idToken, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return pubKey, nil
	},
		jwt.WithIssuer(appleIssuer),
		jwt.WithAudience(v.clientID),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("verify id_token: %w", err)
	}

	claims, ok := verified.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return nil, fmt.Errorf("missing sub claim")
	}

	result := &AppleIDTokenClaims{Sub: sub}
	if email, ok := claims["email"].(string); ok && email != "" {
		result.Email = &email
	}

	return result, nil
}

func (v *AppleVerifier) getPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// Try cache first
	v.mu.RLock()
	key, found := v.keys[kid]
	expired := time.Since(v.fetchedAt) > v.cacheTTL
	v.mu.RUnlock()

	if found && !expired {
		return key, nil
	}

	// Fetch fresh keys
	if err := v.fetchKeys(ctx); err != nil {
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	key, found = v.keys[kid]
	if !found {
		return nil, fmt.Errorf("apple public key not found for kid: %s", kid)
	}
	return key, nil
}

type appleJWKSet struct {
	Keys []appleJWK `json:"keys"`
}

type appleJWK struct {
	KTY string `json:"kty"`
	KID string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (v *AppleVerifier) fetchKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, appleKeysURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch apple keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("apple keys endpoint returned status %d", resp.StatusCode)
	}

	var jwkSet appleJWKSet
	if err := json.NewDecoder(resp.Body).Decode(&jwkSet); err != nil {
		return fmt.Errorf("decode apple keys: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwkSet.Keys))
	for _, jwk := range jwkSet.Keys {
		if jwk.KTY != "RSA" {
			continue
		}
		pubKey, err := jwkToRSAPublicKey(jwk)
		if err != nil {
			continue
		}
		keys[jwk.KID] = pubKey
	}

	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = time.Now()
	v.mu.Unlock()

	return nil
}

func jwkToRSAPublicKey(jwk appleJWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}
