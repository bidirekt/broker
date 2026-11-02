package bearer

import (
	"strconv"
	"time"

	"github.com/bidirekt/broker/internal/components/config"
	"github.com/golang-jwt/jwt/v5"
)

// Hardcoded until its source is decided, before any release: the repository is public, so anyone
// who reads it can forge a token.
var SigningKey = []byte("kRQ+AHq7X4TKaSzlILIt12cVJ0nb9zc4trUzGTR+KzA=")

const (
	tokenType           = "at+jwt"
	tokenLifetime       = 15 * time.Minute
	tokenAudienceSuffix = "/api"
)

func Issue(settings config.Settings, userID int64, issuedAt time.Time) (token string, expiresAt time.Time, err error) {
	claims := jwt.RegisteredClaims{
		Issuer:    settings.PublicURL,
		Audience:  jwt.ClaimStrings{settings.PublicURL + tokenAudienceSuffix},
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ExpiresAt: jwt.NewNumericDate(issuedAt.Add(tokenLifetime)),
	}

	unsigned := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	unsigned.Header["typ"] = tokenType

	token, err = unsigned.SignedString(SigningKey)
	if err != nil {
		return "", time.Time{}, err
	}

	return token, claims.ExpiresAt.Time, nil
}

func UserIDFrom(settings config.Settings, token string) (userID int64, valid bool) {
	claims := &jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return SigningKey, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(settings.PublicURL),
		jwt.WithAudience(settings.PublicURL+tokenAudienceSuffix),
		jwt.WithExpirationRequired(),
	)
	if err != nil || parsed.Header["typ"] != tokenType {
		return 0, false
	}

	userID, err = strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || strconv.FormatInt(userID, 10) != claims.Subject {
		return 0, false
	}

	return userID, true
}
