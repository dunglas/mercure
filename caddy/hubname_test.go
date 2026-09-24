package caddy

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hubNamesConfig(servers ...[]string) []byte {
	configs := make([]string, 0, len(servers))

	for i, names := range servers {
		handlers := make([]string, 0, len(names))

		for _, name := range names {
			handlers = append(handlers, fmt.Sprintf(`{"handler":"mercure","name":%q,"anonymous":true,"transport":{"name":"local"},"issuers":[{"identifier":"https://example.com","publisher":{"jwt":{"key":"test-publisher-key","alg":"HS256"}}}]}`, name))
		}

		configs = append(configs, fmt.Sprintf(`"server%d":{"listen":["127.0.0.1:0"],"automatic_https":{"disable":true},"routes":[{"handle":[%s]}]}`, i, strings.Join(handlers, ",")))
	}

	return fmt.Appendf(nil, `{"admin":{"disabled":true},"apps":{"http":{"servers":{%s}}}}`, strings.Join(configs, ","))
}

func unnamedHubCount() int {
	hubsMu.Lock()
	defer hubsMu.Unlock()

	return len(unnamedHubs)
}

func TestUnnamedHubLimit(t *testing.T) {
	for _, tc := range []struct {
		name      string
		servers   [][]string
		wantError bool
	}{
		{name: "one unnamed", servers: [][]string{{""}}},
		{name: "named hubs", servers: [][]string{{"a", "b"}}},
		{name: "unnamed and named", servers: [][]string{{"", "a"}}},
		{name: "explicit default sharing", servers: [][]string{{"", "default"}}},
		{name: "two unnamed in one route", servers: [][]string{{"", ""}}, wantError: true},
		{name: "named hub between unnamed hubs", servers: [][]string{{"", "a", ""}}, wantError: true},
		{name: "unnamed hubs on different servers", servers: [][]string{{""}, {""}}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := unnamedHubCount()

			var cfg caddy.Config
			require.NoError(t, json.Unmarshal(hubNamesConfig(tc.servers...), &cfg))

			err := caddy.Validate(&cfg)
			if tc.wantError {
				require.ErrorContains(t, err, errMultipleUnnamedHubs.Error())
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, before, unnamedHubCount(), "validation must release its unnamed hub registration")
		})
	}
}

func TestUnnamedHubReload(t *testing.T) {
	t.Cleanup(func() {
		assert.NoError(t, caddy.Stop())
		assert.Zero(t, unnamedHubCount())
	})

	single := hubNamesConfig([]string{""})
	require.NoError(t, caddy.Load(single, true))
	assert.Equal(t, 1, unnamedHubCount())

	require.NoError(t, caddy.Load(single, true))
	assert.Equal(t, 1, unnamedHubCount())

	require.ErrorContains(t, caddy.Load(hubNamesConfig([]string{"", ""}), true), errMultipleUnnamedHubs.Error())
	assert.Equal(t, 1, unnamedHubCount(), "a failed reload must preserve only the active registration")

	require.NoError(t, caddy.Load(single, true))
	assert.Equal(t, 1, unnamedHubCount())
}
