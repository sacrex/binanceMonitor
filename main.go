package main

import (
	"context"
	"fmt"
	binanceFuture "github.com/adshao/go-binance/v2/futures"
	"github.com/crazygit/binance-market-monitor/helper"
	l "github.com/crazygit/binance-market-monitor/helper/log"
	"github.com/crazygit/binance-market-monitor/misc"
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
	return fmt.Sprintf("%s%.2f%s", direction, math.Abs(PriceChangePercent), suffix)
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
*类型*: 5分钟的涨跌幅超过%s%%
*版块*: %s
*开盘价*: %s
*收盘价*: %s
*差\(收盘\-开盘\)*: %s
*开始时间*: %s
*结束时间*: %s
`

var strategy2Text = `
*交易对*: %s
*事件时间*: %s
*类型*: 连续3根5分钟线上涨\(%s\|%s\|%s\)或者15min内上涨超过🚀%s%%
`

func strategy2Handler(symbol string, up1, up2, up3 float64, up string) {
	var postMessageTextBuilder strings.Builder

	v1, v2, v3 := PercentStringify(up1, "%"), PercentStringify(up2, "%"), PercentStringify(up3, "%")
	if up1 < 0 || up2 < 0 || up3 < 0 {
		v1, v2, v3 = "-%", "-%", "-%"
	}

	postMessageTextBuilder.WriteString(fmt.Sprintf(strategy2Text,
		escapeTextToMarkdownV2(symbol),
		escapeTextToMarkdownV2(time.Now().Format(time.DateTime)),
		escapeTextToMarkdownV2(v1),
		escapeTextToMarkdownV2(v2),
		escapeTextToMarkdownV2(v3),
		escapeTextToMarkdownV2(up),
	))

	// send message
	Message(true, symbol, postMessageTextBuilder.String())
}

// BTCUSDT -> 176342342424 表示最后一次发送的交易对时间
var latestPairsTime map[string]int64

// symbolsToTags 获取符号的标签
var symbolsToTags map[string][]string

// pairsToInterval BTCUSDT:5m
var pairsToInterval []map[string]string

// pairs 符号
var pairs []string

var updatePairsChan chan int

func updatePairs(signal bool) bool {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("Error", r).Error("updatePairs panic")
		}
	}()

	exchangeInfo, err := binanceFuture.NewClient("", "").NewExchangeInfoService().Do(context.Background())
	if err != nil {
		log.WithField("Error", err).Error("Get ExchangeInfo failed")
		return false
	}

	symbolsToTags = make(map[string][]string)
	oldSymbolsToTags, err := misc.GetTags()
	if err != nil {
		log.WithField("error", err).Error("misc.GetTags")
		return false
	}
	for k, v := range oldSymbolsToTags {
		symbolsToTags[k+"USDT"] = v
	}

	pairsToInterval = make([]map[string]string, 3)
	pairs = make([]string, 0)
	for i, v := range exchangeInfo.Symbols {
		if v.QuoteAsset == "USDT" {
			i = i % 3
			if pairsToInterval[i] == nil {
				pairsToInterval[i] = make(map[string]string)
			}
			pairsToInterval[i][v.Pair] = "5m"
			pairs = append(pairs, v.Pair)
		}
	}

	if latestPairsTime == nil || len(latestPairsTime) < 1 {
		latestPairsTime = make(map[string]int64)
	}

	updatePairsChan = make(chan int, 1)
	if signal {
		updatePairsChan <- 1
	}
	return true
}

func Message(post bool, symbol, message string) {
	//判断该事件是不是需要频繁发送，若在60s内已经发送过了，就不发送
	if post {
		lastSendTime, ok := latestPairsTime[symbol]
		if !ok {
			latestPairsTime[symbol] = time.Now().Unix()
		} else {
			if time.Now().Unix()-lastSendTime >= 60 {
				latestPairsTime[symbol] = time.Now().Unix()
			} else {
				post = false
			}
		}
	}

	if post {
		if err := PostMessageToTgChannel(getTelegramChannelName(), message); err != nil {
			log.WithField("Error", err).Error("Post message to tg channel failed")
		}
	}
}

func wsKlineHandler(event *binanceFuture.WsKlineEvent) {
	var postMessageTextBuilder strings.Builder
	var postMessage = false
	//log.WithFields(logrus.Fields{"Symbol": event.Symbol, "Price": event.Kline.Close,
	//	"Time": time.UnixMilli(event.Time).Format(time.DateTime)}).Info("Stats")

	tags := "-"
	if len(symbolsToTags[event.Symbol]) > 0 {
		tags = strings.Join(symbolsToTags[event.Symbol], " ")
	}

	closePrice, _ := strconv.ParseFloat(event.Kline.Close, 64)
	openPrice, _ := strconv.ParseFloat(event.Kline.Open, 64)

	PriceChangePercent := (closePrice - openPrice) / openPrice
	if math.Abs(PriceChangePercent) >= 0.025 {
		postMessageTextBuilder.WriteString(fmt.Sprintf(wsKlineText,
			escapeTextToMarkdownV2(event.Symbol),
			escapeTextToMarkdownV2(PercentStringify(PriceChangePercent*100, "%")),
			escapeTextToMarkdownV2(time.UnixMilli(event.Time).Format(time.DateTime)),

			escapeTextToMarkdownV2("2.5"),

			escapeTextToMarkdownV2(tags),

			escapeTextToMarkdownV2(event.Kline.Open),
			escapeTextToMarkdownV2(event.Kline.Close),
			escapeTextToMarkdownV2(getDifference(closePrice, openPrice, "")),

			escapeTextToMarkdownV2(time.UnixMilli(event.Kline.StartTime).Format(time.DateTime)),
			escapeTextToMarkdownV2(time.UnixMilli(event.Kline.EndTime).Format(time.DateTime)),
		))
		postMessage = true
	}

	// send message
	Message(postMessage, event.Symbol, postMessageTextBuilder.String())
}

func errWsKlineHandler(err error) {
	log.Error(err)
}

func getDiffPercent(open, close float64) (res float64, b bool) {
	if close-open > 0 {
		b = true
	}

	res = (close - open) / open * 100
	return
}

func getKline(symbol, interval string, cnt int) (res []*binanceFuture.Kline, err error) {
	res, err = binanceFuture.NewClient("", "").NewKlinesService().
		Symbol(symbol).Interval(interval).Limit(cnt).Do(context.Background())
	if err != nil {
		log.WithField("Error", err).Error("get kline failed")
		return
	}
	return
}

// 二号策略：查看5m的级别，若连续三个都是上涨，或者15m内上涨超过3.5个点，报警
func strategy2(pairs []string, grpCnt int) {
	// 表示每组个数
	if grpCnt == 0 {
		grpCnt = 3
	}
	t := len(pairs) % grpCnt
	if t > 0 {
		t = 1
	}
	grp := len(pairs)/grpCnt + t
	for i := 0; i < grp; i++ {
		for j := 0; j < grpCnt; j++ {
			if i*grpCnt+j >= len(pairs) {
				break
			}

			symbol := pairs[i*grpCnt+j]
			res, err := getKline(pairs[i*grpCnt+j], "5m", 4)
			if err != nil {
				log.WithField("getKline", err).Error("strategy2 error")
				continue
			}

			//res[0] res[1] res[2], res[3]是正在生成的，不需要
			r1o, _ := strconv.ParseFloat(res[0].Open, 64)
			r1c, _ := strconv.ParseFloat(res[0].Close, 64)
			r2o, _ := strconv.ParseFloat(res[1].Open, 64)
			r2c, _ := strconv.ParseFloat(res[1].Close, 64)
			r3o, _ := strconv.ParseFloat(res[2].Open, 64)
			r3c, _ := strconv.ParseFloat(res[2].Close, 64)

			v1, up1 := getDiffPercent(r1o, r1c)
			v2, up2 := getDiffPercent(r2o, r2c)
			v3, up3 := getDiffPercent(r3o, r3c)

			res1 := v1 > 1.0 && v2 > 1.0 || v1 > 1.0 && v3 > 1.0 || v2 > 1.0 && v3 > 1.0
			res1 = res1 || (up1 && up2 && up3 && (v1 > 1.0 || v2 > 1.0 || v3 > 1.0))
			v, _ := getDiffPercent(r1o, r3c)
			res2 := v > 4 // 15min上涨4个点

			if res1 {
				strategy2Handler(symbol, v1, v2, v3, "-")
			}

			if res2 {
				strategy2Handler(symbol, v1, v2, v3, fmt.Sprintf("%.6f", v))
			}

		}
		time.Sleep(time.Second)
	}
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

	if !updatePairs(false) {
		log.Error("updatePairs error")
		return
	}

	// 定时更新符号等数据
	go func() {
		for {
			time.Sleep(4 * time.Hour)
			updatePairs(true)
		}
	}()

	// 策略2
	go func() {
		for {
			log.Infof("start strategy2....")
			strategy2(pairs, 3)
			time.Sleep(10 * time.Second)
		}
	}()

	log.WithField("Count", len(exchangeInfo.Symbols)).Info("Symbols")

	// 5min检测上涨逻辑
	go func() {
		for {
			doneCs := make([]chan struct{}, len(pairsToInterval))
			stopCs := make([]chan struct{}, len(pairsToInterval))
			for i, v := range pairsToInterval {
				go func(p map[string]string, j int) {
					log.Infof("ws worker thread: %d start.", j)
					doneCs[j], stopCs[j], err = binanceFuture.WsCombinedKlineServe(p, wsKlineHandler, errWsKlineHandler)
					if err != nil {
						log.WithField("Error", err).Errorf("Read Kline error. thread: %d", j)
						time.Sleep(5 * time.Second)
					}
					<-doneCs[j]
					log.Infof("ws worker thread: %d stop.", j)
				}(v, i)
				time.Sleep(time.Second)
			}
			time.Sleep(2 * time.Second)

			log.Infof("ws main thread: waiting signal....")
			<-updatePairsChan
			log.Infof("ws main thead: receive signal, let ws worker to exit")
			for _, v := range stopCs {
				close(v)
			}

			log.Infof("ws main thread: wait ws worker thread exit.")
			time.Sleep(3 * time.Second)
		}
	}()

	c := make(chan struct{})
	<-c
}
