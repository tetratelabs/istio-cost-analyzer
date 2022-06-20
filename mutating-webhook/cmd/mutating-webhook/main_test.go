package main

import (
	"bytes"
	_ "embed"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/admission-webhook.json
var reqBody string

func TestServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(mutatePod))
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL, bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	body, _ := io.ReadAll(res.Body)
	t.Logf("Response body: %s\n", body)
	assert.Equal(t, http.StatusOK, res.StatusCode)
}
