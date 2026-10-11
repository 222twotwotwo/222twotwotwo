---
title: 接入微信支付
date: 2026-10-11
category: Go 后端
tags: Go, 微信支付, 支付, APIv3, 服务商模式
readTime: 12 分钟阅读
summary: 依据微信支付合作伙伴文档，梳理服务商模式下 JSAPI、Native、H5、APP 与付款码五种支付方式的场景差异与接入流程，并给出基于官方 wechatpay-go SDK 的 Go 示例代码。
cover: ../images/logos/wechatpay-go-cover.png
---

微信支付针对不同用户场景提供了多种支付接入方式。在[合作伙伴文档中心](https://pay.weixin.qq.com/doc/v3/partner/4012069852)里，它们是并列的几个产品：JSAPI 支付、Native 支付、H5 支付、APP 支付、小程序支付，以及面向线下收银的付款码支付。

服务商模式（也叫机构模式）里有两个角色：**服务商**（`sp_mchid`，自己不直接收款）和**特约商户/子商户**（`sub_mchid`，真正收款的商户）。服务商为一批子商户提供统一的支付能力，所有下单接口都要求同时带上服务商与子商户的编号。如果是自己一家商户直连微信支付，则用[商户文档](https://pay.weixin.qq.com/doc/v3/merchant/4012791504)，接口路径少一层 `partner`。

这篇文章以服务商模式为准，逐个梳理五种支付方式的适用场景、下单流程，并给出 Go 示例。

## 五种支付方式一览

- **JSAPI 支付**：用户在**微信内置浏览器**打开的网页里完成支付。需要用户的 openid，下单返回 `prepay_id`，前端通过 `WeixinJSBridge` 调起收银台。
- **小程序支付**：用户在**微信小程序**内支付。与 JSAPI 共用同一个下单接口 `/v3/pay/partner/transactions/jsapi`，区别只在前端调起方式用 `wx.requestPayment`。
- **Native 支付**：用户在**PC 端网页**扫码支付。不需要 openid，下单返回二维码链接 `code_url`，商户把它生成二维码展示，有效期 2 小时。
- **H5 支付**：用户在**微信外的手机浏览器**里支付。不需要 openid，下单返回跳转链接 `h5_url`（有效期仅 5 分钟），前端直接跳转即可拉起微信。
- **APP 支付**：用户在**商户自有移动应用**内拉起微信支付。下单返回 `prepay_id`，客户端用微信开放平台 OpenSDK 调起。
- **付款码支付**：线下场景，收银员用扫码设备读取**用户出示的付款码**发起扣款，是唯一由商户侧"主动扣钱"的方式，接口同步返回支付结果。

几种方式的公共部分完全一致：查询订单、关闭订单、退款、支付回调通知，接口与支付方式无关。

## 开发前准备

服务商模式下需要的核心参数：

- `sp_appid` / `sp_mchid`：服务商的 APPID 与商户号，二者必须已完成绑定。
- `sub_mchid`：子商户号，由服务商为特约商户进件后获得。
- `sub_appid`（可选）：子商户自己的 APPID，与 `sub_mchid` 绑定。JSAPI 场景下传了 `sub_appid` 就要传该 APPID 下的 `sub_openid`。
- 商户 API 证书私钥 `apiclient_key.pem` 与证书序列号，用于请求签名。
- APIv3 密钥，用于回调报文的 AES-GCM 解密。
- 微信支付公钥或平台证书，用于响应与回调的验签。

Go 官方 SDK 是 `github.com/wechatpay-apiv3/wechatpay-go`。有一个值得提前知道的点：**SDK 的高级封装（`services/payments/*`）只覆盖直连模式**，请求结构体里只有 `Appid`/`Mchid` 字段。服务商模式可以继续用它的 `core.Client`——签名、敏感信息加密、回调验签解密都能复用——只是下单请求体需要自己定义结构体。这也是本文示例的做法。

```bash
go get github.com/wechatpay-apiv3/wechatpay-go
```

初始化客户端，`WithWechatPayAutoAuthCipher` 会自动下载平台证书并处理应答验签：

```go
package wxpay

import (
	"context"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

type Config struct {
	SpAppid    string // 服务商 APPID
	SpMchid    string // 服务商商户号
	SubMchid   string // 子商户号
	SerialNo   string // 商户证书序列号
	APIv3Key   string // APIv3 密钥
	PrivateKey string // apiclient_key.pem 路径
}

func NewClient(ctx context.Context, cfg Config) (*core.Client, error) {
	privateKey, err := utils.LoadPrivateKeyWithPath(cfg.PrivateKey)
	if err != nil {
		return nil, err
	}
	return core.NewClient(ctx,
		option.WithWechatPayAutoAuthCipher(cfg.SpMchid, cfg.SerialNo, privateKey, cfg.APIv3Key),
	)
}
```

再定义一个所有下单接口共用的请求结构体（各方式字段基本一致，JSAPI 多一个 `payer`，H5 多一个 `scene_info`）：

```go
type OrderAmount struct {
	Total    int64  `json:"total"` // 单位：分
	Currency string `json:"currency,omitempty"`
}

type OrderPayer struct {
	SpOpenid  string `json:"sp_openid,omitempty"`
	SubOpenid string `json:"sub_openid,omitempty"`
}

type PartnerPrepayRequest struct {
	SpAppid     string       `json:"sp_appid"`
	SpMchid     string       `json:"sp_mchid"`
	SubMchid    string       `json:"sub_mchid"`
	Description string       `json:"description"`
	OutTradeNo  string       `json:"out_trade_no"`
	NotifyUrl   string       `json:"notify_url"`
	Amount      OrderAmount  `json:"amount"`
	Payer       *OrderPayer  `json:"payer,omitempty"`
}
```

## JSAPI / 小程序支付

JSAPI 支付的完整流程：用户在微信内网页（或小程序）确认订单，后端调用下单接口拿到 `prepay_id`（有效期 2 小时），前端用 `prepay_id` 组装参数调起微信收银台，支付成功后微信异步回调 `notify_url`，商户以回调与主动查单为准更新订单。

服务商模式下，JSAPI 下单接口是 `POST /v3/pay/partner/transactions/jsapi`，`payer` 必填：传了 `sub_appid` 就填 `sub_openid`，否则填 `sp_openid`。

```go
// JsapiPrepay JSAPI/小程序下单，返回 prepay_id
func JsapiPrepay(ctx context.Context, client *core.Client, cfg Config, order PartnerPrepayRequest) (string, error) {
	order.SpAppid, order.SpMchid, order.SubMchid = cfg.SpAppid, cfg.SpMchid, cfg.SubMchid
	order.Payer = &OrderPayer{SubOpenid: "oUpF8uMuAJO_M2pxb1Q9zNjWeS6o"}

	apiResult, err := client.Post(ctx, "/v3/pay/partner/transactions/jsapi", order)
	if err != nil {
		return "", err
	}
	var out struct {
		PrepayID string `json:"prepay_id"`
	}
	if err := json.Unmarshal(apiResult.Response.Body, &out); err != nil {
		return "", err
	}
	return out.PrepayID, nil
}
```

前端拿到 `prepay_id` 后，还需要后端**对调起参数再做一次 RSA 签名**生成 `paySign`，微信内网页与小程序的签名串格式相同：

```go
// JsapiSign 生成 JSAPI/小程序调起支付所需的 paySign
// 签名串：appId \n timeStamp \n nonceStr \n prepay_id=xxx \n
func JsapiSign(privateKey *rsa.PrivateKey, appid, prepayID string) (timeStamp, nonceStr, paySign string, err error) {
	timeStamp = strconv.FormatInt(time.Now().Unix(), 10)
	nonceStr = randHex(16)
	message := appid + "\n" + timeStamp + "\n" + nonceStr + "\nprepay_id=" + prepayID + "\n"

	digest := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", "", "", err
	}
	return timeStamp, nonceStr, base64.StdEncoding.EncodeToString(sig), nil
}
```

微信内网页用 `WeixinJSBridge.invoke('getBrandWCPayRequest', ...)` 调起，小程序用 `wx.requestPayment(...)` 调起，两组参数除了字段名大小写外内容一致。

还有一个服务商模式特有的注意点：JSAPI 支付完成后微信会展示官方的支付成功页，服务商必须接入[点金计划](https://pay.weixin.qq.com/doc/v3/partner/4012072130)并实现商家小票的跳转能力，用户才能返回到商户指定页面。

## Native 支付

Native 适合 PC 网页：后端调 `POST /v3/pay/partner/transactions/native`，拿到的不是 `prepay_id` 而是二维码链接 `code_url`，把它渲染成二维码，用户扫码支付。`code_url` 有效期 2 小时，过期后需要重新下单。

```go
// NativePrepay Native 下单，返回 code_url
func NativePrepay(ctx context.Context, client *core.Client, cfg Config, order PartnerPrepayRequest) (string, error) {
	order.SpAppid, order.SpMchid, order.SubMchid = cfg.SpAppid, cfg.SpMchid, cfg.SubMchid

	apiResult, err := client.Post(ctx, "/v3/pay/partner/transactions/native", order)
	if err != nil {
		return "", err
	}
	var out struct {
		CodeURL string `json:"code_url"`
	}
	if err := json.Unmarshal(apiResult.Response.Body, &out); err != nil {
		return "", err
	}
	return out.CodeURL, nil
}
```

前端任意二维码库都能用，比如把 `code_url` 交给 `qrcode` 这类 JS 库渲染。由于 Native 没有支付完成的前端回调，页面感知支付结果要靠**轮询查单接口**或服务端推送。

## H5 支付

H5 支付面向微信外的手机浏览器。接口 `POST /v3/pay/partner/transactions/h5` 返回 `h5_url`，有效期仅 5 分钟，前端直接跳转（`window.location.href = h5_url`）即可拉起微信。下单时 `scene_info.payer_client_ip` 必填，要传用户终端的真实 IP。

```go
type H5SceneInfo struct {
	PayerClientIP string `json:"payer_client_ip"` // 用户终端 IP，必填
	DeviceID      string `json:"device_id,omitempty"`
}

// H5Prepay H5 下单，返回 h5_url
func H5Prepay(ctx context.Context, client *core.Client, cfg Config, order PartnerPrepayRequest, clientIP string) (string, error) {
	order.SpAppid, order.SpMchid, order.SubMchid = cfg.SpAppid, cfg.SpMchid, cfg.SubMchid

	req := struct {
		PartnerPrepayRequest
		SceneInfo H5SceneInfo `json:"scene_info"`
	}{order, H5SceneInfo{PayerClientIP: clientIP}}

	apiResult, err := client.Post(ctx, "/v3/pay/partner/transactions/h5", req)
	if err != nil {
		return "", err
	}
	var out struct {
		H5URL string `json:"h5_url"`
	}
	if err := json.Unmarshal(apiResult.Response.Body, &out); err != nil {
		return "", err
	}
	return out.H5URL, nil
}
```

支付完成后同样靠回调和查单确认结果。注意微信对 H5 支付有同层限制：referer 必须来自商户配置的 H5 支付域名，否则会报错。

## APP 支付

APP 支付要求商户在微信开放平台有已认证的移动应用。后端调 `POST /v3/pay/partner/transactions/app` 拿到 `prepay_id`，客户端用 OpenSDK 的 `sendReq` 拉起微信。服务商 App 里传 `sp_appid`，子商户 App 里传 `sub_appid`，调起所用 APPID 必须与下单时一致。

```go
// AppPrepay APP 下单，返回 prepay_id
func AppPrepay(ctx context.Context, client *core.Client, cfg Config, order PartnerPrepayRequest) (string, error) {
	order.SpAppid, order.SpMchid, order.SubMchid = cfg.SpAppid, cfg.SpMchid, cfg.SubMchid

	apiResult, err := client.Post(ctx, "/v3/pay/partner/transactions/app", order)
	if err != nil {
		return "", err
	}
	var out struct {
		PrepayID string `json:"prepay_id"`
	}
	if err := json.Unmarshal(apiResult.Response.Body, &out); err != nil {
		return "", err
	}
	return out.PrepayID, nil
}
```

客户端调起前，服务端同样要生成一组签名参数。APP 的签名串比 JSAPI 少一行 `prepay_id=` 前缀，`package` 固定为 `Sign=WXPay` 且不参与签名：

```go
// AppSign 生成 APP 调起支付所需参数
// 签名串：appid \n timestamp \n noncestr \n prepayid \n
func AppSign(privateKey *rsa.PrivateKey, appid, partnerID, prepayID string) (map[string]string, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonceStr := randHex(16)
	message := appid + "\n" + timestamp + "\n" + nonceStr + "\n" + prepayID + "\n"

	digest := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"appid":     appid,
		"partnerid": partnerID,
		"prepayid":  prepayID,
		"package":   "Sign=WXPay",
		"noncestr":  nonceStr,
		"timestamp": timestamp,
		"sign":      base64.StdEncoding.EncodeToString(sig),
	}, nil
}
```

Android 端记得实现 `WXPayEntryActivity` 接收支付结果回调，iOS 端在 `onResp` 里处理。

## 付款码支付

付款码是线下收银场景：用户打开微信钱包出示条形码/二维码，收银员用扫码枪读取授权码 `auth_code`，商户后台直接发起扣款。接口 `POST /v3/pay/partner/transactions/codepay` 与其他方式不同，它**同步返回支付结果**，不需要前端参与。

```go
type CodePayRequest struct {
	SpAppid     string      `json:"sp_appid"`
	SpMchid     string      `json:"sp_mchid"`
	SubMchid    string      `json:"sub_mchid"`
	Description string      `json:"description"`
	OutTradeNo  string      `json:"out_trade_no"`
	NotifyUrl   string      `json:"notify_url,omitempty"`
	Amount      OrderAmount `json:"amount"`
	Payer       struct {
		AuthCode string `json:"auth_code"` // 用户付款码，18 位数字
	} `json:"payer"`
}

// CodePay 提交付款码支付，同步返回交易状态
func CodePay(ctx context.Context, client *core.Client, cfg Config, req CodePayRequest) (map[string]interface{}, error) {
	req.SpAppid, req.SpMchid, req.SubMchid = cfg.SpAppid, cfg.SpMchid, cfg.SubMchid

	apiResult, err := client.Post(ctx, "/v3/pay/partner/transactions/codepay", req)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(apiResult.Response.Body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
```

返回的 `trade_state` 需要重点处理三种情况：

- `SUCCESS`：支付成功，直接出小票。
- `USERPAYING`：用户支付中（例如需要输密码），必须**用原订单号查单**确认最终结果，不能重复扣款。
- `SYSTEMERROR`、`BANKERROR`：结果未知，同样先查单再决定重试或撤销。

付款码还有独有的**撤销订单**接口 `POST /v3/pay/partner/transactions/codepay/reverse`，用于当天内冲正误扣的订单。

## 查单、关单与退款

这三个接口与支付方式无关，服务商用同一套代码处理所有订单。查单按商户订单号：

```go
// QueryOrder 按商户订单号查询订单状态
func QueryOrder(ctx context.Context, client *core.Client, cfg Config, outTradeNo string) (map[string]interface{}, error) {
	url := "/v3/pay/partner/transactions/out-trade-no/" + outTradeNo +
		"?sub_mchid=" + cfg.SubMchid
	apiResult, err := client.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(apiResult.Response.Body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
```

订单不需要再支付时主动关单，避免用户在过期前误付：

```go
// CloseOrder 关闭订单
func CloseOrder(ctx context.Context, client *core.Client, cfg Config, outTradeNo string) error {
	req := struct {
		SpMchid  string `json:"sp_mchid"`
		SubMchid string `json:"sub_mchid"`
	}{cfg.SpMchid, cfg.SubMchid}
	_, err := client.Post(ctx,
		"/v3/pay/partner/transactions/out-trade-no/"+outTradeNo+"/close", req)
	return err
}
```

退款走独立域名的退款接口，服务商模式必须带 `sub_mchid`：

```go
// Refund 申请退款
func Refund(ctx context.Context, client *core.Client, cfg Config, outTradeNo, outRefundNo string, refundAmount, totalAmount int64) error {
	req := struct {
		SubMchid    string `json:"sub_mchid"`
		OutTradeNo  string `json:"out_trade_no"`
		OutRefundNo string `json:"out_refund_no"`
		Amount      struct {
			Refund   int64  `json:"refund"`
			Total    int64  `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}{}
	req.SubMchid, req.OutTradeNo, req.OutRefundNo = cfg.SubMchid, outTradeNo, outRefundNo
	req.Amount.Refund, req.Amount.Total, req.Amount.Currency = refundAmount, totalAmount, "CNY"

	_, err := client.Post(ctx, "/v3/refund/domestic/refunds", req)
	return err
}
```

## 支付回调通知

无论哪种支付方式，支付结果都通过 `notify_url` 异步通知。回调报文经过 APIv3 密钥的 AES-256-GCM 加密，SDK 的 `notify.NotifyHandler` 可以一步完成验签和解密。直连模式的 `payments.Transaction` 结构体不含服务商字段，这里同样自己定义：

```go
type PartnerPayNotify struct {
	SpAppid       string `json:"sp_appid"`
	SpMchid       string `json:"sp_mchid"`
	SubAppid      string `json:"sub_appid,omitempty"`
	SubMchid      string `json:"sub_mchid"`
	OutTradeNo    string `json:"out_trade_no"`
	TransactionID string `json:"transaction_id"`
	TradeState    string `json:"trade_state"`
	SuccessTime   string `json:"success_time"`
}

func HandlePayNotify(w http.ResponseWriter, r *http.Request, client *core.Client, cfg Config) {
	// certVisitor 来自初始化时的自动下载，同一进程内复用
	certVisitor := downloader.MgrInstance().GetCertificateVisitor(cfg.SpMchid)
	handler := notify.NewNotifyHandler(cfg.APIv3Key, verifiers.NewSHA256WithRSAVerifier(certVisitor))

	result := new(PartnerPayNotify)
	if _, err := handler.ParseNotifyRequest(r.Context(), r, result); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// 以 result.TradeState 驱动订单状态机；注意先做幂等处理
	if result.TradeState == "SUCCESS" {
		markOrderPaid(result.OutTradeNo, result.TransactionID)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"code": "SUCCESS"}`)) // 必须应答，否则微信会重试
}
```

两个工程细节：回调必须以"查单兜底"设计，微信的通知是**至少一次**投递，处理逻辑要幂等；收到通知后要返回 `{"code": "SUCCESS"}` 应答，否则微信会持续重发。

## 怎么选

- 用户在微信小程序里 → 小程序支付（JSAPI 下单接口）。
- 用户在微信内打开的网页里 → JSAPI 支付。
- 用户在 PC 网页上 → Native 支付，生成二维码扫码。
- 用户在手机自带浏览器等微信外环境 → H5 支付。
- 商户有自己的 APP → APP 支付。
- 线下柜台，用户出示付款码 → 付款码支付。

对于服务商来说，五种方式的下单参数高度一致，差异集中在三处：接口路径后缀、下单产物（`prepay_id` / `code_url` / `h5_url` / 同步结果）、前端调起方式。把下单、查单、回调做成同一套骨架，只替换路径与产物字段，是最省力的接入方式。

## 参考资料

- [JSAPI 支付产品介绍（服务商）](https://pay.weixin.qq.com/doc/v3/partner/4012069852)
- [JSAPI 下单 API（服务商）](https://pay.wechatpay.cn/doc/v3/partner/4012714678)
- [Native 下单 API（服务商）](https://pay.weixin.qq.com/doc/v3/partner/4012738659)
- [H5 下单 API（服务商）](https://pay.wechatpay.cn/docs/partner/apis/partner-h5-payment/partner-jsons/partner-h5-prepay.html)
- [APP 下单 API（服务商）](https://pay.weixin.qq.com/doc/v3/partner/4012714669)
- [wechatpay-go 官方 SDK](https://github.com/wechatpay-apiv3/wechatpay-go)
- [点金计划（服务商）](https://pay.weixin.qq.com/doc/v3/partner/4012072130)
