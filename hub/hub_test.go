package hub

import (
	"os"
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
)

const testAddr = "127.0.0.1:4242"

func TestNewHub(t *testing.T) {
	h := createDummy()

	assert.IsType(t, &Options{}, h.options)
	assert.IsType(t, map[chan *serializedUpdate]struct{}{}, h.subscribers.m)
	assert.IsType(t, make(chan (chan *serializedUpdate)), h.newSubscribers)
	assert.IsType(t, make(chan (chan *serializedUpdate)), h.removedSubscribers)
	assert.IsType(t, make(chan *serializedUpdate), h.updates)
}

func TestNewHubFromEnv(t *testing.T) {
	os.Setenv("PUBLISHER_JWT_KEY", "foo")
	os.Setenv("JWT_KEY", "bar")
	defer os.Unsetenv("PUBLISHER_JWT_KEY")
	defer os.Unsetenv("JWT_KEY")

	h, db, err := NewHubFromEnv()
	defer db.Close()
	assert.NotNil(t, h)
	assert.NotNil(t, db)
	assert.Nil(t, err)
}

func TestNewHubFromEnvError(t *testing.T) {
	h, db, err := NewHubFromEnv()
	assert.Nil(t, h)
	assert.Nil(t, db)
	assert.Error(t, err)
}

func createDummy() *Hub {
	return NewHub(&localPublisher{}, &noHistory{}, &Options{PublisherJWTKey: []byte("publisher"), SubscriberJWTKey: []byte("subscriber")})
}

func createAnonymousDummy() *Hub {
	return NewHub(&localPublisher{}, &noHistory{}, &Options{
		PublisherJWTKey:  []byte("publisher"),
		SubscriberJWTKey: []byte("subscriber"),
		AllowAnonymous:   true,
		Addr:             testAddr,
	})
}

func createAnonymousDummyWithHistory(h History) *Hub {
	return NewHub(&localPublisher{}, h, &Options{
		PublisherJWTKey:  []byte("publisher"),
		SubscriberJWTKey: []byte("subscriber"),
		AllowAnonymous:   true,
		Addr:             testAddr,
	})
}

func createDummyAuthorizedJWT(h *Hub, publisher bool, targets []string) string {
	var key []byte
	token := jwt.New(jwt.SigningMethodHS256)

	if publisher {
		key = h.options.PublisherJWTKey
		token.Claims = &claims{mercureClaim{Publish: targets}, jwt.StandardClaims{}}
	} else {
		key = h.options.SubscriberJWTKey
		token.Claims = &claims{mercureClaim{Subscribe: targets}, jwt.StandardClaims{}}
	}

	tokenString, _ := token.SignedString(key)

	return tokenString
}

func createDummyUnauthorizedJWT() string {
	token := jwt.New(jwt.SigningMethodHS256)
	tokenString, _ := token.SignedString([]byte("unauthorized"))

	return tokenString
}

func createDummyNoneSignedJWT() string {
	token := jwt.New(jwt.SigningMethodNone)
	// The generated token must have more than 41 chars
	token.Claims = jwt.StandardClaims{Subject: "me"}
	tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	return tokenString
}
