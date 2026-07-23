package setting

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
)

const defaultAlipayGatewayURL = "https://openapi.alipay.com/gateway.do"

type AlipayConfig struct {
	AppID                string
	AppPrivateKeyPath    string
	AlipayPublicKeyPath string
	NotifyURL            string
	GatewayURL           string
	AppAuthToken         string
}

func GetAlipayConfig() AlipayConfig {
	gatewayURL := strings.TrimSpace(os.Getenv("ALIPAY_GATEWAY_URL"))
	if gatewayURL == "" {
		gatewayURL = defaultAlipayGatewayURL
	}
	return AlipayConfig{
		AppID:                strings.TrimSpace(os.Getenv("ALIPAY_APP_ID")),
		AppPrivateKeyPath:    strings.TrimSpace(os.Getenv("ALIPAY_APP_PRIVATE_KEY_PATH")),
		AlipayPublicKeyPath:  strings.TrimSpace(os.Getenv("ALIPAY_PUBLIC_KEY_PATH")),
		NotifyURL:            strings.TrimSpace(os.Getenv("ALIPAY_NOTIFY_URL")),
		GatewayURL:           gatewayURL,
		AppAuthToken:         strings.TrimSpace(os.Getenv("ALIPAY_APP_AUTH_TOKEN")),
	}
}

func (config AlipayConfig) Validate() error {
	if config.AppID == "" || config.AppPrivateKeyPath == "" ||
		config.AlipayPublicKeyPath == "" || config.NotifyURL == "" || config.GatewayURL == "" {
		return errors.New("支付宝配置不完整")
	}
	if err := validateAlipayPublicHTTPSURL(config.NotifyURL, "支付宝回调地址"); err != nil {
		return err
	}
	if err := validateAlipayPublicHTTPSURL(config.GatewayURL, "支付宝网关地址"); err != nil {
		return err
	}
	if _, err := os.Stat(config.AppPrivateKeyPath); err != nil {
		return errors.New("支付宝应用私钥文件不可用")
	}
	if _, err := os.Stat(config.AlipayPublicKeyPath); err != nil {
		return errors.New("支付宝公钥文件不可用")
	}
	return nil
}

func validateAlipayPublicHTTPSURL(rawURL string, fieldName string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" ||
		parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return errors.New(fieldName + "必须是不含查询参数的公网 HTTPS URL")
	}
	hostname := parsedURL.Hostname()
	if strings.EqualFold(hostname, "localhost") {
		return errors.New(fieldName + "必须使用公网域名")
	}
	if ip := net.ParseIP(hostname); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified()) {
		return errors.New(fieldName + "不能使用内网 IP")
	}
	return nil
}
