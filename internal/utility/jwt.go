package utility

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateAdapterJWT(secret string, expiration time.Duration, adapterId int) (string, error) {
	if secret == "" {
		return "", errors.New("No JWT secret set")
	}
	claims := jwt.MapClaims{
		"exp":       time.Now().Add(expiration).Unix(),
		"iat":       time.Now().Unix(),
		"adapterId": adapterId,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}
