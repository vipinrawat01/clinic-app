package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("auth: invalid or expired token")

// Claims has no company/tenant scoping, unlike the main chatbot
// platform's own auth — this whole app serves exactly one clinic, so a
// token only ever needs to say who the staff member is and what their
// role lets them do.
type Claims struct {
	StaffUserID uuid.UUID `json:"staff_user_id"`
	Role        string    `json:"role"`
	jwt.RegisteredClaims
}

type TokenIssuer struct {
	secret   []byte
	tokenTTL time.Duration
}

func NewTokenIssuer(secret string) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), tokenTTL: 12 * time.Hour}
}

// IssueToken issues a session token for a staff member — a 12-hour TTL
// (a full shift, not a short-lived web-app access token) since this is a
// single internal tool with no refresh-token flow of its own yet.
func (t *TokenIssuer) IssueToken(staffUserID uuid.UUID, role string) (string, error) {
	claims := Claims{
		StaffUserID: staffUserID,
		Role:        role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(t.tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(t.secret)
}

func (t *TokenIssuer) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return t.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
