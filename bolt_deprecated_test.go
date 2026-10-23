//go:build deprecated_transport

package mercure

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

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

	// A regular file as parent directory fails even when running as root.
	notADir := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(notADir, nil, 0o600))

	u, _ = url.Parse("bolt://" + filepath.ToSlash(notADir) + "/test.db")
	_, err = DeprecatedNewBoltTransport(u, nil)
	require.ErrorContains(t, err, notADir)

	u, _ = url.Parse("bolt://test.db?cleanup_frequency=invalid")
	_, err = DeprecatedNewBoltTransport(u, nil)
	require.EqualError(t, err, `"bolt://test.db?cleanup_frequency=invalid": invalid "cleanup_frequency" parameter "invalid": invalid transport: strconv.ParseFloat: parsing "invalid": invalid syntax`)

	u, _ = url.Parse("bolt://test.db?size=invalid")
	_, err = DeprecatedNewBoltTransport(u, nil)
	require.EqualError(t, err, `"bolt://test.db?size=invalid": invalid "size" parameter "invalid": invalid transport: strconv.ParseUint: parsing "invalid": invalid syntax`)
}
