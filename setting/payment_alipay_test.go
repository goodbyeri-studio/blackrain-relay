package setting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAlipayConfigValidate(t *testing.T) {
	secretDir := t.TempDir()
	privateKeyPath := filepath.Join(secretDir, "app_private.pem")
	publicKeyPath := filepath.Join(secretDir, "alipay_public.pem")
	require.NoError(t, os.WriteFile(privateKeyPath, []byte("test"), 0o600))
	require.NoError(t, os.WriteFile(publicKeyPath, []byte("test"), 0o600))

	config := AlipayConfig{
		AppID:               "2021000000000000",
		AppPrivateKeyPath:   privateKeyPath,
		AlipayPublicKeyPath: publicKeyPath,
		NotifyURL:           "https://relay.example.com/api/payment/alipay/notify",
		GatewayURL:          "https://openapi.alipay.com/gateway.do",
	}
	require.NoError(t, config.Validate())

	config.NotifyURL = "http://relay.example.com/api/payment/alipay/notify"
	require.Error(t, config.Validate())

	config.NotifyURL = "https://relay.example.com/api/payment/alipay/notify?token=secret"
	require.Error(t, config.Validate())

	config.NotifyURL = "https://localhost/api/payment/alipay/notify"
	require.Error(t, config.Validate())

	config.NotifyURL = "https://127.0.0.1/api/payment/alipay/notify"
	require.Error(t, config.Validate())
}
