package main

import (
	"context"
	"fmt"
	binanceFuture "github.com/adshao/go-binance/v2/futures"
	"github.com/crazygit/binance-market-monitor/helper"
	l "github.com/crazygit/binance-market-monitor/helper/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"math"
	"strconv"
	"strings"
	"time"
)

var log = l.GetLog()

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
	direction := "😭"
	if PriceChangePercent > 0 {
		direction = "🚀"
	}
	return fmt.Sprintf("%s%.6f%s", direction, math.Abs(PriceChangePercent), suffix)
}

func getTelegramChannelName() string {
	channelName := helper.GetRequiredStringEnv("TELEGRAM_CHANNEL_USERNAME")
	if !strings.HasPrefix(channelName, "@") {
		return "@" + channelName
	}
	return channelName
}

var wsKlineText = `
*交易对*: %s
*涨/跌幅*: %s
*事件时间*: %s
*类型*: 15分钟的涨跌幅超过3%%
*开盘价*: %s
*收盘价*: %s
*差\(收盘\-开盘\)*: %s
*开始时间*: %s
*结束时间*: %s
`

// BTCUSDT -> 176342342424 表示最后一次发送的交易对时间
var latestPairsTime map[string]int64

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
			escapeTextToMarkdownV2(event.Symbol),
			escapeTextToMarkdownV2(PercentStringify(PriceChangePercent*100, "%")),
			escapeTextToMarkdownV2(time.UnixMilli(event.Time).Format(time.DateTime)),

			escapeTextToMarkdownV2(event.Kline.Open),
			escapeTextToMarkdownV2(event.Kline.Close),
			escapeTextToMarkdownV2(getDifference(closePrice, openPrice, "")),

			escapeTextToMarkdownV2(time.UnixMilli(event.Kline.StartTime).Format(time.DateTime)),
			escapeTextToMarkdownV2(time.UnixMilli(event.Kline.EndTime).Format(time.DateTime)),
		))
		postMessage = true
	}

	//判断该事件是不是需要频繁发送，若在20s内已经发送过了，就不发送
	if postMessage {
		lastSendTime := latestPairsTime[event.Symbol]
		if lastSendTime == 0 || time.Now().Unix()-lastSendTime >= 20 {
			latestPairsTime[event.Symbol] = time.Now().Unix()
		} else {
			postMessage = false
		}
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
			pairs[i][v.Pair] = "15m"
		}
	}

	log.WithField("Count", len(exchangeInfo.Symbols)).Info("Symbols")

	latestPairsTime = make(map[string]int64)
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
