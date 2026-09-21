package controller

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOfficialPaymentRSA256SignatureRoundTrip(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: mustMarshalPKCS8PrivateKey(t, privateKey),
	})
	publicBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicBytes})

	signature, err := signRSA256("official-payment-message", string(privatePEM))
	require.NoError(t, err)
	assert.NoError(t, verifyRSA256("official-payment-message", signature, string(publicPEM)))
	assert.Error(t, verifyRSA256("tampered-message", signature, string(publicPEM)))
}

func TestCanonicalAlipayValuesExcludesSignatureFields(t *testing.T) {
	values := url.Values{
		"method":    {"alipay.trade.page.pay"},
		"app_id":    {"20260001"},
		"sign_type": {"RSA2"},
		"sign":      {base64.StdEncoding.EncodeToString([]byte("signature"))},
		"empty":     {""},
	}

	assert.Equal(t, "app_id=20260001&method=alipay.trade.page.pay", canonicalAlipayValues(values))
}

func TestDecryptWeChatResource(t *testing.T) {
	apiV3Key := "12345678901234567890123456789012"
	nonce := "123456789012"
	associatedData := "transaction"
	plaintext := []byte(`{"trade_state":"SUCCESS"}`)
	ciphertext := encryptWeChatResourceForTest(t, plaintext, nonce, associatedData, apiV3Key)

	decrypted, err := decryptWeChatResource(ciphertext, nonce, associatedData, apiV3Key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestVerifyWeChatPayHTTPMessage(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: mustMarshalPKCS8PrivateKey(t, privateKey),
	})
	publicBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicBytes})

	originalPublicKey := setting.WeChatPayPlatformPublicKey
	originalPublicKeyID := setting.WeChatPayPlatformPublicKeyID
	t.Cleanup(func() {
		setting.WeChatPayPlatformPublicKey = originalPublicKey
		setting.WeChatPayPlatformPublicKeyID = originalPublicKeyID
	})
	setting.WeChatPayPlatformPublicKey = string(publicPEM)
	setting.WeChatPayPlatformPublicKeyID = "PUB_KEY_ID"

	body := []byte(`{"code_url":"weixin://wxpay/example"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "official-payment-nonce"
	signature, err := signRSA256(timestamp+"\n"+nonce+"\n"+string(body)+"\n", string(privatePEM))
	require.NoError(t, err)
	header := http.Header{
		"Wechatpay-Timestamp": {timestamp},
		"Wechatpay-Nonce":     {nonce},
		"Wechatpay-Serial":    {"PUB_KEY_ID"},
		"Wechatpay-Signature": {signature},
	}

	assert.NoError(t, verifyWeChatPayHTTPMessage(header, body))
	header.Set("Wechatpay-Timestamp", strconv.FormatInt(time.Now().Add(-6*time.Minute).Unix(), 10))
	assert.Error(t, verifyWeChatPayHTTPMessage(header, body))
}

func mustMarshalPKCS8PrivateKey(t *testing.T, privateKey *rsa.PrivateKey) []byte {
	t.Helper()
	value, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	return value
}

func encryptWeChatResourceForTest(t *testing.T, plaintext []byte, nonce string, associatedData string, apiV3Key string) string {
	t.Helper()
	block, err := aes.NewCipher([]byte(apiV3Key))
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	ciphertext := gcm.Seal(nil, []byte(nonce), plaintext, []byte(associatedData))
	return base64.StdEncoding.EncodeToString(ciphertext)
}
