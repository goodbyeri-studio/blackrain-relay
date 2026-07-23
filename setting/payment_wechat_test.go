package setting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWechatPayConfigValidate(t *testing.T) {
	secretDir := t.TempDir()
	privateKeyPath := filepath.Join(secretDir, "merchant.pem")
	publicKeyPath := filepath.Join(secretDir, "wechatpay.pem")
	require.NoError(t, os.WriteFile(privateKeyPath, []byte("test"), 0o600))
	require.NoError(t, os.WriteFile(publicKeyPath, []byte("test"), 0o600))

	config := WechatPayConfig{
		AppID:                  "wx123",
		MchID:                  "1900000001",
		MerchantSerialNo:       "serial",
		MerchantPrivateKeyPath: privateKeyPath,
		PublicKeyID:            "PUB_KEY_ID_123",
		PublicKeyPath:          publicKeyPath,
		APIv3Key:               "12345678901234567890123456789012",
		NotifyURL:              "https://relay.example.com/api/payment/wechat/notify",
	}
	require.NoError(t, config.Validate())

	config.NotifyURL = "http://relay.example.com/api/payment/wechat/notify"
	require.Error(t, config.Validate())

	config.NotifyURL = "https://relay.example.com/api/payment/wechat/notify?token=secret"
	require.Error(t, config.Validate())

	config.NotifyURL = "https://localhost/api/payment/wechat/notify"
	require.Error(t, config.Validate())

	config.NotifyURL = "https://127.0.0.1/api/payment/wechat/notify"
	require.Error(t, config.Validate())

	config.NotifyURL = "https://relay.example.com/api/payment/wechat/notify"
	config.MerchantPrivateKeyPath = ""
	config.PublicKeyPath = ""
	config.MerchantPrivateKeyPEM = "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----"
	config.PublicKeyPEM = "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----"
	require.NoError(t, config.Validate())
}
