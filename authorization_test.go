package mercure

import (
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	validEmptyHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30._esyynAyo2Z6PyGe0mM_SuQ3c-C7sMQJ1YxVLvlj80A"
	validFullHeader  = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.e7USPnr2YHHqLYSu9-jEVsynuTXGtAQUDAZuzoR8lxQ"
)

const (
	validEmptyHeaderRsa          = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.e30.YbkSeO9GIBYedph1uSQz0Y6zp1NwDEB8O7ek3cc3Vw4Fjh6DwrJAwmXoNSqT6FhHDv14QG70qPIuyzsR0Q9nHFo7hGEqE8E85F8z3Pj5eBjHKBMJFno7jww514Vyp35c490ZHD6_d3F9PmxWrPkKezc1mcwlCegwiMJIS2CeR7k"
	validFullHeaderRsa           = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.B-ZBdmCbtDaP27wB_DvF9xIetQm88M2Q1d-LP2DZoEHrz6lYDuHkgXzSDnFdbLCZ653e0r_VOaKxe2Pc6R4F0ok2vksC6P5gHhqIUcQuTSlzNFyTrg4tyy4mMkcm1h85te9gkV4LR6TABfZpFPqqIS4t7fpCMxvtAkyf_RR5Fq4"
	validFullHeaderRsaForCert    = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IkxIbzlPMmNNUzBqbzRsQWwtRk11ayJ9.eyJodHRwczovL21lcmN1cmUucm9ja3MvIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX0sImlzcyI6Imh0dHBzOi8vbWVyY3VyZS10ZXN0LmV1LmF1dGgwLmNvbS8iLCJzdWIiOiJhdXRoMHw1ZmRjM2U4OGUzYjA0YjAwNzZhNTQxM2MiLCJhdWQiOlsiaHR0cHM6Ly9tZXJjdXJlLnJvY2tzL2FwaSIsImh0dHBzOi8vbWVyY3VyZS10ZXN0LmV1LmF1dGgwLmNvbS91c2VyaW5mbyJdLCJpYXQiOjE2MDgyNzAxMTQsImF6cCI6IjMwMWh6bUJBMnZ5ZzdnSlZiSEVMUlRDell0dUJrVU52Iiwic2NvcGUiOiJvcGVuaWQgcHJvZmlsZSBlbWFpbCJ9.QAZKFSYpDJ39Cln-khjyjVzKJkiSCO4o9qIzw395fuP09rPfoLYcbdEoWg_pHN6GqO6oDNr9I2RR7p0FGhZAamXVtZzSd2V8Fv-BM0TfUBeJbb0sCMaSA2Nv3izs2dk_0zoQjGFH_LSNExGkJjwKLBj059GT6o_abtr2iz_77A8"
	validFullHeaderNamespacedRsa = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJodHRwczovL21lcmN1cmUucm9ja3MvIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.SmTVZkxlNbzHlaF4MfA3Fp5d1W2COmHlYPgc6SodAJOQtHh1Uxz0jkhA611w0OSwCaA8C5gqUd-GgekgHVPCBkIzV0qPmmhhJpTtotkeCX3N7oBOJOi58xXouNCNt0vnUH6xACqiZJq_FhNG9ZqP5saa4xNd1E-F1E9Vo1mFji4"
)

const publicKeyRsa = `-----BEGIN PUBLIC KEY-----
MIGeMA0GCSqGSIb3DQEBAQUAA4GMADCBiAKBgHVwuJsFmzsFnOkGj+OgAp4lTNqR
CF0RZSmjY+ECWOJ3sSEzQ8qtkJe61uSjr/PKmqvBxxex0YtUL7waSS4jvq3ws8Bm
WIxK2GqoAVjLjK8HzThSPQpgv2AjiEXD6iAERHeySLGjYAUgfMrVJ01J5fNSL+O+
bCd7nPuNAyYHCOOHAgMBAAE=
-----END PUBLIC KEY-----
`

//nolint:gosec
const privateKeyRsa = `-----BEGIN RSA PRIVATE KEY-----
MIICWwIBAAKBgHVwuJsFmzsFnOkGj+OgAp4lTNqRCF0RZSmjY+ECWOJ3sSEzQ8qt
kJe61uSjr/PKmqvBxxex0YtUL7waSS4jvq3ws8BmWIxK2GqoAVjLjK8HzThSPQpg
v2AjiEXD6iAERHeySLGjYAUgfMrVJ01J5fNSL+O+bCd7nPuNAyYHCOOHAgMBAAEC
gYAiOKOCgMK4Ey2i9YeOQ70fiiz375UpUX1SAcuD8KQn8crKqt6RO7xLimU+ILiP
6LTjYcb7D5TI7dIvFNXIPSA9tpGbuPqzwa0aBkIoIxJkJ7vs6gHijq3kAQl3mik2
ddzL7OtdlbXG8fRnvgKsRLw2gVlv4+8C3OKmKADJR1bSQQJBAM/xS49IxjUIXMLO
77tsGd+VxpKo1jrUG5Ao9feFfSxWiFnnlDG9DOriDvguPf0WUkU08j7fC3A3AKVd
dQkkqWECQQCQlPSA96lJIUD9xCx7S46L7+e+A2EWnhyMb3u4D1EY5rZdA3/Zzc3P
68Jb8RtRryGuDvezLRcqmVJWq5X97i3nAkADeJ2wSKC2ZetWfSnXURileNSVwifB
V6UWJPjmJt5ODSu9hHYe1m8OxLNHRU5XmTXKXfXlQsfoGaLzH7pCatBBAkBraBzT
iiiaiTeszYV1+sVks85m3D/N+5udwFwaelZ2tz4Wjzj1ZuxUYAI9JzpyTjYpBjmB
RCgHn2sJs+Jzh/NVAkEAnRQKOSQRcm/o4PWNsvrqRwoqUzDcnVcEY67pKPwcnnlR
Ki0jUpg2xzzwyA+nEI6Bf6CDaHKnCqxL7x0yk2XqeA==
-----END RSA PRIVATE KEY-----
`

const certificateRsa = `-----BEGIN CERTIFICATE-----
MIIChDCCAe0CCQDlJhrdK2G+pDANBgkqhkiG9w0BAQsFADCBhTELMAkGA1UEBhMC
VVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28x
EjAQBgNVBAoMCUFjbWUsIEluYzEUMBIGA1UEAwwLZXhhbXBsZS5jb20xHzAdBgkq
hkiG9w0BCQEWEGFjbWVAZXhhbXBsZS5jb20wIBcNMjAxMjIxMTYxNjM2WhgPMzAy
MDA0MjMxNjE2MzZaMIGFMQswCQYDVQQGEwJVUzETMBEGA1UECAwKQ2FsaWZvcm5p
YTEWMBQGA1UEBwwNU2FuIEZyYW5jaXNjbzESMBAGA1UECgwJQWNtZSwgSW5jMRQw
EgYDVQQDDAtleGFtcGxlLmNvbTEfMB0GCSqGSIb3DQEJARYQYWNtZUBleGFtcGxl
LmNvbTCBnjANBgkqhkiG9w0BAQEFAAOBjAAwgYgCgYB1cLibBZs7BZzpBo/joAKe
JUzakQhdEWUpo2PhAljid7EhM0PKrZCXutbko6/zypqrwccXsdGLVC+8GkkuI76t
8LPAZliMSthqqAFYy4yvB804Uj0KYL9gI4hFw+ogBER3skixo2AFIHzK1SdNSeXz
Ui/jvmwne5z7jQMmBwjjhwIDAQABMA0GCSqGSIb3DQEBCwUAA4GBABosw/cIJkKr
KKBRFBiYuZEeilRHVP2UiUzC8dAASLyw7r63Fg8J7NEN5bYFNdNw1uvvteMryjYu
t+4Iti/mSObpG8FbNb/pOkSJjuJvAxnAIL8iM/DbF28a0SfWiluu5Nk/PciJXLU4
Utb8p35tfj97usdiEB0AN8ray4wZbVWj
-----END CERTIFICATE-----
`

var keyfuncHS256 = func() jwt.Keyfunc {
	keyfunc, _ := createJWTKeyfunc([]byte("!ChangeMe!"), "HS256")

	return keyfunc
}()

var keyfuncRS256publicKey = func() jwt.Keyfunc {
	keyfunc, _ := createJWTKeyfunc([]byte(publicKeyRsa), "RS256")

	return keyfunc
}()

func TestAuthorizeMultipleAuthorizationHeader(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", validEmptyHeader)
	r.Header.Add("Authorization", validEmptyHeader)

	keyfunc, _ := createJWTKeyfunc([]byte{}, "HS256")

	claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
	require.Error(t, err, `invalid "Authorization" HTTP header`)
	require.Nil(t, claims)
}

func TestAuthorizeMultipleAuthorizationHeaderRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", validEmptyHeaderRsa)
	r.Header.Add("Authorization", validEmptyHeaderRsa)

	keyfunc, _ := createJWTKeyfunc([]byte{}, "RS256")

	claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
	require.Error(t, err, `invalid "Authorization" HTTP header`)
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderTooShort(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", "Bearer x")

	keyfunc, _ := createJWTKeyfunc([]byte{}, "HS256")

	claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
	require.Error(t, err, `invalid "Authorization" HTTP header`)
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderNoBearer(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", "Greater "+validEmptyHeader)

	keyfunc, _ := createJWTKeyfunc([]byte{}, "HS256")

	claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
	require.EqualError(t, err, `invalid "Authorization" HTTP header`)
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderNoBearerRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", "Greater "+validEmptyHeaderRsa)

	keyfunc, _ := createJWTKeyfunc([]byte{}, "RS256")

	claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
	require.EqualError(t, err, `invalid "Authorization" HTTP header`)
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderInvalidAlg(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+createDummyNoneSignedJWT())

	keyfunc, _ := createJWTKeyfunc([]byte{}, "HS256")

	claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
	require.EqualError(t, err, "unable to parse JWT: token is unverifiable: error while executing keyfunc: *jwt.signingMethodNone: unexpected signing method")
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderInvalidKey(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+validEmptyHeader)

	keyfunc, _ := createJWTKeyfunc([]byte{}, "HS256")

	claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
	require.EqualError(t, err, "unable to parse JWT: token signature is invalid: signature is invalid")
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationHeaderNoContent(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+validEmptyHeader)

	claims, err := authorize(r, keyfuncHS256, []string{}, defaultCookieName)
	require.NoError(t, err)
	require.Nil(t, claims.Mercure.Publish)
	require.Nil(t, claims.Mercure.Subscribe)
}

func TestAuthorizeAuthorizationHeaderNoContentRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+validEmptyHeaderRsa)

	claims, err := authorize(r, keyfuncRS256publicKey, []string{}, defaultCookieName)
	require.NoError(t, err)
	require.Nil(t, claims.Mercure.Publish)
	require.Nil(t, claims.Mercure.Subscribe)
}

func TestAuthorizeAuthorizationHeader(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+validFullHeader)

	claims, err := authorize(r, keyfuncHS256, []string{}, defaultCookieName)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
}

func TestAuthorizeAuthorizationHeaderRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+validFullHeaderRsa)

	claims, err := authorize(r, keyfuncRS256publicKey, []string{}, defaultCookieName)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
}

func TestAuthorizeAuthorizationHeaderNamespacedRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+validFullHeaderNamespacedRsa)

	claims, err := authorize(r, keyfuncRS256publicKey, []string{}, defaultCookieName)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
}

func TestAuthorizeAuthorizationHeaderRsaWithCert(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+validFullHeaderRsaForCert)

	keyfunc, _ := createJWTKeyfunc([]byte(certificateRsa), "RS256")

	claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
}

func TestAuthorizeAuthorizationHeaderWrongAlgorithm(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.Header.Add("Authorization", bearerPrefix+validFullHeaderRsa)

	claims, err := authorize(r, keyfuncHS256, []string{}, defaultCookieName)
	require.EqualError(t, err, "unable to parse JWT: token is unverifiable: error while executing keyfunc: *jwt.SigningMethodRSA: unexpected signing method")
	assert.Nil(t, claims)
}

func TestAuthorizeAuthorizationQueryTooShort(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	query := r.URL.Query()
	query.Set("authorization", "x")
	r.URL.RawQuery = query.Encode()

	keyfunc, _ := createJWTKeyfunc([]byte{}, "HS256")

	claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
	require.EqualError(t, err, `invalid "authorization" Query parameter`)
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationQueryInvalidAlg(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	query := r.URL.Query()
	query.Set("authorization", createDummyNoneSignedJWT())
	r.URL.RawQuery = query.Encode()

	keyfunc, _ := createJWTKeyfunc([]byte{}, "HS256")

	claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
	require.EqualError(t, err, "unable to parse JWT: token is unverifiable: error while executing keyfunc: *jwt.signingMethodNone: unexpected signing method")
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationQueryInvalidKey(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	query := r.URL.Query()
	query.Set("authorization", validEmptyHeader)
	r.URL.RawQuery = query.Encode()

	keyfunc, _ := createJWTKeyfunc([]byte{}, "HS256")

	claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
	require.EqualError(t, err, "unable to parse JWT: token signature is invalid: signature is invalid")
	require.Nil(t, claims)
}

func TestAuthorizeAuthorizationQueryNoContent(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	query := r.URL.Query()
	query.Set("authorization", validEmptyHeader)
	r.URL.RawQuery = query.Encode()

	claims, err := authorize(r, keyfuncHS256, []string{}, defaultCookieName)
	require.NoError(t, err)
	require.Nil(t, claims.Mercure.Publish)
	require.Nil(t, claims.Mercure.Subscribe)
}

func TestAuthorizeAuthorizationQueryNoContentRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	query := r.URL.Query()
	query.Set("authorization", validEmptyHeaderRsa)
	r.URL.RawQuery = query.Encode()

	claims, err := authorize(r, keyfuncRS256publicKey, []string{}, defaultCookieName)
	require.NoError(t, err)
	require.Nil(t, claims.Mercure.Publish)
	require.Nil(t, claims.Mercure.Subscribe)
}

func TestAuthorizeAuthorizationQuery(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	query := r.URL.Query()
	query.Set("authorization", validFullHeader)
	r.URL.RawQuery = query.Encode()

	claims, err := authorize(r, keyfuncHS256, []string{}, defaultCookieName)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
}

func TestAuthorizeAuthorizationQueryRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	query := r.URL.Query()
	query.Set("authorization", validFullHeaderRsa)
	r.URL.RawQuery = query.Encode()

	claims, err := authorize(r, keyfuncRS256publicKey, []string{}, defaultCookieName)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
}

func TestAuthorizeAuthorizationQueryNamespacedRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	query := r.URL.Query()
	query.Set("authorization", validFullHeaderNamespacedRsa)
	r.URL.RawQuery = query.Encode()

	claims, err := authorize(r, keyfuncRS256publicKey, []string{}, defaultCookieName)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
}

func TestAuthorizeAuthorizationQueryRsaWithCert(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	query := r.URL.Query()
	query.Set("authorization", validFullHeaderRsaForCert)
	r.URL.RawQuery = query.Encode()

	keyfunc, _ := createJWTKeyfunc([]byte(certificateRsa), "RS256")

	claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
}

func TestAuthorizeAuthorizationQueryWrongAlgorithm(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	query := r.URL.Query()
	query.Set("authorization", validFullHeaderRsa)
	r.URL.RawQuery = query.Encode()

	claims, err := authorize(r, keyfuncHS256, []string{}, defaultCookieName)
	require.EqualError(t, err, "unable to parse JWT: token is unverifiable: error while executing keyfunc: *jwt.SigningMethodRSA: unexpected signing method")
	require.Nil(t, claims)
}

func TestAuthorizeCookieInvalidAlg(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyNoneSignedJWT()})

	claims, err := authorize(r, keyfuncHS256, []string{}, defaultCookieName)
	require.EqualError(t, err, "unable to parse JWT: token is unverifiable: error while executing keyfunc: *jwt.signingMethodNone: unexpected signing method")
	require.Nil(t, claims)
}

func TestAuthorizeCookieInvalidKey(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validEmptyHeader})

	keyfunc, _ := createJWTKeyfunc([]byte{}, "HS256")

	claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
	require.EqualError(t, err, "unable to parse JWT: token signature is invalid: signature is invalid")
	require.Nil(t, claims)
}

func TestAuthorizeCookieNoContent(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validEmptyHeader})

	claims, err := authorize(r, keyfuncHS256, []string{}, defaultCookieName)
	require.NoError(t, err)
	require.Nil(t, claims.Mercure.Publish)
	require.Nil(t, claims.Mercure.Subscribe)
}

func TestAuthorizeCookieNoContentRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validEmptyHeaderRsa})

	claims, err := authorize(r, keyfuncRS256publicKey, []string{}, defaultCookieName)
	require.NoError(t, err)
	require.Nil(t, claims.Mercure.Publish)
	require.Nil(t, claims.Mercure.Subscribe)
}

func TestAuthorizeCookie(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validFullHeader})

	claims, err := authorize(r, keyfuncHS256, []string{}, defaultCookieName)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
}

func TestAuthorizeCookieRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validFullHeaderRsa})

	claims, err := authorize(r, keyfuncRS256publicKey, []string{}, defaultCookieName)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
}

func TestAuthorizeCookieNoOriginNoReferer(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validFullHeader})

	claims, err := authorize(r, keyfuncHS256, []string{}, defaultCookieName)
	require.EqualError(t, err, `an "Origin" or a "Referer" HTTP header must be present to use the cookie-based authorization mechanism`)
	require.Nil(t, claims)
}

func TestAuthorizeCookieNoOriginNoRefererRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validFullHeaderRsa})

	claims, err := authorize(r, keyfuncRS256publicKey, []string{}, defaultCookieName)
	require.EqualError(t, err, `an "Origin" or a "Referer" HTTP header must be present to use the cookie-based authorization mechanism`)
	require.Nil(t, claims)
}

func TestAuthorizeCookieOriginNotAllowed(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.Header.Add("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validFullHeader})

	claims, err := authorize(r, keyfuncHS256, []string{"http://example.net"}, defaultCookieName)
	require.EqualError(t, err, `"http://example.com": origin not allowed to post updates`)
	require.Nil(t, claims)
}

func TestAuthorizeCookieOriginNotAllowedRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.Header.Add("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validFullHeaderRsa})

	claims, err := authorize(r, keyfuncRS256publicKey, []string{"http://example.net"}, defaultCookieName)
	require.EqualError(t, err, `"http://example.com": origin not allowed to post updates`)
	require.Nil(t, claims)
}

func TestAuthorizeCookieRefererNotAllowed(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.Header.Add("Referer", "http://example.com/foo/bar")
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validFullHeader})

	claims, err := authorize(r, keyfuncHS256, []string{"http://example.net"}, defaultCookieName)
	require.EqualError(t, err, `"http://example.com": origin not allowed to post updates`)
	require.Nil(t, claims)
}

func TestAuthorizeCookieRefererNotAllowedRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.Header.Add("Referer", "http://example.com/foo/bar")
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validFullHeaderRsa})

	claims, err := authorize(r, keyfuncRS256publicKey, []string{"http://example.net"}, defaultCookieName)
	require.EqualError(t, err, `"http://example.com": origin not allowed to post updates`)
	require.Nil(t, claims)
}

func TestAuthorizeCookieInvalidReferer(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.Header.Add("Referer", "http://192.168.0.%31/")
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validFullHeader})

	claims, err := authorize(r, keyfuncHS256, []string{"http://example.net"}, defaultCookieName)
	require.EqualError(t, err, `unable to parse referer: parse "http://192.168.0.%31/": invalid URL escape "%31"`)
	require.Nil(t, claims)
}

func TestAuthorizeCookieInvalidRefererRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.Header.Add("Referer", "http://192.168.0.%31/")
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validFullHeaderRsa})

	claims, err := authorize(r, keyfuncRS256publicKey, []string{"http://example.net"}, defaultCookieName)
	require.EqualError(t, err, `unable to parse referer: parse "http://192.168.0.%31/": invalid URL escape "%31"`)
	require.Nil(t, claims)
}

func TestAuthorizeCookieOriginHasPriority(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.Header.Add("Origin", "http://example.net")
	r.Header.Add("Referer", "http://example.com")
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validFullHeader})

	claims, err := authorize(r, keyfuncHS256, []string{"http://example.net"}, defaultCookieName)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
}

func TestAuthorizeCookieOriginHasPriorityRsa(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.Header.Add("Origin", "http://example.net")
	r.Header.Add("Referer", "http://example.com")
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validFullHeaderRsa})

	claims, err := authorize(r, keyfuncRS256publicKey, []string{"http://example.net"}, defaultCookieName)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
	assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
}

func TestAuthorizeAllOriginsAllowed(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.Header.Add("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: validFullHeader})

	_, err := authorize(r, keyfuncHS256, []string{"*"}, defaultCookieName)
	require.NoError(t, err)
}

func TestAuthorizeCustomCookieName(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
	r.Header.Add("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: "foo", Value: validFullHeader})

	_, err := authorize(r, keyfuncHS256, []string{"*"}, "foo")
	require.NoError(t, err)
}

func TestCanReceive(t *testing.T) {
	tss := &TopicSelectorStore{}
	assert.True(t, canReceive(tss, []string{"foo", "bar"}, []string{"foo", "bar"}))
	assert.True(t, canReceive(tss, []string{"foo", "bar"}, []string{"bar"}))
	assert.True(t, canReceive(tss, []string{"foo", "bar"}, []string{"*"}))
	assert.False(t, canReceive(tss, []string{"foo", "bar"}, []string{}))
	assert.False(t, canReceive(tss, []string{"foo", "bar"}, []string{"baz"}))
	assert.False(t, canReceive(tss, []string{"foo", "bar"}, []string{"baz", "bat"}))
}

func TestCanDispatch(t *testing.T) {
	tss := &TopicSelectorStore{}
	assert.True(t, canDispatch(tss, []string{"foo", "bar"}, []string{"foo", "bar"}))
	assert.True(t, canDispatch(tss, []string{"foo", "bar"}, []string{"*"}))
	assert.False(t, canDispatch(tss, []string{"foo", "bar"}, []string{}))
	assert.False(t, canDispatch(tss, []string{"foo", "bar"}, []string{"foo"}))
	assert.False(t, canDispatch(tss, []string{"foo", "bar"}, []string{"baz"}))
	assert.False(t, canDispatch(tss, []string{"foo", "bar"}, []string{"baz", "bat"}))
}
