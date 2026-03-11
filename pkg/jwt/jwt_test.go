package jwt

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTokenPair(t *testing.T) {
	tests := []struct {
		name          string
		accessExpiry  time.Duration
		refreshExpiry time.Duration
	}{
		{
			name:          "standard expiry durations",
			accessExpiry:  15 * time.Minute,
			refreshExpiry: 7 * 24 * time.Hour,
		},
		{
			name:          "short expiry durations",
			accessExpiry:  1 * time.Minute,
			refreshExpiry: 10 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager("test-secret-key", tt.accessExpiry, tt.refreshExpiry)
			userID := uuid.New()

			pair, err := m.GenerateTokenPair(userID)

			require.NoError(t, err)
			assert.NotEmpty(t, pair.AccessToken)
			assert.NotEmpty(t, pair.RefreshToken)
			assert.NotEqual(t, pair.AccessToken, pair.RefreshToken)
			assert.Equal(t, int(tt.accessExpiry.Seconds()), pair.ExpiresIn)
		})
	}
}

func TestValidateToken_ValidAccessToken(t *testing.T) {
	m := NewManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	pair, err := m.GenerateTokenPair(userID)
	require.NoError(t, err)

	gotUserID, err := m.ValidateToken(pair.AccessToken)

	assert.NoError(t, err)
	assert.Equal(t, userID, gotUserID)
}

func TestValidateToken_ValidRefreshToken(t *testing.T) {
	m := NewManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	pair, err := m.GenerateTokenPair(userID)
	require.NoError(t, err)

	gotUserID, err := m.ValidateToken(pair.RefreshToken)

	assert.NoError(t, err)
	assert.Equal(t, userID, gotUserID)
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	// Use a very short expiry so the token expires immediately.
	// The leeway is 30s, so we set expiry to 1ms and sleep briefly,
	// then manually craft a token that is well past leeway.
	m := NewManager("test-secret-key", 1*time.Millisecond, 1*time.Millisecond)
	userID := uuid.New()

	// Directly create an already-expired token using the internal method approach.
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-31 * time.Second)), // expired beyond 30s leeway
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-secret-key"))
	require.NoError(t, err)

	_, err = m.ValidateToken(tokenString)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse token")
}

func TestValidateToken_TamperedToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "completely malformed string",
			token: "not-a-jwt-token",
		},
		{
			name:  "partial jwt format",
			token: "eyJhbGciOiJIUzI1NiJ9.tampered.payload",
		},
		{
			name:  "modified signature",
			token: func() string {
				m := NewManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)
				pair, _ := m.GenerateTokenPair(uuid.New())
				// Flip the last character of the token to tamper with the signature
				tokenBytes := []byte(pair.AccessToken)
				if tokenBytes[len(tokenBytes)-1] == 'A' {
					tokenBytes[len(tokenBytes)-1] = 'B'
				} else {
					tokenBytes[len(tokenBytes)-1] = 'A'
				}
				return string(tokenBytes)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)

			_, err := m.ValidateToken(tt.token)

			assert.Error(t, err)
		})
	}
}

func TestValidateToken_WrongSigningMethod(t *testing.T) {
	userID := uuid.New()

	// Create a token with "none" signing method
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	m := NewManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)

	_, err = m.ValidateToken(tokenString)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signing method")
}

func TestValidateToken_EmptyString(t *testing.T) {
	m := NewManager("test-secret-key", 15*time.Minute, 7*24*time.Hour)

	_, err := m.ValidateToken("")

	assert.Error(t, err)
}

func TestValidateToken_WrongSecret(t *testing.T) {
	generator := NewManager("secret-one", 15*time.Minute, 7*24*time.Hour)
	validator := NewManager("secret-two", 15*time.Minute, 7*24*time.Hour)
	userID := uuid.New()

	pair, err := generator.GenerateTokenPair(userID)
	require.NoError(t, err)

	_, err = validator.ValidateToken(pair.AccessToken)

	assert.Error(t, err)
}
