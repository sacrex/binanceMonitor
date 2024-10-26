package misc

import (
	"encoding/json"
	l "github.com/crazygit/binance-market-monitor/helper/log"
	"io"
	"net/http"
)

var log = l.GetLog()

type BAPIBaseResponse struct {
	Code          string      `json:"code"`
	Data          interface{} `json:"data"`
	Message       interface{} `json:"message"`
	MessageDetail interface{} `json:"messageDetail"`
	Success       bool        `json:"success"`
}

type Asset struct {
	Id                   string      `json:"id"`
	AssetCode            string      `json:"assetCode"`
	AssetName            string      `json:"assetName"`
	Unit                 string      `json:"unit"`
	CommissionRate       float64     `json:"commissionRate"`
	FreeAuditWithdrawAmt float64     `json:"freeAuditWithdrawAmt"`
	FreeUserChargeAmount float64     `json:"freeUserChargeAmount"`
	CreateTime           int64       `json:"createTime"`
	Test                 int         `json:"test"`
	Gas                  interface{} `json:"gas"`
	IsLegalMoney         bool        `json:"isLegalMoney"`
	ReconciliationAmount float64     `json:"reconciliationAmount"`
	SeqNum               string      `json:"seqNum"`
	ChineseName          string      `json:"chineseName"`
	CnLink               string      `json:"cnLink"`
	EnLink               string      `json:"enLink"`
	LogoUrl              string      `json:"logoUrl"`
	FullLogoUrl          string      `json:"fullLogoUrl"`
	SupportMarket        interface{} `json:"supportMarket"`
	FeeReferenceAsset    interface{} `json:"feeReferenceAsset"`
	FeeRate              interface{} `json:"feeRate"`
	FeeDigit             int         `json:"feeDigit"`
	AssetDigit           int         `json:"assetDigit"`
	Trading              bool        `json:"trading"`
	Tags                 []string    `json:"tags"`
	PlateType            string      `json:"plateType"`
	Etf                  bool        `json:"etf"`
	IsLedgerOnly         bool        `json:"isLedgerOnly"`
	Delisted             bool        `json:"delisted"`
	PreDelist            bool        `json:"preDelist"`
	PdTradeDeadline      int64       `json:"pdTradeDeadline"`
	PdDepositDeadline    int64       `json:"pdDepositDeadline"`
	PdAnnounceUrl        string      `json:"pdAnnounceUrl"`
	TagBits              string      `json:"tagBits"`
}

func GetTags() (map[string][]string, error) {
	url := "https://www.binance.com/bapi/asset/v2/public/asset/asset/get-all-asset"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 读取响应体中的数据
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ret BAPIBaseResponse
	err = json.Unmarshal(body, &ret)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(ret.Data)
	var assets []Asset
	err = json.Unmarshal(data, &assets)
	if err != nil {
		return nil, err
	}

	symbolToTags := make(map[string][]string)
	for _, v := range assets {
		symbolToTags[v.AssetCode] = v.Tags
	}

	return symbolToTags, nil
}
