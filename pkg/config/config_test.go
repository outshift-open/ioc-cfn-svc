package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDSN(t *testing.T) {
	d := Database{Host: "h.com", Port: "1", Name: "a", User: "x", Password: "y"}
	assert.Equal(t, d.Enabled(), true)
	assert.Equal(t, d.PostgresURL(), "postgresql://x:y@h.com:1/a?sslmode=require&timezone=UTC")
	assert.Equal(t, d.PostgresMigrateURL(), "postgresql://x:y@h.com:1/a?sslmode=require&timezone=UTC&search_path=public")
	assert.Equal(t, d.DSN(), "host=h.com port=1 dbname=a user=x password=y timezone=UTC sslmode=require")

	d.Host = "localhost"
	assert.Equal(t, d.DSN(), "host=localhost port=1 dbname=a user=x password=y timezone=UTC sslmode=disable")

	d.User = ""
	d.Password = ""
	assert.Equal(t, d.Enabled(), false)

	d.User = "x"
	d.Password = "y"
	assert.Equal(t, d.Enabled(), true)
	assert.Equal(t, d.Loggable(), "host=localhost port=1 dbname=a user=x password=<redacted_1> timezone=UTC sslmode=disable")

	d.SSLMode = "verify-full"
	assert.Equal(t, d.DSN(),
		"host=localhost port=1 dbname=a user=x password=y timezone=UTC sslmode=verify-full")
	assert.Equal(t, d.PostgresURL(),
		"postgresql://x:y@localhost:1/a?sslmode=verify-full&timezone=UTC")
	assert.Equal(t, d.PostgresMigrateURL(),
		"postgresql://x:y@localhost:1/a?sslmode=verify-full&timezone=UTC&search_path=public")

	d.SSLRootCert = "/etc/ssl/certs/special.pem"
	assert.Equal(t, d.DSN(),
		"host=localhost port=1 dbname=a user=x password=y timezone=UTC sslmode=verify-full sslrootcert=/etc/ssl/certs/special.pem")
	assert.Equal(t, d.PostgresURL(),
		"postgresql://x:y@localhost:1/a?sslmode=verify-full&sslrootcert=%2Fetc%2Fssl%2Fcerts%2Fspecial.pem&timezone=UTC")
	assert.Equal(t, d.PostgresMigrateURL(),
		"postgresql://x:y@localhost:1/a?sslmode=verify-full&sslrootcert=%2Fetc%2Fssl%2Fcerts%2Fspecial.pem&timezone=UTC&search_path=public")
	assert.Equal(t, d.Loggable(),
		"host=localhost port=1 dbname=a user=x password=<redacted_1> timezone=UTC sslmode=verify-full sslrootcert=/etc/ssl/certs/special.pem")
}
