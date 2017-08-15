package main

import (
	"time"
	"github.com/mediocregopher/radix.v2/pool"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/mediocregopher/radix.v2/sentinel"
	"github.com/mediocregopher/radix.v2/util"
	"log"
	"fmt"
	"strings"
	"strconv"
)

var sentinelPool *sentinel.Client
var redisPool *pool.Pool

var dashboardMetaInfo []MetaData

func AppendIfMissing(windowList []string, i string) []string {
	for _, ele := range windowList {
		if ele == i {
			return windowList
		}
	}
	return append(windowList, i)
}

func AppendListIfMissing(windowList1 []string, windowList2 []string) []string {
	notExist := true
	for _, ele2 := range windowList2 {
		for _, ele := range windowList1 {
			if ele == ele2 {
				notExist = false
				break
			}
		}

		if notExist == true {
			windowList1 = append(windowList1, ele2)
		}
	}

	return windowList1
}

func InitiateRedis() {

	var err error

	df := func(network, addr string) (*redis.Client, error) {
		client, err := redis.Dial(network, addr)
		if err != nil {
			return nil, err
		}
		if err = client.Cmd("AUTH", redisPassword).Err; err != nil {
			client.Close()
			return nil, err
		}
		if err = client.Cmd("select", redisDb).Err; err != nil {
			client.Close()
			return nil, err
		}
		return client, nil
	}

	if redisMode == "sentinel" {
		sentinelIps := strings.Split(sentinelHosts, ",")

		if len(sentinelIps) > 1 {
			sentinelIp := fmt.Sprintf("%s:%s", sentinelIps[0], sentinelPort)
			sentinelPool, err = sentinel.NewClientCustom("tcp", sentinelIp, 10, df, redisClusterName)

			if err != nil {
				errHandler("InitiateRedis", "InitiateSentinel", err)
			}
		} else {
			fmt.Println("Not enough sentinel servers")
		}
	} else {
		redisPool, err = pool.NewCustom("tcp", redisIp, 10, df)

		if err != nil {
			errHandler("InitiateRedis", "InitiatePool", err)
		}
	}
}

func ScanAndGetKeys(pattern string) []string {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in ScanAndGetKeys", r)
		}
	}()

	matchingKeys := make([]string, 0)

	var client *redis.Client
	var err error

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHandler("ScanAndGetKeys", "getConnFromSentinel", err)
		defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHandler("ScanAndGetKeys", "getConnFromPool", err)
		defer redisPool.Put(client)
	}

	log.Println("Start ScanAndGetKeys:: ", pattern)
	scanResult := util.NewScanner(client, util.ScanOpts{Command: "SCAN", Pattern: pattern, Count: 1000})

	for scanResult.HasNext() {
		matchingKeys = AppendIfMissing(matchingKeys, scanResult.Next())
	}

	log.Println("Scan Result:: ", matchingKeys)
	return matchingKeys
}

func OnSetDailySummary(_date time.Time) {
	totCountEventSearch := fmt.Sprintf("TOTALCOUNT:*")
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in OnSetDailySummary", r)
		}
	}()
	var client *redis.Client
	var err error

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHandler("OnSetDailySummary", "getConnFromSentinel", err)
		defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHandler("OnSetDailySummary", "getConnFromPool", err)
		defer redisPool.Put(client)
	}

	todaySummary := make([]SummeryDetail, 0)
	totalEventKeys := ScanAndGetKeys(totCountEventSearch)
	for _, key := range totalEventKeys {
		fmt.Println("Key: ", key)
		keyItems := strings.Split(key, ":")
		summery := SummeryDetail{}
		tenant, _ := strconv.Atoi(keyItems[1])
		company, _ := strconv.Atoi(keyItems[2])
		summery.Tenant = tenant
		summery.Company = company
		summery.WindowName = keyItems[3]
		summery.Param1 = keyItems[4]
		summery.Param2 = keyItems[5]

		currentTime := 0
		if summery.WindowName == "LOGIN" {
			sessEventSearch := fmt.Sprintf("SESSION:%d:%d:%s:*:%s:%s", tenant, company, summery.WindowName, summery.Param1, summery.Param2)
			sessEvents := ScanAndGetKeys(sessEventSearch)
			if len(sessEvents) > 0 {
				tmx, tmxErr := client.Cmd("hget", sessEvents[0], "time").Str()
				errHandler("OnSetDailySummary", "Cmd", tmxErr)
				tm2, _ := time.Parse(layout, tmx)
				currentTime = int(_date.Sub(tm2.Local()).Seconds())
				fmt.Println("currentTime: ", currentTime)
			}
		}
		totTimeEventName := fmt.Sprintf("TOTALTIME:%d:%d:%s:%s:%s", tenant, company, summery.WindowName, summery.Param1, summery.Param2)
		maxTimeEventName := fmt.Sprintf("MAXTIME:%d:%d:%s:%s:%s", tenant, company, summery.WindowName, summery.Param1, summery.Param2)
		thresholdEventName := fmt.Sprintf("THRESHOLD:%d:%d:%s:%s:%s", tenant, company, summery.WindowName, summery.Param1, summery.Param2)

		log.Println("totTimeEventName: ", totTimeEventName)
		log.Println("maxTimeEventName: ", maxTimeEventName)
		log.Println("thresholdEventName: ", thresholdEventName)

		totCount, totCountErr := client.Cmd("get", key).Int()
		totTime, totTimeErr := client.Cmd("get", totTimeEventName).Int()
		maxTime, maxTimeErr := client.Cmd("get", maxTimeEventName).Int()
		threshold, thresholdErr := client.Cmd("get", thresholdEventName).Int()

		errHandler("OnSetDailySummary", "Cmd", totCountErr)
		errHandler("OnSetDailySummary", "Cmd", totTimeErr)
		errHandler("OnSetDailySummary", "Cmd", maxTimeErr)
		errHandler("OnSetDailySummary", "Cmd", thresholdErr)

		log.Println("totCount: ", totCount)
		log.Println("totTime: ", totTime)
		log.Println("maxTime: ", maxTime)
		log.Println("threshold: ", threshold)

		summery.TotalCount = totCount
		summery.TotalTime = totTime + currentTime
		summery.MaxTime = maxTime
		summery.ThresholdValue = threshold
		summery.SummaryDate = _date

		todaySummary = append(todaySummary, summery)
	}

	if len(todaySummary) > 0 {
		go PersistDailySummaries(todaySummary)
	}
}

func OnSetDailyThresholdBreakDown(_date time.Time) {
	thresholdEventSearch := fmt.Sprintf("THRESHOLDBREAKDOWN:*")
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in OnSetDailyThesholdBreakDown", r)
		}
	}()
	var client *redis.Client
	var err error

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHandler("OnSetDailyThesholdBreakDown", "getConnFromSentinel", err)
		defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHandler("OnSetDailyThesholdBreakDown", "getConnFromPool", err)
		defer redisPool.Put(client)
	}

	thresholdRecords := make([]ThresholdBreakDownDetail, 0)
	thresholdEventKeys := ScanAndGetKeys(thresholdEventSearch)

	for _, key := range thresholdEventKeys {
		fmt.Println("Key: ", key)
		keyItems := strings.Split(key, ":")

		if len(keyItems) >= 9 {
			summery := ThresholdBreakDownDetail{}
			tenant, _ := strconv.Atoi(keyItems[1])
			company, _ := strconv.Atoi(keyItems[2])
			hour, _ := strconv.Atoi(keyItems[6])
			summery.Tenant = tenant
			summery.Company = company
			summery.WindowName = keyItems[3]
			summery.Param1 = keyItems[4]
			summery.Param2 = keyItems[5]
			summery.BreakDown = fmt.Sprintf("%s-%s", keyItems[7], keyItems[8])
			summery.Hour = hour

			thCount, thCountErr := client.Cmd("get", key).Int()
			errHandler("OnSetDailyThesholdBreakDown", "Cmd", thCountErr)
			summery.ThresholdCount = thCount
			summery.SummaryDate = _date

			thresholdRecords = append(thresholdRecords, summery)
		}
	}

	if len(thresholdRecords) > 0 {
		go PersistThresholdBreakDown(thresholdRecords)
	}
}

func OnReset() {

	_searchName := fmt.Sprintf("META:*:FLUSH")
	fmt.Println("Search Windows to Flush: ", _searchName)

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in OnReset", r)
		}
	}()
	var client *redis.Client
	var err error

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHandler("OnReset", "getConnFromSentinel", err)
		defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHandler("OnReset", "getConnFromPool", err)
		defer redisPool.Put(client)
	}

	_windowList := make([]string, 0)
	_keysToRemove := make([]string, 0)
	_loginSessions := make([]string, 0)
	_productivitySessions := make([]string, 0)

	fmt.Println("---------------------Use Memoey----------------------")
	for _, dmi := range dashboardMetaInfo {
		if dmi.FlushEnable == true {
			_windowList = AppendIfMissing(_windowList, dmi.WindowName)
		}
	}

	fmt.Println("Windoes To Flush:: ", _windowList)

	for _, window := range _windowList {

		fmt.Println("WindowList_: ", window)

		concEventSearch := fmt.Sprintf("CONCURRENT:*:%s:*", window)
		sessEventSearch := fmt.Sprintf("SESSION:*:%s:*", window)
		sessParamsEventSearch := fmt.Sprintf("SESSIONPARAMS:*:%s:*", window)
		totTimeEventSearch := fmt.Sprintf("TOTALTIME:*:%s:*", window)
		totCountEventSearch := fmt.Sprintf("TOTALCOUNT:*:%s:*", window)
		totCountHr := fmt.Sprintf("TOTALCOUNTHR:*:%s:*", window)
		maxTimeEventSearch := fmt.Sprintf("MAXTIME:*:%s:*", window)
		thresholdEventSearch := fmt.Sprintf("THRESHOLD:*:%s:*", window)
		thresholdBDEventSearch := fmt.Sprintf("THRESHOLDBREAKDOWN:*:%s:*", window)

		concEventNameWithoutParams := fmt.Sprintf("CONCURRENTWOPARAMS:*:%s", window)
		totTimeEventNameWithoutParams := fmt.Sprintf("TOTALTIMEWOPARAMS:*:%s", window)
		totCountEventNameWithoutParams := fmt.Sprintf("TOTALCOUNTWOPARAMS:*:%s", window)

		concEventNameWithSingleParam := fmt.Sprintf("CONCURRENTWSPARAM:*:%s:*", window)
		totTimeEventNameWithSingleParam := fmt.Sprintf("TOTALTIMEWSPARAM:*:%s:*", window)
		totCountEventNameWithSingleParam := fmt.Sprintf("TOTALCOUNTWSPARAM:*:%s:*", window)

		concEventNameWithLastParam := fmt.Sprintf("CONCURRENTWLPARAM:*:%s:*", window)
		totTimeEventNameWithLastParam := fmt.Sprintf("TOTALTIMEWLPARAM:*:%s:*", window)
		totCountEventNameWithLastParam := fmt.Sprintf("TOTALCOUNTWLPARAM:*:%s:*", window)

		concVal := ScanAndGetKeys(concEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, concVal)

		sessParamsVal := ScanAndGetKeys(sessParamsEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, sessParamsVal)

		sessVal := ScanAndGetKeys(sessEventSearch)
		for _, sess := range sessVal {
			sessItems := strings.Split(sess, ":")
			if len(sessItems) >= 4 && sessItems[3] == "LOGIN" {
				_loginSessions = AppendIfMissing(_loginSessions, sess)
			} else if len(sessItems) >= 4 && sessItems[3] == "PRODUCTIVITY" {
				_productivitySessions = AppendIfMissing(_productivitySessions, sess)
			} else {
				_keysToRemove = AppendIfMissing(_keysToRemove, sess)
			}
		}

		totTimeVal := ScanAndGetKeys(totTimeEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, totTimeVal)

		totCountVal := ScanAndGetKeys(totCountEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, totCountVal)

		totCountHrVal := ScanAndGetKeys(totCountHr)
		_keysToRemove = AppendListIfMissing(_keysToRemove, totCountHrVal)

		maxTimeVal := ScanAndGetKeys(maxTimeEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, maxTimeVal)

		thresholdCountVal := ScanAndGetKeys(thresholdEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, thresholdCountVal)

		thresholdBDCountVal := ScanAndGetKeys(thresholdBDEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, thresholdBDCountVal)

		cewop := ScanAndGetKeys(concEventNameWithoutParams)
		_keysToRemove = AppendListIfMissing(_keysToRemove, cewop)

		ttwop := ScanAndGetKeys(totTimeEventNameWithoutParams)
		_keysToRemove = AppendListIfMissing(_keysToRemove, ttwop)

		tcewop := ScanAndGetKeys(totCountEventNameWithoutParams)
		_keysToRemove = AppendListIfMissing(_keysToRemove, tcewop)

		cewsp := ScanAndGetKeys(concEventNameWithSingleParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, cewsp)

		ttwsp := ScanAndGetKeys(totTimeEventNameWithSingleParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, ttwsp)

		tcwsp := ScanAndGetKeys(totCountEventNameWithSingleParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, tcwsp)

		cewlp := ScanAndGetKeys(concEventNameWithLastParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, cewlp)

		ttwlp := ScanAndGetKeys(totTimeEventNameWithLastParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, ttwlp)

		tcwlp := ScanAndGetKeys(totCountEventNameWithLastParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, tcwlp)

	}
	tm := time.Now()
	for _, remove := range _keysToRemove {
		fmt.Println("remove_: ", remove)
		errHandler("OnReset", "Cmd", client.Cmd("del", remove).Err)
	}
	for _, session := range _loginSessions {
		fmt.Println("readdSession: ", session)
		errHandler("OnReset", "Cmd", client.Cmd("hset", session, "time", tm.Format(layout)).Err)
		sessItemsL := strings.Split(session, ":")
		if len(sessItemsL) >= 7 {
			LsessParamEventName := fmt.Sprintf("SESSIONPARAMS:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4])
			LtotTimeEventName := fmt.Sprintf("TOTALTIME:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[5], sessItemsL[6])
			LtotCountEventName := fmt.Sprintf("TOTALCOUNT:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[5], sessItemsL[6])
			LtotTimeEventNameWithoutParams := fmt.Sprintf("TOTALTIMEWOPARAMS:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3])
			LtotCountEventNameWithoutParams := fmt.Sprintf("TOTALCOUNTWOPARAMS:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3])
			LtotTimeEventNameWithSingleParam := fmt.Sprintf("TOTALTIMEWSPARAM:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[5])
			LtotCountEventNameWithSingleParam := fmt.Sprintf("TOTALCOUNTWSPARAM:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[5])
			LtotTimeEventNameWithLastParam := fmt.Sprintf("TOTALTIMEWLPARAM:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[6])
			LtotCountEventNameWithLastParam := fmt.Sprintf("TOTALCOUNTWLPARAM:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[6])

			errHandler("OnReset", "Cmd", client.Cmd("hmset", LsessParamEventName, "param1", sessItemsL[5], "param2", sessItemsL[6]).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotTimeEventName, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotCountEventName, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithoutParams, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithoutParams, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithSingleParam, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithSingleParam, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithLastParam, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithLastParam, 0).Err)
		}
	}
}