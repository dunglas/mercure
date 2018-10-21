package hub

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

const validEmptyHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.gciTwpaT-wC-s73qBP7OQ_BMp9Oosm8YXvIpqYWddzY"
const validFullHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.pylpKeHDlAN2HHBayzufKYc6VqDfhjCuzlH72r1W6Nw"

func TestAuthorizeMultipleAuthorizationHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", validEmptyHeader)
	r.Header.Add("Authorization", validEmptyHeader)

	claims, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "Invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderTooShort(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", "Bearer x")

	claims, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "Invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderNoBearer(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", "Greater "+validEmptyHeader)

	claims, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "Invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderInvalidAlg(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", "Bearer "+createDummyNoneSignedJWT())

	claims, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "Unexpected signing method: none")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderInvalidKey(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", "Bearer "+validEmptyHeader)

	claims, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "signature is invalid")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderNoContent(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", "Bearer "+validEmptyHeader)

	claims, err := authorize(r, []byte("!UnsecureChangeMe!"), []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", "Bearer "+validFullHeader)

	claims, err := authorize(r, []byte("!UnsecureChangeMe!"), []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieInvalidAlg(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: createDummyNoneSignedJWT()})

	claims, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "Unexpected signing method: none")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidKey(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeader})

	claims, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "signature is invalid")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieNoContent(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeader})

	claims, err := authorize(r, []byte("!UnsecureChangeMe!"), []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookie(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!UnsecureChangeMe!"), []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieNoOriginNoReferer(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!UnsecureChangeMe!"), []string{})
	assert.EqualError(t, err, "An \"Origin\" or a \"Referer\" HTTP header must be present to use the cookie-based authorization mechanism")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieOriginNotAllowed(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.Header.Add("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!UnsecureChangeMe!"), []string{"http://example.net"})
	assert.EqualError(t, err, "The origin \"http://example.com\" is not allowed to post updates")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieRefererNotAllowed(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.Header.Add("Referer", "http://example.com/foo/bar")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!UnsecureChangeMe!"), []string{"http://example.net"})
	assert.EqualError(t, err, "The origin \"http://example.com\" is not allowed to post updates")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidReferer(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.Header.Add("Referer", "http://192.168.0.%31/")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!UnsecureChangeMe!"), []string{"http://example.net"})
	assert.EqualError(t, err, "parse http://192.168.0.%31/: invalid URL escape \"%31\"")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieOriginHasPriority(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.Header.Add("Origin", "http://example.net")
	r.Header.Add("Referer", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!UnsecureChangeMe!"), []string{"http://example.net"})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizedNilClaim(t *testing.T) {
	all, targets := authorizedTargets(nil, true)
	assert.False(t, all)
	assert.Empty(t, targets)
}

func TestAuthorizedTargetsPublisher(t *testing.T) {
	c := &claims{Mercure: mercureClaim{
		Publish: []string{"foo", "bar"},
	}}

	all, targets := authorizedTargets(c, true)
	assert.False(t, all)
	assert.Equal(t, map[string]struct{}{"foo": {}, "bar": {}}, targets)
}

func TestAuthorizedAllTargetsPublisher(t *testing.T) {
	c := &claims{Mercure: mercureClaim{
		Publish: []string{"*"},
	}}

	all, targets := authorizedTargets(c, true)
	assert.True(t, all)
	assert.Empty(t, targets)
}

func TestAuthorizedTargetsSubsciber(t *testing.T) {
	c := &claims{Mercure: mercureClaim{
		Subscribe: []string{"foo", "bar"},
	}}

	all, targets := authorizedTargets(c, false)
	assert.False(t, all)
	assert.Equal(t, map[string]struct{}{"foo": {}, "bar": {}}, targets)
}

func TestAuthorizedAllTargetsSubsciber(t *testing.T) {
	c := &claims{Mercure: mercureClaim{
		Subscribe: []string{"*"},
	}}

	all, targets := authorizedTargets(c, false)
	assert.True(t, all)
	assert.Empty(t, targets)
}
