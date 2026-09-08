package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestJWTIssueAndParseRS256(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	j := &JWT{private: key, public: &key.PublicKey, issuer: "test", ttl: time.Hour}
	id := uuid.New()
	token, err := j.Issue(id, "102304015", "admin")
	require.NoError(t, err)
	claims, err := j.Parse(token)
	require.NoError(t, err)
	require.Equal(t, id, claims.UserID)
	require.Equal(t, "102304015", claims.ThaparID)
	require.Equal(t, "admin", claims.Role)
}

func TestJWTRejectsExpiredToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	j := &JWT{private: key, public: &key.PublicKey, issuer: "test", ttl: -time.Hour}
	token, err := j.Issue(uuid.New(), "id", "student")
	require.NoError(t, err)
	_, err = j.Parse(token)
	require.Error(t, err)
}
