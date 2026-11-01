package components_test

import (
	"testing"
	"time"

	"github.com/bidirekt/broker/internal/components"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	jwtPublicURL = "http://localhost:8080"
	jwtAudience  = "http://localhost:8080/api"
	jwtUserID    = int64(42)
	jwtSubject   = "42"
)

var jwtSettings = components.Settings{PublicURL: jwtPublicURL}

func validJWTClaims() jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss": jwtPublicURL,
		"aud": jwtAudience,
		"sub": jwtSubject,
		"iat": now.Unix(),
		"exp": now.Add(15 * time.Minute).Unix(),
	}
}

func signJWT(t *testing.T, method jwt.SigningMethod, header map[string]any, claims jwt.MapClaims, key any) string {
	t.Helper()
	token := jwt.NewWithClaims(method, claims)
	token.Header = header
	token.Header["alg"] = method.Alg()

	signed, err := token.SignedString(key)
	require.NoError(t, err)

	return signed
}

func TestIssueTokenThenUserIDFromTokenReturnsTheUser(t *testing.T) {
	issuedAt := time.Now()

	token, expiresAt, err := components.IssueToken(jwtSettings, jwtUserID, issuedAt)
	require.NoError(t, err)

	userID, valid := components.UserIDFromToken(jwtSettings, token)
	assert.True(t, valid)
	assert.Equal(t, jwtUserID, userID)
	assert.Equal(t, issuedAt.Add(15*time.Minute).Unix(), expiresAt.Unix())
}

func TestIssueTokenWritesTheExpectedHeaderAndClaims(t *testing.T) {
	issuedAt := time.Now()

	token, _, err := components.IssueToken(jwtSettings, jwtUserID, issuedAt)
	require.NoError(t, err)

	claims := jwt.MapClaims{}
	parsed, _, err := jwt.NewParser().ParseUnverified(token, claims)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"alg": "HS256", "typ": "at+jwt"}, parsed.Header)
	assert.Equal(t, jwt.MapClaims{
		"iss": jwtPublicURL,
		"aud": []any{jwtAudience},
		"sub": jwtSubject,
		"iat": float64(issuedAt.Unix()),
		"exp": float64(issuedAt.Add(15 * time.Minute).Unix()),
	}, claims)
}

func TestUserIDFromTokenRefusesAnExpiredToken(t *testing.T) {
	token, _, err := components.IssueToken(jwtSettings, jwtUserID, time.Now().Add(-16*time.Minute))
	require.NoError(t, err)

	_, valid := components.UserIDFromToken(jwtSettings, token)
	assert.False(t, valid)
}

func TestUserIDFromTokenRefusesDefectiveTokens(t *testing.T) {
	typHeader := func() map[string]any { return map[string]any{"typ": "at+jwt"} }
	withClaim := func(name string, value any) jwt.MapClaims {
		claims := validJWTClaims()
		claims[name] = value
		return claims
	}
	withoutClaim := func(name string) jwt.MapClaims {
		claims := validJWTClaims()
		delete(claims, name)
		return claims
	}

	defective := map[string]string{
		"HS512":           signJWT(t, jwt.SigningMethodHS512, typHeader(), validJWTClaims(), components.TokenSigningKey),
		"none":            signJWT(t, jwt.SigningMethodNone, typHeader(), validJWTClaims(), jwt.UnsafeAllowNoneSignatureType),
		"wrong aud":       signJWT(t, jwt.SigningMethodHS256, typHeader(), withClaim("aud", jwtPublicURL), components.TokenSigningKey),
		"wrong iss":       signJWT(t, jwt.SigningMethodHS256, typHeader(), withClaim("iss", "http://localhost:9090"), components.TokenSigningKey),
		"missing typ":     signJWT(t, jwt.SigningMethodHS256, map[string]any{}, validJWTClaims(), components.TokenSigningKey),
		"wrong typ":       signJWT(t, jwt.SigningMethodHS256, map[string]any{"typ": "JWT"}, validJWTClaims(), components.TokenSigningKey),
		"missing exp":     signJWT(t, jwt.SigningMethodHS256, typHeader(), withoutClaim("exp"), components.TokenSigningKey),
		"non-numeric sub": signJWT(t, jwt.SigningMethodHS256, typHeader(), withClaim("sub", "ana"), components.TokenSigningKey),
		"padded sub":      signJWT(t, jwt.SigningMethodHS256, typHeader(), withClaim("sub", "042"), components.TokenSigningKey),
		"another key":     signJWT(t, jwt.SigningMethodHS256, typHeader(), validJWTClaims(), []byte("another-signing-key-of-32-bytes!")),
	}

	for name, token := range defective {
		t.Run(name, func(t *testing.T) {
			_, valid := components.UserIDFromToken(jwtSettings, token)
			assert.False(t, valid)
		})
	}
}

func TestUserIDFromTokenAcceptsAHandBuiltValidToken(t *testing.T) {
	token := signJWT(t, jwt.SigningMethodHS256, map[string]any{"typ": "at+jwt"}, validJWTClaims(), components.TokenSigningKey)

	userID, valid := components.UserIDFromToken(jwtSettings, token)
	assert.True(t, valid)
	assert.Equal(t, jwtUserID, userID)
}
