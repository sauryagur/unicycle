package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID   uuid.UUID `json:"user_id"`
	ThaparID string    `json:"thapar_id"`
	Role     string    `json:"role"`
	jwt.RegisteredClaims
}

type JWT struct {
	private *rsa.PrivateKey
	public  *rsa.PublicKey
	issuer  string
	ttl     time.Duration
}

// NewEphemeral creates a process-local key pair for development only. Tokens
// become invalid after restart; production must use LoadFromEnv with mounted
// persistent keys.
func NewEphemeral() (*JWT, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &JWT{private: key, public: &key.PublicKey, issuer: os.Getenv("JWT_ISSUER"), ttl: 24 * time.Hour}, nil
}

func LoadFromEnv() (*JWT, error) {
	privatePEM, err := os.ReadFile(os.Getenv("JWT_PRIVATE_KEY_FILE"))
	if err != nil {
		return nil, err
	}
	publicPEM, err := os.ReadFile(os.Getenv("JWT_PUBLIC_KEY_FILE"))
	if err != nil {
		return nil, err
	}
	privBlock, _ := pem.Decode(privatePEM)
	pubBlock, _ := pem.Decode(publicPEM)
	if privBlock == nil || pubBlock == nil {
		return nil, errors.New("invalid JWT PEM")
	}
	priv, err := x509.ParsePKCS8PrivateKey(privBlock.Bytes)
	if err != nil {
		if parsed, pkcs1Err := x509.ParsePKCS1PrivateKey(privBlock.Bytes); pkcs1Err == nil {
			priv = parsed
		} else {
			return nil, err
		}
	}
	pub, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		if parsed, pkcs1Err := x509.ParsePKCS1PublicKey(pubBlock.Bytes); pkcs1Err == nil {
			pub = parsed
		} else {
			return nil, err
		}
	}
	private, ok := priv.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("JWT private key is not RSA")
	}
	public, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("JWT public key is not RSA")
	}
	ttl := 24 * time.Hour
	if v := os.Getenv("JWT_TTL"); v != "" {
		if parsed, e := time.ParseDuration(v); e == nil {
			ttl = parsed
		}
	}
	return &JWT{private: private, public: public, issuer: os.Getenv("JWT_ISSUER"), ttl: ttl}, nil
}

func (j *JWT) Issue(userID uuid.UUID, thaparID, role string) (string, error) {
	now := time.Now()
	c := Claims{UserID: userID, ThaparID: thaparID, Role: role, RegisteredClaims: jwt.RegisteredClaims{Issuer: j.issuer, Subject: userID.String(), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)), ID: uuid.NewString()}}
	return jwt.NewWithClaims(jwt.SigningMethodRS256, c).SignedString(j.private)
}
func (j *JWT) Parse(token string) (*Claims, error) {
	c := &Claims{}
	t, err := jwt.ParseWithClaims(token, c, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodRS256 {
			return nil, errors.New("unexpected JWT signing method")
		}
		return j.public, nil
	}, jwt.WithIssuer(j.issuer))
	if err != nil || !t.Valid {
		return nil, errors.New("invalid JWT")
	}
	return c, nil
}
