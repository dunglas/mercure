package hub

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

const validEmptyHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30._esyynAyo2Z6PyGe0mM_SuQ3c-C7sMQJ1YxVLvlj80A"
const validFullHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.e7USPnr2YHHqLYSu9-jEVsynuTXGtAQUDAZuzoR8lxQ"

const validEmptyHeaderRsa = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.e30.YbkSeO9GIBYedph1uSQz0Y6zp1NwDEB8O7ek3cc3Vw4Fjh6DwrJAwmXoNSqT6FhHDv14QG70qPIuyzsR0Q9nHFo7hGEqE8E85F8z3Pj5eBjHKBMJFno7jww514Vyp35c490ZHD6_d3F9PmxWrPkKezc1mcwlCegwiMJIS2CeR7k"
const validFullHeaderRsa = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.B-ZBdmCbtDaP27wB_DvF9xIetQm88M2Q1d-LP2DZoEHrz6lYDuHkgXzSDnFdbLCZ653e0r_VOaKxe2Pc6R4F0ok2vksC6P5gHhqIUcQuTSlzNFyTrg4tyy4mMkcm1h85te9gkV4LR6TABfZpFPqqIS4t7fpCMxvtAkyf_RR5Fq4"

const publicKeyRsa = `-----BEGIN PUBLIC KEY-----
MIGeMA0GCSqGSIb3DQEBAQUAA4GMADCBiAKBgHVwuJsFmzsFnOkGj+OgAp4lTNqR
CF0RZSmjY+ECWOJ3sSEzQ8qtkJe61uSjr/PKmqvBxxex0YtUL7waSS4jvq3ws8Bm
WIxK2GqoAVjLjK8HzThSPQpgv2AjiEXD6iAERHeySLGjYAUgfMrVJ01J5fNSL+O+
bCd7nPuNAyYHCOOHAgMBAAE=
-----END PUBLIC KEY-----
`

func TestAuthorizeMultipleAuthorizationHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", validEmptyHeader)
	r.Header.Add("Authorization", validEmptyHeader)

	claims, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "Invalid \"Authorization\" HTTP header")
	assert.Nil(t, claims)
}

func TestAuthorizeMultipleAuthorizationHeaderRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", validEmptyHeaderRsa)
	r.Header.Add("Authorization", validEmptyHeaderRsa)

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

func TestAuthorizeAuthorizationHeaderNoBearerRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", "Greater "+validEmptyHeaderRsa)

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

func TestAuthorizeAuthorizationHeaderInvalidKeyRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", "Bearer "+validEmptyHeaderRsa)

	claims, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "public key error")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderNoContent(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", "Bearer "+validEmptyHeader)

	claims, err := authorize(r, []byte("!ChangeMe!"), []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeaderNoContentRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", "Bearer "+validEmptyHeaderRsa)

	claims, err := authorize(r, []byte(publicKeyRsa), []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", "Bearer "+validFullHeader)

	claims, err := authorize(r, []byte("!ChangeMe!"), []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeAuthorizationHeaderRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.Header.Add("Authorization", "Bearer "+validFullHeaderRsa)

	claims, err := authorize(r, []byte(publicKeyRsa), []string{})
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

func TestAuthorizeCookieInvalidKeyRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeaderRsa})

	claims, err := authorize(r, []byte{}, []string{})
	assert.EqualError(t, err, "public key error")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieNoContent(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieNoContentRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validEmptyHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), []string{})
	assert.Nil(t, claims.Mercure.Publish)
	assert.Nil(t, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookie(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieRsa(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/hub", nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), []string{})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieNoOriginNoReferer(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), []string{})
	assert.EqualError(t, err, "An \"Origin\" or a \"Referer\" HTTP header must be present to use the cookie-based authorization mechanism")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieNoOriginNoRefererRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), []string{})
	assert.EqualError(t, err, "An \"Origin\" or a \"Referer\" HTTP header must be present to use the cookie-based authorization mechanism")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieOriginNotAllowed(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.Header.Add("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), []string{"http://example.net"})
	assert.EqualError(t, err, "The origin \"http://example.com\" is not allowed to post updates")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieOriginNotAllowedRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.Header.Add("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), []string{"http://example.net"})
	assert.EqualError(t, err, "The origin \"http://example.com\" is not allowed to post updates")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieRefererNotAllowed(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.Header.Add("Referer", "http://example.com/foo/bar")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), []string{"http://example.net"})
	assert.EqualError(t, err, "The origin \"http://example.com\" is not allowed to post updates")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieRefererNotAllowedRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.Header.Add("Referer", "http://example.com/foo/bar")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), []string{"http://example.net"})
	assert.EqualError(t, err, "The origin \"http://example.com\" is not allowed to post updates")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidReferer(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.Header.Add("Referer", "http://192.168.0.%31/")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), []string{"http://example.net"})
	assert.EqualError(t, err, "parse http://192.168.0.%31/: invalid URL escape \"%31\"")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieInvalidRefererRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.Header.Add("Referer", "http://192.168.0.%31/")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), []string{"http://example.net"})
	assert.EqualError(t, err, "parse http://192.168.0.%31/: invalid URL escape \"%31\"")
	assert.Nil(t, claims)
}

func TestAuthorizeCookieOriginHasPriority(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.Header.Add("Origin", "http://example.net")
	r.Header.Add("Referer", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeader})

	claims, err := authorize(r, []byte("!ChangeMe!"), []string{"http://example.net"})
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	assert.Nil(t, err)
}

func TestAuthorizeCookieOriginHasPriorityRsa(t *testing.T) {
	r, _ := http.NewRequest("POST", "http://example.com/hub", nil)
	r.Header.Add("Origin", "http://example.net")
	r.Header.Add("Referer", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "mercureAuthorization", Value: validFullHeaderRsa})

	claims, err := authorize(r, []byte(publicKeyRsa), []string{"http://example.net"})
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
