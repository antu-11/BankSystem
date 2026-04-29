// Package auth provides JWT token generation, validation, and Redis-backed
// blacklist management for OmniLedger.
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// TokenTTL is the lifetime of an issued JWT.
	TokenTTL = 24 * time.Hour

	// blacklistPrefix namespaces blacklisted JTI keys in Redis.
	blacklistPrefix = "vault:blacklist:"
)

// Claims extends the standard JWT claims with Vault-specific fields.
type Claims struct {
	jwt.RegisteredClaims
	UserID       uuid.UUID `json:"user_id"`
	Username     string    `json:"username"`
	IsSystemUser bool      `json:"is_system_user"`
}

// TokenManager handles JWT creation, parsing, and blacklisting.
type TokenManager struct {
	secret []byte
	rdb    *redis.Client
}

// NewTokenManager returns a TokenManager using the given HMAC secret and Redis client.
func NewTokenManager(secret string, rdb *redis.Client) *TokenManager {
	return &TokenManager{
		secret: []byte(secret),
		rdb:    rdb,
	}
}

// IssueToken generates a signed JWT for the given user.
// Returns the signed token string and the JTI (for later blacklisting).
func (tm *TokenManager) IssueToken(userID uuid.UUID, username string, isSystemUser bool) (string, string, error) {
	jti := uuid.New().String()
	now := time.Now()

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Issuer:    "omniledger",
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		},
		UserID:       userID,
		Username:     username,
		IsSystemUser: isSystemUser,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(tm.secret)
	if err != nil {
		return "", "", fmt.Errorf("sign token: %w", err)
	}
	return signed, jti, nil
}

// ValidateToken parses and validates a JWT string, returning the claims if valid.
func (tm *TokenManager) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return tm.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// BlacklistToken stores the JTI in Redis so it is rejected on future requests.
// TTL is set to the remaining lifetime of the token (floor of 1 second).
func (tm *TokenManager) BlacklistToken(ctx context.Context, jti string, expiry time.Time) error {
	remaining := time.Until(expiry)
	if remaining <= 0 {
		remaining = time.Second
	}

	key := blacklistPrefix + jti
	if err := tm.rdb.Set(ctx, key, "blacklisted", remaining).Err(); err != nil {
		return fmt.Errorf("blacklist token: %w", err)
	}
	return nil
}

// IsBlacklisted reports whether a JTI has been revoked.
func (tm *TokenManager) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	key := blacklistPrefix + jti
	exists, err := tm.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("check blacklist: %w", err)
	}
	return exists > 0, nil
}
