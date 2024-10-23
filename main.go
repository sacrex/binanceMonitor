package main

import (
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	binanceFuture "github.com/adshao/go-binance/v2/futures"
	"github.com/crazygit/binance-market-monitor/helper"
	l "github.com/crazygit/binance-market-monitor/helper/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	"math"
	"strconv"
	"strings"
	"time"
)

var log = l.GetLog()

// 报警间隔时间10min
const alertDurationMilli = 10 * 60 * 1000

var (
	quoteAsset = strings.ToUpper(helper.GetStringEnv("QUOTE_ASSET", "USDT"))
)

var lastAlert = map[string]ExtendWsMarketStatEvent{}

type ExtendWsMarketStatEvent struct {
	*binance.WsMarketStatEvent
	PriceChangePercentFloat float64
	LastPriceFloat          float64
	CloseQtyFloat           float64
}

func (e ExtendWsMarketStatEvent) url() string {
	return fmt.Sprintf("https://www.binance.com/zh-CN/trade/%s?theme=dark&type=spot", e.PrettySymbol("_"))
}

func (e ExtendWsMarketStatEvent) PrettySymbol(separator string) string {
	var replacer *strings.Replacer
	replacer = strings.NewReplacer(quoteAsset, fmt.Sprintf("%s%s", separator, quoteAsset))
	return replacer.Replace(e.Symbol)
}

func (e ExtendWsMarketStatEvent) AlertText(oldEvent ExtendWsMarketStatEvent) string {
	return fmt.Sprintf(`
*交易对*: %s

_最新报警信息_

*成交价格*: %s
*价格变化百分比*: %s
*成交价格上的成交量*: %s
*24小时内成交量*: %s
*24小时内成交额*: %s

_上次报警信息_

*成交价格*: %s
*价格变化百分比*: %s
*价格上的成交量*: %s

两次报警间隔时间: %s

[详情](%s)
`, escapeTextToMarkdownV2(e.PrettySymbol("/")),

		escapeTextToMarkdownV2("$"+prettyFloatString(e.LastPrice)+" "+getDifference(e.LastPriceFloat, oldEvent.LastPriceFloat, "")),                          // 最新成交价格
		escapeTextToMarkdownV2(prettyFloatString(e.PriceChangePercent)+"% "+getDifference(e.PriceChangePercentFloat, oldEvent.PriceChangePercentFloat, "%")), //  24小时价格变化(百分比)
		escapeTextToMarkdownV2(prettyFloatString(e.CloseQty)+" "+getDifference(e.CloseQtyFloat, oldEvent.CloseQtyFloat, "")),                                 // 最新成交价格上的成交量
		escapeTextToMarkdownV2(prettyFloatString(e.BaseVolume)),                                                                                              // 24小时内成交量
		escapeTextToMarkdownV2(prettyFloatString(e.QuoteVolume)),                                                                                             // 24小时内成交额

		escapeTextToMarkdownV2("$"+prettyFloatString(oldEvent.LastPrice)),          //上次报警价格
		escapeTextToMarkdownV2(prettyFloatString(oldEvent.PriceChangePercent)+"%"), //上次价格变化百分比
		escapeTextToMarkdownV2(prettyFloatString(oldEvent.CloseQty)),               //上次价格上的成交量

		escapeTextToMarkdownV2(time.UnixMilli(e.Time).Truncate(time.Second).Sub(time.UnixMilli(oldEvent.Time).Truncate(time.Second)).String()), //两次报警间隔时间
		e.url(), //链接
	)
}

func escapeTextToMarkdownV2(text string) string {
	return tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, text)
}

func getDifference(newValue, oldValue float64, suffix string) string {
	diff := newValue - oldValue
	direction := "🔻"
	if diff > 0 {
		direction = "🔺"
	}
	return fmt.Sprintf("(%s%.6f%s)", direction, math.Abs(diff), suffix)
}

func PercentStringify(PriceChangePercent float64, suffix string) string {
	direction := "🔻"
	if PriceChangePercent > 0 {
		direction = "🔺"
	}
	return fmt.Sprintf("%s%.6f%s", direction, math.Abs(PriceChangePercent), suffix)
}

func prettyFloatString(value string) string {
	if p, err := strconv.ParseFloat(value, 64); err != nil {
		return value
	} else {
		return fmt.Sprintf("%.6f", p)
	}
}

func isNeedAlert(newEvent ExtendWsMarketStatEvent) bool {
	if oldEvent, ok := lastAlert[newEvent.Symbol]; ok {
		priceChangePercent := math.Abs(newEvent.PriceChangePercentFloat - oldEvent.PriceChangePercentFloat)
		duration := newEvent.Time - oldEvent.Time
		if duration >= alertDurationMilli {
			if newEvent.LastPriceFloat <= 1 && priceChangePercent >= 3 {
				return true
			} else if newEvent.LastPriceFloat >= 300 && priceChangePercent >= 3 {
				return true
			} else if newEvent.LastPriceFloat > 1 && newEvent.LastPriceFloat >= 300 && priceChangePercent >= 3 {
				return true
			}
		}
		return false
	} else {
		// 首次启动时会触发大量报警，忽略程序启动时,波动已经大于预设值的报警
		lastAlert[newEvent.Symbol] = newEvent
		return false
	}
}

func isIgnoreEvent(event *binance.WsMarketStatEvent) bool {
	return !strings.HasSuffix(event.Symbol, quoteAsset)
}

func eventHandler(events binance.WsAllMarketsStatEvent) {
	var postMessageTextBuilder strings.Builder
	var postMessage = false
	log.WithFields(logrus.Fields{"SymbolsInAlertMap": len(lastAlert), "RevivedEventsNumber": len(events)}).Info("Stats")
	for _, event := range events {
		if isIgnoreEvent(event) {
			continue
		}
		priceChangePercentFloat, _ := strconv.ParseFloat(event.PriceChangePercent, 64)
		lastPriceFloat, _ := strconv.ParseFloat(event.LastPrice, 64)
		closeQtyFloat, _ := strconv.ParseFloat(event.CloseQty, 64)
		newEvent := ExtendWsMarketStatEvent{
			WsMarketStatEvent:       event,
			PriceChangePercentFloat: priceChangePercentFloat,
			LastPriceFloat:          lastPriceFloat,
			CloseQtyFloat:           closeQtyFloat,
		}
		log.WithFields(logrus.Fields{
			"Symbol":             newEvent.Symbol,
			"PriceChange":        prettyFloatString(newEvent.LastPrice),
			"PriceChangePercent": newEvent.PriceChangePercent,
			"LastPrice":          prettyFloatString(newEvent.LastPrice),
			"Time":               newEvent.Time,
			"CloseQty":           prettyFloatString(newEvent.CloseQty),
		}).Debug("Received Event")
		if isNeedAlert(newEvent) {
			postMessageTextBuilder.WriteString(newEvent.AlertText(lastAlert[newEvent.Symbol]))
			lastAlert[newEvent.Symbol] = newEvent
			postMessage = true
		}
	}
	if postMessage {
		postMessageTextBuilder.WriteString(fmt.Sprintf("\n\n%s", escapeTextToMarkdownV2(fmt.Sprintf("(%s)", time.Now().Format(time.RFC3339)))))
		if err := PostMessageToTgChannel(getTelegramChannelName(), postMessageTextBuilder.String()); err != nil {
			log.WithField("Error", err).Error("Post message to tg channel failed")
		}
	}
}

func getTelegramChannelName() string {
	channelName := helper.GetRequiredStringEnv("TELEGRAM_CHANNEL_USERNAME")
	if !strings.HasPrefix(channelName, "@") {
		return "@" + channelName
	}
	return channelName
}

func errHandler(err error) {
	log.Error(err)
}

var wsKlineText = `
*事件时间*: %s
*类型*: 5分钟的涨跌幅超过3%%
*交易对*: %s
*开盘价*: %s
*收盘价*: %s
*差\(收盘\-开盘\)*: %s
*涨/跌幅*: %s
*开始时间*: %s
*结束时间*: %s
`

func wsKlineHandler(event *binanceFuture.WsKlineEvent) {
	var postMessageTextBuilder strings.Builder
	var postMessage = false
	//log.WithFields(logrus.Fields{"Symbol": event.Symbol, "Price": event.Kline.Close,
	//	"Time": time.UnixMilli(event.Time).Format(time.DateTime)}).Info("Stats")

	closePrice, _ := strconv.ParseFloat(event.Kline.Close, 64)
	openPrice, _ := strconv.ParseFloat(event.Kline.Open, 64)

	PriceChangePercent := (closePrice - openPrice) / openPrice
	if math.Abs(PriceChangePercent) >= 0.03 {
		postMessageTextBuilder.WriteString(fmt.Sprintf(wsKlineText,
			escapeTextToMarkdownV2(time.UnixMilli(event.Time).Format(time.DateTime)),
			escapeTextToMarkdownV2(event.Symbol),
			escapeTextToMarkdownV2(event.Kline.Open),
			escapeTextToMarkdownV2(event.Kline.Close),
			escapeTextToMarkdownV2(getDifference(closePrice, openPrice, "")),
			escapeTextToMarkdownV2(PercentStringify(PriceChangePercent*100, "%")),
			escapeTextToMarkdownV2(time.UnixMilli(event.Kline.StartTime).Format(time.DateTime)),
			escapeTextToMarkdownV2(time.UnixMilli(event.Kline.EndTime).Format(time.DateTime)),
		))
		postMessage = true
	}

	if postMessage {
		if err := PostMessageToTgChannel(getTelegramChannelName(), postMessageTextBuilder.String()); err != nil {
			log.WithField("Error", err).Error("Post message to tg channel failed")
		}
	}
}

func errWsKlineHandler(err error) {
	log.Error(err)
}

func init() {
	binanceFuture.WebsocketKeepalive = true
}

func main() {
	exchangeInfo, err := binanceFuture.NewClient("", "").NewExchangeInfoService().Do(context.Background())
	if err != nil {
		log.WithField("Error", err).Error("Get ExchangeInfo failed")
		return
	}

	pairs := make([]map[string]string, 3)
	for i, v := range exchangeInfo.Symbols {
		if v.QuoteAsset == "USDT" {
			i = i % 3
			if pairs[i] == nil {
				pairs[i] = make(map[string]string)
			}
			pairs[i][v.Pair] = "5m"
		}
	}

	log.WithField("Count", len(exchangeInfo.Symbols)).Info("Symbols")

	for _, v := range pairs {
		go func(p map[string]string) {
			for {
				doneC, _, err := binanceFuture.WsCombinedKlineServe(p, wsKlineHandler, errWsKlineHandler)
				if err != nil {
					log.WithField("Error", err).Error("Read Kline")
					time.Sleep(5 * time.Second)
					continue
				}
				<-doneC
			}
		}(v)
		time.Sleep(time.Second)
	}
	c := make(chan struct{})
	<-c
}
