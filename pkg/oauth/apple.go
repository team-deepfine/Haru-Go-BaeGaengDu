package oauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/daewon/haru/pkg/applejwks"
	"github.com/golang-jwt/jwt/v5"
)

const (
	appleTokenURL  = "https://appleid.apple.com/auth/token"
	appleRevokeURL = "https://appleid.apple.com/auth/revoke"
)

// AppleUserInfo holds user information extracted from Apple's token response.
type AppleUserInfo struct {
	Sub   string  // Apple user ID
	Email *string // user email (may be nil after first login)
}

// AppleClient exchanges Apple authorization codes for user information.
type AppleClient struct {
	clientID     string
	teamID       string
	keyID        string
	privateKey   *ecdsa.PrivateKey
	redirectURI  string
	tokenURL     string
	revokeURL    string
	httpClient   *http.Client
	jwksVerifier *applejwks.Verifier
	skipJWKS     bool // for testing only
}

// NewAppleClient creates a new Apple OAuth client.
// privateKeyPEM is the PEM-encoded ES256 private key from Apple Developer.
func NewAppleClient(clientID, teamID, keyID, privateKeyPEM, redirectURI string) (*AppleClient, error) {
	privKey, err := parseECPrivateKey(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse apple private key: %w", err)
	}

	// Initialize JWKS verifier for id_token signature verification.
	// Non-fatal if it fails — will retry on first verification.
	jwksVerifier, _ := applejwks.NewVerifier()

	return &AppleClient{
		clientID:     clientID,
		teamID:       teamID,
		keyID:        keyID,
		privateKey:   privKey,
		redirectURI:  redirectURI,
		tokenURL:     appleTokenURL,
		revokeURL:    appleRevokeURL,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		jwksVerifier: jwksVerifier,
	}, nil
}

// SetTokenURL overrides the Apple token endpoint URL (for testing).
func (a *AppleClient) SetTokenURL(url string) {
	a.tokenURL = url
}

// SetRevokeURL overrides the Apple revoke endpoint URL (for testing).
func (a *AppleClient) SetRevokeURL(url string) {
	a.revokeURL = url
}

// SetSkipJWKS disables JWKS signature verification (for testing only).
func (a *AppleClient) SetSkipJWKS(skip bool) {
	a.skipJWKS = skip
}

// ExchangeAndGetUser exchanges an authorization code for an Apple access token,
// then extracts user info from the returned id_token.
func (a *AppleClient) ExchangeAndGetUser(ctx context.Context, code string) (*AppleUserInfo, error) {
	clientSecret, err := a.generateClientSecret()
	if err != nil {
		return nil, fmt.Errorf("generate client secret: %w", err)
	}

	tokenResp, err := a.exchangeCode(ctx, code, clientSecret)
	if err != nil {
		return nil, err
	}

	return a.decodeIDToken(tokenResp.IDToken)
}

// generateClientSecret creates a JWT client_secret signed with ES256.
func (a *AppleClient) generateClientSecret() (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    a.teamID,
		Subject:   a.clientID,
		Audience:  jwt.ClaimStrings{"https://appleid.apple.com"},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = a.keyID

	return token.SignedString(a.privateKey)
}

// exchangeCode posts the authorization code to Apple's token endpoint.
func (a *AppleClient) exchangeCode(ctx context.Context, code, clientSecret string) (*appleTokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {a.clientID},
		"client_secret": {clientSecret},
		"code":          {code},
	}
	if a.redirectURI != "" {
		data.Set("redirect_uri", a.redirectURI)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("apple token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errBody); err == nil {
			return nil, fmt.Errorf("apple token endpoint returned status %d: %v", resp.StatusCode, errBody)
		}
		return nil, fmt.Errorf("apple token endpoint returned status %d", resp.StatusCode)
	}

	var tokenResp appleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	if tokenResp.IDToken == "" {
		return nil, fmt.Errorf("empty id_token in apple response")
	}

	return &tokenResp, nil
}

// RevokeByAuthCode exchanges the authorization code for tokens, then revokes
// the refresh token via Apple's revoke endpoint. Used during account deletion.
func (a *AppleClient) RevokeByAuthCode(ctx context.Context, code string) error {
	clientSecret, err := a.generateClientSecret()
	if err != nil {
		return fmt.Errorf("generate client secret: %w", err)
	}

	tokenResp, err := a.exchangeCode(ctx, code, clientSecret)
	if err != nil {
		return fmt.Errorf("exchange code for revoke: %w", err)
	}

	if tokenResp.RefreshToken == "" {
		return fmt.Errorf("apple did not return a refresh token")
	}

	return a.revokeToken(ctx, tokenResp.RefreshToken, clientSecret)
}

// revokeToken calls Apple's token revoke endpoint.
func (a *AppleClient) revokeToken(ctx context.Context, token, clientSecret string) error {
	data := url.Values{
		"client_id":       {a.clientID},
		"client_secret":   {clientSecret},
		"token":           {token},
		"token_type_hint": {"refresh_token"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.revokeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("apple revoke request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errBody); err == nil {
			return fmt.Errorf("apple revoke endpoint returned status %d: %v", resp.StatusCode, errBody)
		}
		return fmt.Errorf("apple revoke endpoint returned status %d", resp.StatusCode)
	}

	return nil
}

// decodeIDToken verifies the id_token signature against Apple's JWKS and extracts claims.
func (a *AppleClient) decodeIDToken(idToken string) (*AppleUserInfo, error) {
	var claims jwt.MapClaims

	if a.skipJWKS {
		// Testing mode: parse without signature verification
		parser := jwt.NewParser()
		token, _, err := parser.ParseUnverified(idToken, jwt.MapClaims{})
		if err != nil {
			return nil, fmt.Errorf("parse id_token: %w", err)
		}
		var ok bool
		claims, ok = token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, fmt.Errorf("invalid token claims")
		}
	} else {
		if a.jwksVerifier == nil {
			v, err := applejwks.NewVerifier()
			if err != nil {
				return nil, fmt.Errorf("apple jwks unavailable: %w", err)
			}
			a.jwksVerifier = v
		}

		var err error
		claims, err = a.jwksVerifier.VerifyAndParse(idToken)
		if err != nil {
			return nil, fmt.Errorf("verify id_token: %w", err)
		}
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return nil, fmt.Errorf("missing sub claim in id_token")
	}

	info := &AppleUserInfo{Sub: sub}
	if email, ok := claims["email"].(string); ok && email != "" {
		info.Email = &email
	}

	return info, nil
}

// parseECPrivateKey parses a PEM-encoded EC private key.
// Handles escaped newlines (\n) commonly found in environment variables.
func parseECPrivateKey(pemStr string) (*ecdsa.PrivateKey, error) {
	pemStr = strings.ReplaceAll(pemStr, `\n`, "\n")

	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		ecKey, ecErr := x509.ParseECPrivateKey(block.Bytes)
		if ecErr != nil {
			return nil, fmt.Errorf("parse private key (PKCS8: %v, EC: %v)", err, ecErr)
		}
		return ecKey, nil
	}

	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not ECDSA")
	}

	return ecKey, nil
}

// Apple API response types (unexported).

type appleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}
