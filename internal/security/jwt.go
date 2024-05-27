package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateToken(subject string, securityKey string) (string, error) {
	now := time.Now()
	tokenString, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.MapClaims{
			"sub": subject,
			"iat": now.Unix(),
			"exp": now.Add(24 * time.Hour).Unix(),
		}).SignedString([]byte(securityKey))

	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func VerifyToken(token string, securityKey string) (*jwt.Token, error) {
	claims := &jwt.RegisteredClaims{}
	parsedToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(securityKey), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("JWT token is expired: %v", token)
		}
		return nil, err
	}
	if !parsedToken.Valid {
		return nil, fmt.Errorf("JWT token is invalid: %v", token)
	}
	return parsedToken, nil
}
