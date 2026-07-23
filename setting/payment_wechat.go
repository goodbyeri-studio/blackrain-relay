package setting

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
)

type WechatPayConfig struct {
	AppID                  string
	MchID                  string
	MerchantSerialNo       string
	MerchantPrivateKeyPath string
	MerchantPrivateKeyPEM  string
	PublicKeyID            string
	PublicKeyPath          string
	PublicKeyPEM           string
	APIv3Key               string
	NotifyURL              string
}

func GetWechatPayConfig() WechatPayConfig {
	return WechatPayConfig{
		AppID:                  strings.TrimSpace(os.Getenv("WECHAT_PAY_APP_ID")),
		MchID:                  strings.TrimSpace(os.Getenv("WECHAT_PAY_MCH_ID")),
		MerchantSerialNo:       strings.TrimSpace(os.Getenv("WECHAT_PAY_MERCHANT_SERIAL_NO")),
		MerchantPrivateKeyPath: strings.TrimSpace(os.Getenv("WECHAT_PAY_MERCHANT_PRIVATE_KEY_PATH")),
		MerchantPrivateKeyPEM:  strings.TrimSpace(os.Getenv("WECHAT_PAY_MERCHANT_PRIVATE_KEY_PEM")),
		PublicKeyID:            strings.TrimSpace(os.Getenv("WECHAT_PAY_PUBLIC_KEY_ID")),
		PublicKeyPath:          strings.TrimSpace(os.Getenv("WECHAT_PAY_PUBLIC_KEY_PATH")),
		PublicKeyPEM:           strings.TrimSpace(os.Getenv("WECHAT_PAY_PUBLIC_KEY_PEM")),
		APIv3Key:               strings.TrimSpace(os.Getenv("WECHAT_PAY_API_V3_KEY")),
		NotifyURL:              strings.TrimSpace(os.Getenv("WECHAT_PAY_NOTIFY_URL")),
	}
}

func (config WechatPayConfig) Validate() error {
	if config.AppID == "" || config.MchID == "" || config.MerchantSerialNo == "" ||
		(config.MerchantPrivateKeyPath == "" && config.MerchantPrivateKeyPEM == "") || config.PublicKeyID == "" ||
		(config.PublicKeyPath == "" && config.PublicKeyPEM == "") || config.APIv3Key == "" || config.NotifyURL == "" {
		return errors.New("微信支付配置不完整")
	}
	if len(config.APIv3Key) != 32 {
		return errors.New("微信支付 APIv3 密钥必须为 32 个字符")
	}
	parsedNotifyURL, err := url.Parse(config.NotifyURL)
	if err != nil || parsedNotifyURL.Scheme != "https" || parsedNotifyURL.Host == "" || parsedNotifyURL.User != nil || parsedNotifyURL.RawQuery != "" || parsedNotifyURL.Fragment != "" {
		return errors.New("微信支付回调地址必须是不含查询参数的公网 HTTPS URL")
	}
	hostname := parsedNotifyURL.Hostname()
	if strings.EqualFold(hostname, "localhost") {
		return errors.New("微信支付回调地址必须使用公网域名")
	}
	if ip := net.ParseIP(hostname); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified()) {
		return errors.New("微信支付回调地址不能使用内网 IP")
	}
	if config.MerchantPrivateKeyPEM == "" {
		if _, err = os.Stat(config.MerchantPrivateKeyPath); err != nil {
			return errors.New("微信支付商户私钥文件不可用")
		}
	}
	if config.PublicKeyPEM == "" {
		if _, err = os.Stat(config.PublicKeyPath); err != nil {
			return errors.New("微信支付公钥文件不可用")
		}
	}
	return nil
}
