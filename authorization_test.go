package mercure

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

const publicKeyEcdsa = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEOnSJ6Iht/FleVEz4s3ZFGcWQCM/I
rX2Ld/0veRv8vTAm3NU/fErGzL/raNhOxt+BcXqZ6IpQQ4aWOFZh3hDd+Q==
-----END PUBLIC KEY-----
`

//nolint:gosec
const privateKeyEcdsa = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIDZCo0gXrI9bKtspq8mLSrQ7BrGRm4WQylp4V2tx4MewoAoGCCqGSM49
AwEHoUQDQgAEOnSJ6Iht/FleVEz4s3ZFGcWQCM/IrX2Ld/0veRv8vTAm3NU/fErG
zL/raNhOxt+BcXqZ6IpQQ4aWOFZh3hDd+Q==
-----END EC PRIVATE KEY-----
`

const certificateEcdsa = `-----BEGIN CERTIFICATE-----
MIICYjCCAgmgAwIBAgIUXRW9kusU+9K8dehUwIMiRYfJjC8wCgYIKoZIzj0EAwIw
gYUxCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQHDA1T
YW4gRnJhbmNpc2NvMRIwEAYDVQQKDAlBY21lLCBJbmMxFDASBgNVBAMMC2V4YW1w
bGUuY29tMR8wHQYJKoZIhvcNAQkBFhBhY21lQGV4YW1wbGUuY29tMCAXDTI0MDkx
NDA3MjEzM1oYDzMwMjAwODIzMDcyMTMzWjCBhTELMAkGA1UEBhMCVVMxEzARBgNV
BAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xEjAQBgNVBAoM
CUFjbWUsIEluYzEUMBIGA1UEAwwLZXhhbXBsZS5jb20xHzAdBgkqhkiG9w0BCQEW
EGFjbWVAZXhhbXBsZS5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAQ6dIno
iG38WV5UTPizdkUZxZAIz8itfYt3/S95G/y9MCbc1T98SsbMv+to2E7G34Fxepno
ilBDhpY4VmHeEN35o1MwUTAdBgNVHQ4EFgQUfE5wC1hbiE60iRLKmevGqbeMSyww
HwYDVR0jBBgwFoAUfE5wC1hbiE60iRLKmevGqbeMSywwDwYDVR0TAQH/BAUwAwEB
/zAKBggqhkjOPQQDAgNHADBEAiB84zB7sZrNN8KzDO1JgeS8h2mtUeceAqnCnBwZ
krdhhAIgQg4ytVMOy0m51tnOJ+B9nq9keVwNlsJOf7rwGVpRlFQ=
-----END CERTIFICATE-----
`

const publicKeyEd25519 = `-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAKdgAvlExrnPk8TYc9cNuk4fmruFOd88FYgg9M6SQKm4=
-----END PUBLIC KEY-----
`

//nolint:gosec
const privateKeyEd25519 = `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIEYj1RXJNLVFPWeuXZfpZJBW/s/Z+gTIsP0SGRCOEHKo
-----END PRIVATE KEY-----
`

const certificateEd25519 = `-----BEGIN CERTIFICATE-----
MIICIzCCAdWgAwIBAgIUJ1OPv+s3BuDm6amXrQimaDEq9AowBQYDK2VwMIGFMQsw
CQYDVQQGEwJVUzETMBEGA1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZy
YW5jaXNjbzESMBAGA1UECgwJQWNtZSwgSW5jMRQwEgYDVQQDDAtleGFtcGxlLmNv
bTEfMB0GCSqGSIb3DQEJARYQYWNtZUBleGFtcGxlLmNvbTAgFw0yNDA5MTQwNjE2
MjdaGA8zMDIwMDgyMzA2MTYyN1owgYUxCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApD
YWxpZm9ybmlhMRYwFAYDVQQHDA1TYW4gRnJhbmNpc2NvMRIwEAYDVQQKDAlBY21l
LCBJbmMxFDASBgNVBAMMC2V4YW1wbGUuY29tMR8wHQYJKoZIhvcNAQkBFhBhY21l
QGV4YW1wbGUuY29tMCowBQYDK2VwAyEAKdgAvlExrnPk8TYc9cNuk4fmruFOd88F
Ygg9M6SQKm6jUzBRMB0GA1UdDgQWBBS75Y11AoWgeHyumy6sNTJCozENuDAfBgNV
HSMEGDAWgBS75Y11AoWgeHyumy6sNTJCozENuDAPBgNVHRMBAf8EBTADAQH/MAUG
AytlcANBANLnIRgPfKYAzigLMsUOgEoZ80tMFimhsZpgsJ2pmmzjXoX5+Zaah+kj
x3wF0MFr23e1kD/sOFatjV6h5sBZNQo=
-----END CERTIFICATE-----
`

type AuthorizationTestData struct {
	algorithm       string
	privateKey      string
	publicKey       string
	certificate     string
	validEmpty      string
	valid           string
	validForCert    string
	validNamespaced string
}

var AuthTestData = []AuthorizationTestData{
	{
		algorithm:       "HS256",
		privateKey:      "!ChangeMe!",
		publicKey:       "!ChangeMe!",
		certificate:     "",
		validEmpty:      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30._esyynAyo2Z6PyGe0mM_SuQ3c-C7sMQJ1YxVLvlj80A",
		valid:           "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.e7USPnr2YHHqLYSu9-jEVsynuTXGtAQUDAZuzoR8lxQ",
		validForCert:    "",
		validNamespaced: "",
	},
	{
		algorithm:       "RS256",
		privateKey:      privateKeyRsa,
		publicKey:       publicKeyRsa,
		certificate:     certificateRsa,
		validEmpty:      "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.e30.YbkSeO9GIBYedph1uSQz0Y6zp1NwDEB8O7ek3cc3Vw4Fjh6DwrJAwmXoNSqT6FhHDv14QG70qPIuyzsR0Q9nHFo7hGEqE8E85F8z3Pj5eBjHKBMJFno7jww514Vyp35c490ZHD6_d3F9PmxWrPkKezc1mcwlCegwiMJIS2CeR7k",
		valid:           "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.B-ZBdmCbtDaP27wB_DvF9xIetQm88M2Q1d-LP2DZoEHrz6lYDuHkgXzSDnFdbLCZ653e0r_VOaKxe2Pc6R4F0ok2vksC6P5gHhqIUcQuTSlzNFyTrg4tyy4mMkcm1h85te9gkV4LR6TABfZpFPqqIS4t7fpCMxvtAkyf_RR5Fq4",
		validForCert:    "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IkxIbzlPMmNNUzBqbzRsQWwtRk11ayJ9.eyJodHRwczovL21lcmN1cmUucm9ja3MvIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX0sImlzcyI6Imh0dHBzOi8vbWVyY3VyZS10ZXN0LmV1LmF1dGgwLmNvbS8iLCJzdWIiOiJhdXRoMHw1ZmRjM2U4OGUzYjA0YjAwNzZhNTQxM2MiLCJhdWQiOlsiaHR0cHM6Ly9tZXJjdXJlLnJvY2tzL2FwaSIsImh0dHBzOi8vbWVyY3VyZS10ZXN0LmV1LmF1dGgwLmNvbS91c2VyaW5mbyJdLCJpYXQiOjE2MDgyNzAxMTQsImF6cCI6IjMwMWh6bUJBMnZ5ZzdnSlZiSEVMUlRDell0dUJrVU52Iiwic2NvcGUiOiJvcGVuaWQgcHJvZmlsZSBlbWFpbCJ9.QAZKFSYpDJ39Cln-khjyjVzKJkiSCO4o9qIzw395fuP09rPfoLYcbdEoWg_pHN6GqO6oDNr9I2RR7p0FGhZAamXVtZzSd2V8Fv-BM0TfUBeJbb0sCMaSA2Nv3izs2dk_0zoQjGFH_LSNExGkJjwKLBj059GT6o_abtr2iz_77A8",
		validNamespaced: "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJodHRwczovL21lcmN1cmUucm9ja3MvIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.SmTVZkxlNbzHlaF4MfA3Fp5d1W2COmHlYPgc6SodAJOQtHh1Uxz0jkhA611w0OSwCaA8C5gqUd-GgekgHVPCBkIzV0qPmmhhJpTtotkeCX3N7oBOJOi58xXouNCNt0vnUH6xACqiZJq_FhNG9ZqP5saa4xNd1E-F1E9Vo1mFji4",
	},
	{
		algorithm:       "ES256",
		privateKey:      privateKeyEcdsa,
		publicKey:       publicKeyEcdsa,
		certificate:     certificateEcdsa,
		validEmpty:      "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.e30.hINnU7MroT7vlOH_DHCesipKULonewy_jnc7pNBrqCD-C9I-FjFOK8dBwbb1zG9nppYvvMDt5filtIwvcVDZUw",
		valid:           "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.5iVzGj4lm-MxaqKGOcBUdu3nAajsH1H0nq2mTyOdc9dvEyRqkKWShK-cK6KC5rkKv7vWNt8gRjR4-aV5ckvRzA",
		validForCert:    "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJodHRwczovL21lcmN1cmUucm9ja3MvIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX0sImlzcyI6Imh0dHBzOi8vbWVyY3VyZS10ZXN0LmV1LmF1dGgwLmNvbS8iLCJzdWIiOiJhdXRoMHw1ZmRjM2U4OGUzYjA0YjAwNzZhNTQxM2MiLCJhdWQiOlsiaHR0cHM6Ly9tZXJjdXJlLnJvY2tzL2FwaSIsImh0dHBzOi8vbWVyY3VyZS10ZXN0LmV1LmF1dGgwLmNvbS91c2VyaW5mbyJdLCJpYXQiOjE2MDgyNzAxMTQsImF6cCI6IjMwMWh6bUJBMnZ5ZzdnSlZiSEVMUlRDell0dUJrVU52Iiwic2NvcGUiOiJvcGVuaWQgcHJvZmlsZSBlbWFpbCJ9.iyOpr6Dxgs5yBKuUKJFvbTTaFRo65r55eEHQfWgGt0H0iRzCx5D3kheDe29Da1aRClRfunrpoxhpr8EqeO7Pxg",
		validNamespaced: "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJodHRwczovL21lcmN1cmUucm9ja3MvIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.MczmR4h4eS_cetXZ-cP8NwONOpYzDec-Wijl0u78n9GCnqYFmYbWczln250fFuYYqbnHAbtX_br84YxBdoQv3Q",
	},
	{
		algorithm:   "EdDSA",
		privateKey:  privateKeyEd25519,
		publicKey:   publicKeyEd25519,
		certificate: certificateEd25519,
		validEmpty:  "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.e30.p6y-JMVdkSrjtj1qChGi8Z5PQnAu8GiTJsq8_Txp7Yg_RATrJi6IgDlNaobyaxaHy_ypwS4G4RTmQ9mlPwFNDQ",
		valid:       "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.A2EbfgruNGekfK-VTPDX_MsrYlJdvZcAF4K5i9aKy4US2Syo4tmn9yT7aYBBdRZNBkRDqhF1sF1u26pvMLlNAw",
		// jwt.ParseEdPublicKeyFromPEM() doesn't support certificates yet
		// validForCert: "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJodHRwczovL21lcmN1cmUucm9ja3MvIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX0sImlzcyI6Imh0dHBzOi8vbWVyY3VyZS10ZXN0LmV1LmF1dGgwLmNvbS8iLCJzdWIiOiJhdXRoMHw1ZmRjM2U4OGUzYjA0YjAwNzZhNTQxM2MiLCJhdWQiOlsiaHR0cHM6Ly9tZXJjdXJlLnJvY2tzL2FwaSIsImh0dHBzOi8vbWVyY3VyZS10ZXN0LmV1LmF1dGgwLmNvbS91c2VyaW5mbyJdLCJpYXQiOjE2MDgyNzAxMTQsImF6cCI6IjMwMWh6bUJBMnZ5ZzdnSlZiSEVMUlRDell0dUJrVU52Iiwic2NvcGUiOiJvcGVuaWQgcHJvZmlsZSBlbWFpbCJ9.FBWmpbdYpuU58d3p-e_RPu-Szzj_ZPbZtwvHbUD6nQJOe83RTrsBbVpVnI54ISG6D4N5c2mLeksC_I7OAw1KCA",
		validForCert:    "",
		validNamespaced: "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJodHRwczovL21lcmN1cmUucm9ja3MvIjp7InB1Ymxpc2giOlsiZm9vIiwiYmFyIl0sInN1YnNjcmliZSI6WyJmb28iLCJiYXoiXX19.yBpIkxTSACRxEpFSiDOVhpSNRbhvhJy2ds90MycP9mK7oxiyRVyvZkRHRwbH26haa7PhR-HRzw828mGids2xDA",
	},
}

func TestAuthorizeMultipleAuthorizationHeader(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.Header.Add("Authorization", testdata.validEmpty)
		r.Header.Add("Authorization", testdata.validEmpty)

		keyfunc, _ := createJWTKeyfunc([]byte{}, testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.EqualError(t, err, `invalid "Authorization" HTTP header`, testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeAuthorizationHeaderTooShort(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.Header.Add("Authorization", "Bearer x")

		keyfunc, _ := createJWTKeyfunc([]byte{}, testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.EqualError(t, err, `invalid "Authorization" HTTP header`, testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeAuthorizationHeaderNoBearer(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.Header.Add("Authorization", "Greater "+testdata.validEmpty)

		keyfunc, _ := createJWTKeyfunc([]byte{}, testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.EqualError(t, err, `invalid "Authorization" HTTP header`, testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeAuthorizationHeaderInvalidAlg(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.Header.Add("Authorization", bearerPrefix+createDummyNoneSignedJWT())

		keyfunc, _ := createJWTKeyfunc([]byte{}, testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.Error(t, err, testdata.algorithm)
		require.Regexp(t, "^unable to parse JWT: token is unverifiable: ", err.Error(), testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeAuthorizationHeaderInvalidKey(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.Header.Add("Authorization", bearerPrefix+testdata.validEmpty)

		keyfunc, _ := createJWTKeyfunc([]byte{}, testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.Error(t, err, testdata.algorithm)
		require.Regexp(t, "^unable to parse JWT: ", err.Error(), testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeAuthorizationHeaderInvalidSignature(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.Header.Add("Authorization", bearerPrefix+testdata.validEmpty[:len(testdata.validEmpty)-8]+"12345678")

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.Error(t, err, testdata.algorithm)
		require.Regexp(t, "^unable to parse JWT: token signature is invalid: ", err.Error(), testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeAuthorizationHeaderNoContent(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.Header.Add("Authorization", bearerPrefix+testdata.validEmpty)

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.NoError(t, err, testdata.algorithm)
		require.Nil(t, claims.Mercure.Publish, testdata.algorithm)
		require.Nil(t, claims.Mercure.Subscribe, testdata.algorithm)
	}
}

func TestAuthorizeAuthorizationHeader(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.Header.Add("Authorization", bearerPrefix+testdata.valid)

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.NoError(t, err, testdata.algorithm)
		assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
		assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	}
}

func TestAuthorizeAuthorizationHeaderWithCert(t *testing.T) {
	for _, testdata := range AuthTestData {
		if testdata.validForCert != "" {
			r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
			r.Header.Add("Authorization", bearerPrefix+testdata.validForCert)

			keyfunc, _ := createJWTKeyfunc([]byte(testdata.certificate), testdata.algorithm)

			claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
			require.NoError(t, err, testdata.algorithm)
			assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
			assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
		}
	}
}

func TestAuthorizeAuthorizationHeaderNamespaced(t *testing.T) {
	for _, testdata := range AuthTestData {
		if testdata.validNamespaced != "" {
			r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
			r.Header.Add("Authorization", bearerPrefix+testdata.validNamespaced)

			keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

			claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
			require.NoError(t, err, testdata.algorithm)
			assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
			assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
		}
	}
}

func TestAuthorizeAuthorizationHeaderWrongAlgorithm(t *testing.T) {
	for idx, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.Header.Add("Authorization", bearerPrefix+testdata.valid)

		nextIdx := (idx + 1) % len(AuthTestData)
		keyfunc, _ := createJWTKeyfunc([]byte(AuthTestData[nextIdx].publicKey), AuthTestData[nextIdx].algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.Error(t, err, testdata.algorithm)
		require.Regexp(t, "^unable to parse JWT: token is unverifiable: error while executing keyfunc: (.*): unexpected signing method$", err.Error())
		assert.Nil(t, claims)
	}
}

func TestAuthorizeAuthorizationQueryTooShort(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		query := r.URL.Query()
		query.Set("authorization", "x")
		r.URL.RawQuery = query.Encode()

		keyfunc, _ := createJWTKeyfunc([]byte{}, testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.EqualError(t, err, `invalid "authorization" Query parameter`, testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeAuthorizationQueryInvalidAlg(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		query := r.URL.Query()
		query.Set("authorization", createDummyNoneSignedJWT())
		r.URL.RawQuery = query.Encode()

		keyfunc, _ := createJWTKeyfunc([]byte{}, testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.Error(t, err, testdata.algorithm)
		require.Regexp(t, "^unable to parse JWT: token is unverifiable: ", err.Error(), testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeAuthorizationQueryInvalidKey(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		query := r.URL.Query()
		query.Set("authorization", testdata.validEmpty)
		r.URL.RawQuery = query.Encode()

		keyfunc, _ := createJWTKeyfunc([]byte{}, testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.Error(t, err, testdata.algorithm)
		require.Regexp(t, "^unable to parse JWT: ", err.Error(), testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeAuthorizationQueryInvalidSignature(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		query := r.URL.Query()
		query.Set("authorization", testdata.validEmpty[:len(testdata.validEmpty)-8]+"12345678")
		r.URL.RawQuery = query.Encode()

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.Error(t, err, testdata.algorithm)
		require.Regexp(t, "^unable to parse JWT: token signature is invalid: ", err.Error(), testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeAuthorizationQueryNoContent(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		query := r.URL.Query()
		query.Set("authorization", testdata.validEmpty)
		r.URL.RawQuery = query.Encode()

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.NoError(t, err, testdata.algorithm)
		require.Nil(t, claims.Mercure.Publish, testdata.algorithm)
		require.Nil(t, claims.Mercure.Subscribe, testdata.algorithm)
	}
}

func TestAuthorizeAuthorizationQuery(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		query := r.URL.Query()
		query.Set("authorization", testdata.valid)
		r.URL.RawQuery = query.Encode()

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.NoError(t, err, testdata.algorithm)
		assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
		assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	}
}

func TestAuthorizeAuthorizationQueryNamespaced(t *testing.T) {
	for _, testdata := range AuthTestData {
		if testdata.validNamespaced != "" {
			r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
			query := r.URL.Query()
			query.Set("authorization", testdata.validNamespaced)
			r.URL.RawQuery = query.Encode()

			keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

			claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
			require.NoError(t, err, testdata.algorithm)
			assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
			assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
		}
	}
}

func TestAuthorizeAuthorizationQueryRsaWithCert(t *testing.T) {
	for _, testdata := range AuthTestData {
		if testdata.validForCert != "" {
			r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
			query := r.URL.Query()
			query.Set("authorization", testdata.validForCert)
			r.URL.RawQuery = query.Encode()

			keyfunc, _ := createJWTKeyfunc([]byte(testdata.certificate), testdata.algorithm)

			claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
			require.NoError(t, err, testdata.algorithm)
			assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
			assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
		}
	}
}

func TestAuthorizeAuthorizationQueryWrongAlgorithm(t *testing.T) {
	for idx, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		query := r.URL.Query()
		query.Set("authorization", testdata.valid)
		r.URL.RawQuery = query.Encode()

		nextIdx := (idx + 1) % len(AuthTestData)
		keyfunc, _ := createJWTKeyfunc([]byte(AuthTestData[nextIdx].publicKey), AuthTestData[nextIdx].algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.Error(t, err, testdata.algorithm)
		require.Regexp(t, "^unable to parse JWT: token is unverifiable: error while executing keyfunc: (.*): unexpected signing method$", err.Error(), testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeCookieInvalidAlg(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: createDummyNoneSignedJWT()})

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.EqualError(t, err, "unable to parse JWT: token is unverifiable: error while executing keyfunc: *jwt.signingMethodNone: unexpected signing method", testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeCookieInvalidKey(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: testdata.validEmpty})

		keyfunc, _ := createJWTKeyfunc([]byte{}, testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.Error(t, err, testdata.algorithm)
		require.Regexp(t, "^unable to parse JWT: ", err.Error(), testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeCookieInvalidSignature(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: testdata.validEmpty[:len(testdata.validEmpty)-8] + "12345678"})

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.Error(t, err, testdata.algorithm)
		require.Regexp(t, "^unable to parse JWT: token signature is invalid: ", err.Error(), testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeCookieNoContent(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: testdata.validEmpty})

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.NoError(t, err, testdata.algorithm)
		require.Nil(t, claims.Mercure.Publish, testdata.algorithm)
		require.Nil(t, claims.Mercure.Subscribe, testdata.algorithm)
	}
}

func TestAuthorizeCookie(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodGet, defaultHubURL, nil)
		r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: testdata.valid})

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.NoError(t, err, testdata.algorithm)
		assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
		assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	}
}

func TestAuthorizeCookieNoOriginNoReferer(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
		r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: testdata.valid})

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{}, defaultCookieName)
		require.EqualError(t, err, `an "Origin" or a "Referer" HTTP header must be present to use the cookie-based authorization mechanism`, testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeCookieOriginNotAllowed(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
		r.Header.Add("Origin", "http://example.com")
		r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: testdata.valid})

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{"http://example.net"}, defaultCookieName)
		require.EqualError(t, err, `"http://example.com": origin not allowed to post updates`, testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeCookieRefererNotAllowed(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
		r.Header.Add("Referer", "http://example.com/foo/bar")
		r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: testdata.valid})

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{"http://example.net"}, defaultCookieName)
		require.EqualError(t, err, `"http://example.com": origin not allowed to post updates`, testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeCookieInvalidReferer(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
		r.Header.Add("Referer", "http://192.168.0.%31/")
		r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: testdata.valid})

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{"http://example.net"}, defaultCookieName)
		require.EqualError(t, err, `unable to parse referer: parse "http://192.168.0.%31/": invalid URL escape "%31"`, testdata.algorithm)
		require.Nil(t, claims, testdata.algorithm)
	}
}

func TestAuthorizeCookieOriginHasPriority(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
		r.Header.Add("Origin", "http://example.net")
		r.Header.Add("Referer", "http://example.com")
		r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: testdata.valid})

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		claims, err := authorize(r, keyfunc, []string{"http://example.net"}, defaultCookieName)
		require.NoError(t, err, testdata.algorithm)
		assert.Equal(t, []string{"foo", "bar"}, claims.Mercure.Publish)
		assert.Equal(t, []string{"foo", "baz"}, claims.Mercure.Subscribe)
	}
}

func TestAuthorizeAllOriginsAllowed(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
		r.Header.Add("Origin", "http://example.com")
		r.AddCookie(&http.Cookie{Name: defaultCookieName, Value: testdata.valid})

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		_, err := authorize(r, keyfunc, []string{"*"}, defaultCookieName)
		require.NoError(t, err, testdata.algorithm)
	}
}

func TestAuthorizeCustomCookieName(t *testing.T) {
	for _, testdata := range AuthTestData {
		r, _ := http.NewRequest(http.MethodPost, defaultHubURL, nil)
		r.Header.Add("Origin", "http://example.com")
		r.AddCookie(&http.Cookie{Name: "foo", Value: testdata.valid})

		keyfunc, _ := createJWTKeyfunc([]byte(testdata.publicKey), testdata.algorithm)

		_, err := authorize(r, keyfunc, []string{"*"}, "foo")
		require.NoError(t, err, testdata.algorithm)
	}
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
