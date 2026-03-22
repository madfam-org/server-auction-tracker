package auth

import (
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims from Janua.
type Claims struct {
	Email string   `json:"email"`
	Name  string   `json:"name"`
	Roles []string `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

// Validator validates Janua JWTs and checks authorization.
type Validator struct {
	jwks           *JWKSFetcher
	expectedIssuer string
	allowedDomains []string
	allowedRoles   []string
}

// NewValidator creates a JWT validator backed by the given JWKS fetcher.
func NewValidator(jwks *JWKSFetcher, issuer string, allowedDomains, allowedRoles []string) *Validator {
	return &Validator{
		jwks:           jwks,
		expectedIssuer: issuer,
		allowedDomains: allowedDomains,
		allowedRoles:   allowedRoles,
	}
}

// ValidateToken parses and validates a JWT string, returning the claims.
func (v *Validator) ValidateToken(tokenString string) (*Claims, error) {
	// Parse header to extract kid
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.expectedIssuer),
		jwt.WithExpirationRequired(),
	)

	claims := &Claims{}
	token, err := parser.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		return v.jwks.GetPublicKey(kid)
	})
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	return claims, nil
}

// IsAuthorized performs dual-gate authorization: email domain AND role.
func (v *Validator) IsAuthorized(claims *Claims) bool {
	if claims == nil || claims.Email == "" {
		return false
	}

	domainOK := false
	emailLower := strings.ToLower(claims.Email)
	for _, domain := range v.allowedDomains {
		if strings.HasSuffix(emailLower, strings.ToLower(domain)) {
			domainOK = true
			break
		}
	}
	if !domainOK {
		return false
	}

	if len(v.allowedRoles) == 0 {
		return true
	}

	for _, role := range claims.Roles {
		for _, allowed := range v.allowedRoles {
			if strings.EqualFold(role, allowed) {
				return true
			}
		}
	}

	return false
}
