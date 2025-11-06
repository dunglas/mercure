//go:build deprecated_transport

package mercure

import (
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBoltTransport(t *testing.T) {
	t.Parallel()

	u, _ := url.Parse("bolt://test-" + t.Name() + ".db?bucket_name=demo")
	transport, err := DeprecatedNewBoltTransport(u, nil)
	require.NoError(t, err)
	require.NotNil(t, transport)
	require.NoError(t, transport.Close(t.Context()))
	require.NoError(t, os.Remove("test-"+t.Name()+".db"))

	u, _ = url.Parse("bolt://")
	_, err = DeprecatedNewBoltTransport(u, nil)
	require.EqualError(t, err, `"bolt:": invalid transport: missing path`)

	u, _ = url.Parse("bolt:///test.db")
	_, err = DeprecatedNewBoltTransport(u, nil)

	// The exact error message depends on the OS
	assert.Contains(t, err.Error(), "open /test.db:")

	u, _ = url.Parse("bolt://test.db?cleanup_frequency=invalid")
	_, err = DeprecatedNewBoltTransport(u, nil)
	require.EqualError(t, err, `"bolt://test.db?cleanup_frequency=invalid": invalid "cleanup_frequency" parameter "invalid": invalid transport: strconv.ParseFloat: parsing "invalid": invalid syntax`)

	u, _ = url.Parse("bolt://test.db?size=invalid")
	_, err = DeprecatedNewBoltTransport(u, nil)
	require.EqualError(t, err, `"bolt://test.db?size=invalid": invalid "size" parameter "invalid": invalid transport: strconv.ParseUint: parsing "invalid": invalid syntax`)
}
