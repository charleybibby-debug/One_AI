package controller

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const (
	alipayGatewayURL    = "https://openapi.alipay.com/gateway.do"
	wechatNativePayPath = "/v3/pay/transactions/native"
)

type officialPayRequest struct {
	Amount        int64  `json:"amount"`
	PaymentMethod string `json:"payment_method"`
}

type wechatNativePayRequest struct {
	AppID       string `json:"appid"`
	MchID       string `json:"mchid"`
	Description string `json:"description"`
	OutTradeNo  string `json:"out_trade_no"`
	NotifyURL   string `json:"notify_url"`
	Amount      struct {
		Total    int64  `json:"total"`
		Currency string `json:"currency"`
	} `json:"amount"`
}

type wechatNativePayResponse struct {
	CodeURL string `json:"code_url"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type wechatNotification struct {
	EventType string `json:"event_type"`
	Resource  struct {
		Algorithm      string `json:"algorithm"`
		Ciphertext     string `json:"ciphertext"`
		AssociatedData string `json:"associated_data"`
		Nonce          string `json:"nonce"`
	} `json:"resource"`
}

type wechatTransaction struct {
	AppID      string `json:"appid"`
	MchID      string `json:"mchid"`
	OutTradeNo string `json:"out_trade_no"`
	TradeState string `json:"trade_state"`
	Amount     struct {
		Total    int64  `json:"total"`
		Currency string `json:"currency"`
	} `json:"amount"`
}

func isWeChatPayDirectEnabled() bool {
	return isPaymentComplianceConfirmed() && setting.WeChatPayDirectEnabled
}

func isWeChatPayDirectConfigured() bool {
	return isPaymentComplianceConfirmed() &&
		strings.TrimSpace(setting.WeChatPayAppID) != "" &&
		strings.TrimSpace(setting.WeChatPayMchID) != "" &&
		strings.TrimSpace(setting.WeChatPayMchSerialNo) != "" &&
		strings.TrimSpace(setting.WeChatPayPrivateKey) != "" &&
		len(strings.TrimSpace(setting.WeChatPayAPIv3Key)) == 32 &&
		strings.TrimSpace(setting.WeChatPayPlatformPublicKey) != ""
}

func isAlipayDirectEnabled() bool {
	return isPaymentComplianceConfirmed() && setting.AlipayDirectEnabled
}

func isAlipayDirectConfigured() bool {
	return isPaymentComplianceConfirmed() &&
		strings.TrimSpace(setting.AlipayAppID) != "" &&
		strings.TrimSpace(setting.AlipayPrivateKey) != "" &&
		strings.TrimSpace(setting.AlipayPublicKey) != ""
}

func parseRSAPrivateKey(value string) (*rsa.PrivateKey, error) {
	trimmed := strings.TrimSpace(value)
	block, _ := pem.Decode([]byte(trimmed))
	if block == nil && trimmed != "" {
		block, _ = pem.Decode([]byte("-----BEGIN PRIVATE KEY-----\n" + trimmed + "\n-----END PRIVATE KEY-----"))
	}
	if block == nil {
		return nil, errors.New("无法解析 RSA 私钥")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("私钥不是 RSA 格式")
	}
	return key, nil
}

func parseRSAPublicKey(value string) (*rsa.PublicKey, error) {
	trimmed := strings.TrimSpace(value)
	block, _ := pem.Decode([]byte(trimmed))
	if block == nil && trimmed != "" {
		block, _ = pem.Decode([]byte("-----BEGIN PUBLIC KEY-----\n" + trimmed + "\n-----END PUBLIC KEY-----"))
	}
	if block == nil {
		return nil, errors.New("无法解析 RSA 公钥")
	}
	if certificate, err := x509.ParseCertificate(block.Bytes); err == nil {
		key, ok := certificate.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("证书公钥不是 RSA 格式")
		}
		return key, nil
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("公钥不是 RSA 格式")
	}
	return key, nil
}

func signRSA256(message string, privateKeyValue string) (string, error) {
	privateKey, err := parseRSAPrivateKey(privateKeyValue)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func verifyRSA256(message string, signatureValue string, publicKeyValue string) error {
	publicKey, err := parseRSAPublicKey(publicKeyValue)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(signatureValue)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(message))
	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature)
}

func verifyWeChatPayHTTPMessage(header http.Header, body []byte) error {
	timestampValue := header.Get("Wechatpay-Timestamp")
	nonce := header.Get("Wechatpay-Nonce")
	serial := header.Get("Wechatpay-Serial")
	signature := header.Get("Wechatpay-Signature")
	if timestampValue == "" || nonce == "" || serial == "" || signature == "" {
		return errors.New("微信支付签名头不完整")
	}
	timestamp, err := strconv.ParseInt(timestampValue, 10, 64)
	if err != nil || timestamp <= 0 {
		return errors.New("微信支付时间戳无效")
	}
	if time.Since(time.Unix(timestamp, 0)).Abs() > 5*time.Minute {
		return errors.New("微信支付签名已过期")
	}
	if setting.WeChatPayPlatformPublicKeyID != "" && serial != setting.WeChatPayPlatformPublicKeyID {
		return errors.New("微信支付平台公钥 ID 不匹配")
	}
	message := timestampValue + "\n" + nonce + "\n" + string(body) + "\n"
	return verifyRSA256(message, signature, setting.WeChatPayPlatformPublicKey)
}

func newOfficialTopUp(c *gin.Context, req officialPayRequest, paymentMethod string, paymentProvider string) (*model.TopUp, error) {
	if req.PaymentMethod != paymentMethod {
		return nil, errors.New("不支持的支付渠道")
	}
	if req.Amount < getMinTopup() {
		return nil, fmt.Errorf("充值数量不能小于 %d", getMinTopup())
	}
	userID := c.GetInt("id")
	creditedQuota, err := validateTopUpQuota(req.Amount)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateTopUpQuotaCapacity(userID, creditedQuota); err != nil {
		return nil, err
	}
	group, err := model.GetUserGroup(userID, true)
	if err != nil {
		return nil, errors.New("获取用户分组失败")
	}
	payMoney := decimal.NewFromFloat(getPayMoney(req.Amount, group)).Round(2)
	if payMoney.LessThan(decimal.NewFromFloat(0.01)) {
		return nil, errors.New("充值金额过低")
	}
	tradeNo := fmt.Sprintf("USR%d%s%d", userID, common.GetRandomString(6), time.Now().Unix())
	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		amount = decimal.NewFromInt(amount).Div(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
	}
	return &model.TopUp{
		UserId:          userID,
		Amount:          amount,
		Money:           payMoney.InexactFloat64(),
		TradeNo:         tradeNo,
		PaymentMethod:   paymentMethod,
		PaymentProvider: paymentProvider,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}, nil
}

func RequestAlipayDirect(c *gin.Context) {
	if !isAlipayDirectEnabled() {
		common.ApiErrorMsg(c, "支付宝官方支付未启用")
		return
	}
	if !isAlipayDirectConfigured() {
		common.ApiErrorMsg(c, "支付宝官方支付未配置")
		return
	}
	var req officialPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	topUp, err := newOfficialTopUp(c, req, model.PaymentMethodAlipayDirect, model.PaymentProviderAlipayDirect)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	bizContent, err := common.Marshal(map[string]interface{}{
		"out_trade_no": topUp.TradeNo,
		"total_amount": decimal.NewFromFloat(topUp.Money).StringFixed(2),
		"subject":      fmt.Sprintf("账户充值 %d", req.Amount),
		"product_code": "FAST_INSTANT_TRADE_PAY",
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	params := url.Values{
		"app_id":      {setting.AlipayAppID},
		"method":      {"alipay.trade.page.pay"},
		"format":      {"JSON"},
		"charset":     {"utf-8"},
		"sign_type":   {"RSA2"},
		"timestamp":   {time.Now().Format("2006-01-02 15:04:05")},
		"version":     {"1.0"},
		"notify_url":  {service.GetCallbackAddress() + "/api/alipay/notify"},
		"return_url":  {paymentReturnPath("/wallet")},
		"biz_content": {string(bizContent)},
	}
	signature, err := signRSA256(canonicalAlipayValues(params), setting.AlipayPrivateKey)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝签名失败 error=%q", err.Error()))
		common.ApiErrorMsg(c, "支付宝私钥配置无效")
		return
	}
	params.Set("sign", signature)
	if err := topUp.Insert(); err != nil {
		common.ApiErrorMsg(c, "创建订单失败")
		return
	}
	common.ApiSuccess(c, gin.H{"pay_link": alipayGatewayURL + "?" + params.Encode()})
}

func canonicalAlipayValues(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "sign" && key != "sign_type" && values.Get(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	return strings.Join(parts, "&")
}

func AlipayDirectNotify(c *gin.Context) {
	if !isAlipayDirectConfigured() || c.Request.ParseForm() != nil {
		c.String(http.StatusOK, "failure")
		return
	}
	form := c.Request.PostForm
	if form.Get("app_id") != setting.AlipayAppID || verifyRSA256(canonicalAlipayValues(form), form.Get("sign"), setting.AlipayPublicKey) != nil {
		logger.LogWarn(c.Request.Context(), "支付宝官方回调验签失败")
		c.String(http.StatusOK, "failure")
		return
	}
	if form.Get("trade_status") != "TRADE_SUCCESS" && form.Get("trade_status") != "TRADE_FINISHED" {
		c.String(http.StatusOK, "success")
		return
	}
	paidMoney, err := decimal.NewFromString(form.Get("total_amount"))
	if err != nil {
		c.String(http.StatusOK, "failure")
		return
	}
	_, err = model.RechargeOfficialPayment(form.Get("out_trade_no"), model.PaymentProviderAlipayDirect, paidMoney, c.ClientIP())
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝官方回调入账失败 trade_no=%s error=%q", form.Get("out_trade_no"), err.Error()))
		c.String(http.StatusOK, "failure")
		return
	}
	c.String(http.StatusOK, "success")
}

func RequestWeChatPayDirect(c *gin.Context) {
	if !isWeChatPayDirectEnabled() {
		common.ApiErrorMsg(c, "微信支付官方支付未启用")
		return
	}
	if !isWeChatPayDirectConfigured() {
		common.ApiErrorMsg(c, "微信支付官方支付未配置")
		return
	}
	var req officialPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	topUp, err := newOfficialTopUp(c, req, model.PaymentMethodWechatDirect, model.PaymentProviderWechatDirect)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	requestBody := wechatNativePayRequest{
		AppID:       setting.WeChatPayAppID,
		MchID:       setting.WeChatPayMchID,
		Description: fmt.Sprintf("账户充值 %d", req.Amount),
		OutTradeNo:  topUp.TradeNo,
		NotifyURL:   service.GetCallbackAddress() + "/api/wechat-pay/notify",
	}
	requestBody.Amount.Total = decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	requestBody.Amount.Currency = "CNY"
	if err := topUp.Insert(); err != nil {
		common.ApiErrorMsg(c, "创建订单失败")
		return
	}
	body, err := common.Marshal(requestBody)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	requestURL := "https://api.mch.weixin.qq.com" + wechatNativePayPath
	httpRequest, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := common.GetRandomString(24)
	message := http.MethodPost + "\n" + wechatNativePayPath + "\n" + timestamp + "\n" + nonce + "\n" + string(body) + "\n"
	signature, err := signRSA256(message, setting.WeChatPayPrivateKey)
	if err != nil {
		common.ApiErrorMsg(c, "微信支付商户私钥配置无效")
		return
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Authorization", fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",timestamp="%s",serial_no="%s",signature="%s"`, setting.WeChatPayMchID, nonce, timestamp, setting.WeChatPayMchSerialNo, signature))
	response, err := service.GetHttpClient().Do(httpRequest)
	if err != nil {
		common.ApiErrorMsg(c, "请求微信支付失败")
		return
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := verifyWeChatPayHTTPMessage(response.Header, responseBody); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付下单响应验签失败 error=%q", err.Error()))
		common.ApiErrorMsg(c, "微信支付下单响应验签失败")
		return
	}
	var payResponse wechatNativePayResponse
	if err := common.Unmarshal(responseBody, &payResponse); err != nil || response.StatusCode != http.StatusOK || payResponse.CodeURL == "" {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付下单失败 status=%d body=%q", response.StatusCode, string(responseBody)))
		common.ApiErrorMsg(c, "微信支付下单失败: "+payResponse.Message)
		return
	}
	common.ApiSuccess(c, gin.H{"code_url": payResponse.CodeURL, "trade_no": topUp.TradeNo})
}

func WeChatPayDirectNotify(c *gin.Context) {
	if !isWeChatPayDirectConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FAIL", "message": "payment disabled"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "invalid body"})
		return
	}
	if err := verifyWeChatPayHTTPMessage(c.Request.Header, body); err != nil {
		logger.LogWarn(c.Request.Context(), "微信支付官方回调验签失败")
		c.JSON(http.StatusUnauthorized, gin.H{"code": "FAIL", "message": "invalid signature"})
		return
	}
	var notification wechatNotification
	if common.Unmarshal(body, &notification) != nil || notification.EventType != "TRANSACTION.SUCCESS" {
		c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
		return
	}
	if notification.Resource.Algorithm != "AEAD_AES_256_GCM" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "unsupported algorithm"})
		return
	}
	plaintext, err := decryptWeChatResource(notification.Resource.Ciphertext, notification.Resource.Nonce, notification.Resource.AssociatedData, setting.WeChatPayAPIv3Key)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "decrypt failed"})
		return
	}
	var transaction wechatTransaction
	if common.Unmarshal(plaintext, &transaction) != nil || transaction.AppID != setting.WeChatPayAppID || transaction.MchID != setting.WeChatPayMchID || transaction.TradeState != "SUCCESS" || transaction.Amount.Currency != "CNY" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "invalid transaction"})
		return
	}
	paidMoney := decimal.NewFromInt(transaction.Amount.Total).Div(decimal.NewFromInt(100))
	_, err = model.RechargeOfficialPayment(transaction.OutTradeNo, model.PaymentProviderWechatDirect, paidMoney, c.ClientIP())
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付官方回调入账失败 trade_no=%s error=%q", transaction.OutTradeNo, err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "settlement failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
}

func decryptWeChatResource(ciphertextValue string, nonce string, associatedData string, apiV3Key string) ([]byte, error) {
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertextValue)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher([]byte(apiV3Key))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, []byte(nonce), ciphertextBytes, []byte(associatedData))
}
