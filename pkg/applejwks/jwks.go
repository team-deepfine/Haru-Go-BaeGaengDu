package applejwks

import (
	"context"
	"fmt"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

const appleJWKSURL = "https://appleid.apple.com/auth/keys"

// Verifier verifies JWTs signed by Apple using their public JWKS.
type Verifier struct {
	jwks keyfunc.Keyfunc
}

// NewVerifier creates a new Apple JWKS verifier by fetching Apple's public keys.
func NewVerifier() (*Verifier, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{appleJWKSURL})
	if err != nil {
		return nil, fmt.Errorf("fetch apple jwks: %w", err)
	}

	return &Verifier{jwks: jwks}, nil
}

// VerifyAndParse verifies the JWT signature against Apple's JWKS and returns the claims.
func (v *Verifier) VerifyAndParse(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, v.jwks.KeyfuncCtx(context.Background()),
		jwt.WithValidMethods([]string{"RS256", "ES256"}),
	)
	if err != nil {
		return nil, fmt.Errorf("verify jwt signature: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
