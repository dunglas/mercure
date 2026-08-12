package caddy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignTokenHMACDefaultsToHS256(t *testing.T) {
	t.Parallel()

	signed, alg, err := signToken(jwt.MapClaims{claimIss: "https://example.com"}, "", "!ChangeMe!", "")
	require.NoError(t, err)
	assert.Equal(t, "HS256", alg)

	parsed, err := jwt.Parse(signed, func(*jwt.Token) (any, error) { return []byte("!ChangeMe!"), nil })
	require.NoError(t, err)
	assert.True(t, parsed.Valid)
	assert.Equal(t, accessTokenType, parsed.Header["typ"])
}

func TestReadKeyMaterialLiteral(t *testing.T) {
	t.Parallel()

	material, err := readKeyMaterial("!ChangeMe!")
	require.NoError(t, err)
	assert.Equal(t, []byte("!ChangeMe!"), material)
}

func TestReadKeyMaterialFromFileTrimsTrailingNewline(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "key")
	require.NoError(t, os.WriteFile(path, []byte("!ChangeMe!\n"), 0o600))

	material, err := readKeyMaterial("@" + path)
	require.NoError(t, err)
	assert.Equal(t, []byte("!ChangeMe!"), material)
}

// TestReadKeyMaterialFromStdin swaps the package-level os.Stdin, so it must
// not run in parallel with another test doing the same (none do).
func TestReadKeyMaterialFromStdin(t *testing.T) {
	old := os.Stdin

	t.Cleanup(func() { os.Stdin = old })

	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdin = r

	go func() {
		_, _ = w.WriteString("!ChangeMe!\r\n")
		w.Close()
	}()

	material, err := readKeyMaterial("@-")
	require.NoError(t, err)
	assert.Equal(t, []byte("!ChangeMe!"), material)
}

func TestSignTokenPEMWithoutAlgFails(t *testing.T) {
	t.Parallel()

	_, _, err := signToken(jwt.MapClaims{}, "", "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----", "")
	require.ErrorIs(t, err, errPEMKeyNeedsAlg)
}

// TestSignTokenPEMWithHMACAlgFails guards against the algorithm-confusion
// attack normalizeJWT already rejects on the verifier side: an HMAC algorithm
// would sign with the PEM bytes as a shared secret, which anyone holding the
// public half of the key pair could reproduce.
func TestSignTokenPEMWithHMACAlgFails(t *testing.T) {
	t.Parallel()

	_, _, err := signToken(jwt.MapClaims{}, "", "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----", "HS256")
	require.ErrorIs(t, err, errPEMKeyHMACAlgorithm)
}

func TestBuildAuthorizationDetailsMergesExactAndURLPattern(t *testing.T) {
	t.Parallel()

	p := &tokenParams{
		publish:      []string{"https://example.com/books/1"},
		publishURL:   []string{"https://example.com/books/:id"},
		subscribe:    []string{"https://example.com/books/1"},
		subscribeURL: nil,
		payload:      `{"tenant":"acme"}`,
	}

	details, err := p.buildAuthorizationDetails()
	require.NoError(t, err)
	require.Len(t, details, 2)

	pub := details[0]
	assert.Equal(t, authorizationDetailType, pub.Type)
	assert.Equal(t, []string{"publish"}, pub.Actions)
	require.Len(t, pub.Topics, 2)
	assert.Equal(t, topicMatcher{Match: "https://example.com/books/1"}, pub.Topics[0])
	assert.Equal(t, topicMatcher{Match: "https://example.com/books/:id", MatchType: "urlpattern"}, pub.Topics[1])

	sub := details[1]
	assert.Equal(t, []string{"subscribe"}, sub.Actions)
	assert.Equal(t, map[string]any{"tenant": "acme"}, sub.Payload)
}

// TestMintedTokenRoundTrips exercises buildClaims and signToken together, the
// same path cmdMercureToken uses, and parses the result back with golang-jwt
// directly (bypassing the hub) to catch structural mistakes — wrong typ, a
// claims shape the server-side validator in this repo would also reject —
// without needing a running hub.
func TestMintedTokenRoundTrips(t *testing.T) {
	t.Parallel()

	p := &tokenParams{
		iss:      "https://example.com",
		aud:      "https://hub.example.com/.well-known/mercure",
		key:      "!ChangeMe!",
		publish:  []string{"*"},
		clientID: "https://example.com",
	}

	details, err := p.buildAuthorizationDetails()
	require.NoError(t, err)

	claims := p.buildClaims(details, time.Unix(0, 0), 4102444800)

	signed, alg, err := signToken(claims, "", p.key, p.alg)
	require.NoError(t, err)
	assert.Equal(t, "HS256", alg)

	parsed, err := jwt.Parse(signed, func(*jwt.Token) (any, error) { return []byte(p.key), nil })
	require.NoError(t, err)
	assert.True(t, parsed.Valid)
	assert.Equal(t, accessTokenType, parsed.Header["typ"])

	parsedClaims, ok := parsed.Claims.(jwt.MapClaims)
	require.True(t, ok)
	assert.Equal(t, p.iss, parsedClaims[claimIss])
	assert.Equal(t, p.aud, parsedClaims[claimAud])
	assert.NotEmpty(t, parsedClaims[claimSub])
	assert.NotEmpty(t, parsedClaims[claimJTI])
}
