package middleware

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 载荷：租户 ID 随令牌下发，后续请求据此隔离数据。
// TenantID = 0 表示单机版（默认租户）。
type Claims struct {
	UserID      int64  `json:"user_id"`
	TenantID    int64  `json:"tenant_id"`
	DisplayName string `json:"display_name"`
	jwt.RegisteredClaims
}

func MakeToken(userID, tenantID int64, displayName, secret string) (string, error) {
	claims := Claims{
		UserID:      userID,
		TenantID:    tenantID,
		DisplayName: displayName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func ParseToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
