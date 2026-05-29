package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type WeChatServiceConfig struct {
	AppID             string
	AppSecret         string
	MchID             string
	MchSerialNo       string
	APIV3Key          string
	PrivateKeyPath    string
	NotifyURL         string
	DefaultTemplateID string
	MockMode          bool
}

type CreateOrderInput struct {
	OutTradeNo   string
	OpenID       string
	AmountFen    int64
	Description  string
	NotifyURL    string
	Attach       string
	ClientIP     string
	ExpireMinute int64
}

type CreateOrderOutput struct {
	PrepayID  string                 `json:"prepayId"`
	PayParams map[string]interface{} `json:"payParams"`
}

type WeChatPayService struct {
	cfg    WeChatServiceConfig
	client *http.Client
}

func NewWeChatPayService(cfg WeChatServiceConfig) *WeChatPayService {
	return &WeChatPayService{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WeChatPayService) Login(code string) (string, string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", "", errors.New("code is required")
	}

	if w.cfg.MockMode {
		return "mock_" + code, "", nil
	}

	if w.cfg.AppID == "" || w.cfg.AppSecret == "" {
		return "", "", errors.New("wechat app_id/app_secret not configured")
	}

	loginURL := "https://api.weixin.qq.com/sns/jscode2session?appid=" + url.QueryEscape(w.cfg.AppID) +
		"&secret=" + url.QueryEscape(w.cfg.AppSecret) +
		"&js_code=" + url.QueryEscape(code) + "&grant_type=authorization_code"
	resp, err := w.client.Get(loginURL)
	if err != nil {
		return "", "", fmt.Errorf("wechat login request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var result struct {
		OpenID    string `json:"openid"`
		UnionID   string `json:"unionid"`
		ErrCode   int    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		SessionID string `json:"session_key"`
	}
	if err = json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("wechat login decode failed: %w", err)
	}

	if result.ErrCode != 0 {
		return "", "", fmt.Errorf("wechat login failed: %d %s", result.ErrCode, result.ErrMsg)
	}
	if result.OpenID == "" {
		return "", "", errors.New("wechat login returned empty openid")
	}

	return result.OpenID, result.UnionID, nil
}

func (w *WeChatPayService) CreateOrder(input CreateOrderInput) (*CreateOrderOutput, error) {
	if input.OutTradeNo == "" || input.OpenID == "" || input.AmountFen <= 0 {
		return nil, errors.New("invalid order input")
	}

	if w.cfg.MockMode {
		now := time.Now().Unix()
		prepayID := "mock_prepay_" + input.OutTradeNo
		nonce := fmt.Sprintf("mock_nonce_%d", now)
		pkg := "prepay_id=" + prepayID
		payParams := map[string]interface{}{
			"timeStamp": fmt.Sprintf("%d", now),
			"nonceStr":  nonce,
			"package":   pkg,
			"signType":  "RSA",
			"paySign":   "MOCK_SIGN",
		}
		return &CreateOrderOutput{PrepayID: prepayID, PayParams: payParams}, nil
	}

	return nil, errors.New("real wechat pay create order is not implemented yet, enable mock_mode for local development")
}

func (w *WeChatPayService) QueryOrder(outTradeNo string) (string, string, error) {
	if outTradeNo == "" {
		return "", "", errors.New("outTradeNo is required")
	}

	if w.cfg.MockMode {
		return "USERPAYING", "", nil
	}

	return "", "", errors.New("real wechat pay order query is not implemented yet")
}

func (w *WeChatPayService) SendSubscribeMessage(openid, templateID string, data map[string]string) error {
	if openid == "" {
		return errors.New("openid is required")
	}
	if templateID == "" {
		return errors.New("templateID is required")
	}

	if w.cfg.MockMode {
		return nil
	}

	return errors.New("real subscribe message send is not implemented yet")
}

func (w *WeChatPayService) DefaultTemplateID() string {
	return w.cfg.DefaultTemplateID
}

func (w *WeChatPayService) RetryDelay() time.Duration {
	return 5 * time.Minute
}
