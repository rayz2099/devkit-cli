package td

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

// pubBody 是 TE 4.4 前端写死的 RSA-1024 公钥. 登录接口只认这把钥匙的 PKCS1 密文.
const pubBody = "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCT+JeQB4oij4007Kslmma0sfFodahq1olOpWJoZR+aimyNuYUSTmbK1/bArGkIb0CxIPUGQIrARpY6apQNncORutdmBHonFkn7r7XpktL7bxZhDUKeLjIdjaEB1lh1tFxLl3cP0siHwuc1aAaEOWAjuCu2f9+FgY7WCEJla9Qa/QIDAQAB"

func encryptPwd(plain string) (string, error) {
	der, err := base64.StdEncoding.DecodeString(pubBody)
	if err != nil {
		return "", fmt.Errorf("rsa key: %w", err)
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return "", fmt.Errorf("rsa key: %w", err)
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("rsa key: not a public key")
	}
	buf, err := rsa.EncryptPKCS1v15(rand.Reader, key, []byte(plain))
	if err != nil {
		return "", fmt.Errorf("encrypt password: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}
