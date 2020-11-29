package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mediocregopher/radix.v2/pool"
	"github.com/mediocregopher/radix.v2/redis"
	"github.com/mediocregopher/radix.v2/sentinel"
	"github.com/mediocregopher/radix.v2/util"
)

type DashboardRequest struct {
	Company int `json/"company"`
	Tenant  int `json/"tenant"`
}

var sentinelPool *sentinel.Client
var redisPool *pool.Pool

var dashboardMetaInfo []MetaData
var companyInfoData []CompanyInfo

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
		if redisPassword != "" {
			if err = client.Cmd("AUTH", redisPassword).Err; err != nil {
				client.Close()
				return nil, err
			}
		}
		// if err = client.Cmd("AUTH", redisPassword).Err; err != nil {
		// 	client.Close()
		// 	return nil, err
		// }
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
	var client *redis.Client
	var err error

	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in ScanAndGetKeys", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	matchingKeys := make([]string, 0)

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHandler("ScanAndGetKeys", "getConnFromSentinel", err)
		//defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHandler("ScanAndGetKeys", "getConnFromPool", err)
		//defer redisPool.Put(client)
	}

	log.Println("Start ScanAndGetKeys:: ", pattern)
	scanResult := util.NewScanner(client, util.ScanOpts{Command: "SCAN", Pattern: pattern, Count: 1000})

	for scanResult.HasNext() {
		matchingKeys = AppendIfMissing(matchingKeys, scanResult.Next())
	}

	log.Println("Scan Result:: ", matchingKeys)
	return matchingKeys
}

func GetTask() (task DashboardRequest, er error) {
	var client *redis.Client
	var err error

	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in GetTask", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHandler("GetTask", "getConnFromSentinel", err)
		//defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHandler("GetTask", "getConnFromPool", err)
		//defer redisPool.Put(client)
	}

	log.Println("Start GetTask:: ")

	data := DashboardRequest{}
	//taskResp, cmdErr := client.Cmd("BLPOP", "DashboardService-Backup", 5).Str()
	// if cmdErr != nil {
	// 	log.Println(cmdErr.Error())
	// }
	// if taskResp != "" {
	// 	json.Unmarshal([]byte(taskResp), &data)
	// }
	taskResp, cmdErr := client.Cmd("BLPOP", "DashboardService-Backup", 5).List()
	if cmdErr != nil {
		log.Println(cmdErr.Error())
	}

	if len(taskResp) > 1 && taskResp[1] != "" {
		json.Unmarshal([]byte(taskResp[1]), &data)
	}

	return data, cmdErr
}

func Contains(a []CompanyInfo, c int, t int) bool {
	for _, n := range a {
		if c == n.Company && t == n.Tenant {
			return true
		}
	}
	return false
}

func OnSetDailySummary(company int, tenant int) {
	var client *redis.Client
	var err error

	totCountEventSearch := fmt.Sprintf("TOTALCOUNT:%d:%d:*", tenant, company)
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in OnSetDailySummary", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHandler("OnSetDailySummary", "getConnFromSentinel", err)
		//defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHandler("OnSetDailySummary", "getConnFromPool", err)
		//defer redisPool.Put(client)
	}

	todaySummary := make([]SummeryDetail, 0)
	totalEventKeys := ScanAndGetKeys(totCountEventSearch)

	for _, key := range totalEventKeys {
		fmt.Println("Key: ", key)
		keyItems := strings.Split(key, ":")

		if len(keyItems) >= 7 {
			summery := SummeryDetail{}
			cmpData := CompanyInfo{}

			// tenant, _ := strconv.Atoi(keyItems[1])
			// company, _ := strconv.Atoi(keyItems[2])
			cmpData.Tenant = tenant
			cmpData.Company = company

			if !Contains(companyInfoData, company, tenant) {
				companyInfoData = append(companyInfoData, cmpData)
			}

			summery.Tenant = tenant
			summery.Company = company
			summery.BusinessUnit = keyItems[3]
			summery.WindowName = keyItems[4]
			summery.Param1 = keyItems[5]
			summery.Param2 = keyItems[6]

			currentTime := 0
			if summery.WindowName == "LOGIN" {
				sessEventSearch := fmt.Sprintf("SESSION:%d:%d:%s:%s:*:%s:%s", tenant, company, summery.BusinessUnit, summery.WindowName, summery.Param1, summery.Param2)
				sessEvents := ScanAndGetKeys(sessEventSearch)
				if len(sessEvents) > 0 {
					tmx, tmxErr := client.Cmd("hget", sessEvents[0], "time").Str()
					errHandler("OnSetDailySummary", "Cmd", tmxErr)
					tm2, _ := time.Parse(layout, tmx)
					currentTime = int(time.Now().Local().Sub(tm2.Local()).Seconds())
					fmt.Println("currentTime: ", currentTime)
				}
			}
			totTimeEventName := fmt.Sprintf("TOTALTIME:%d:%d:%s:%s:%s:%s", tenant, company, summery.BusinessUnit, summery.WindowName, summery.Param1, summery.Param2)
			maxTimeEventName := fmt.Sprintf("MAXTIME:%d:%d:%s:%s:%s:%s", tenant, company, summery.BusinessUnit, summery.WindowName, summery.Param1, summery.Param2)
			thresholdEventName := fmt.Sprintf("THRESHOLD:%d:%d:%s:%s:%s:%s", tenant, company, summery.BusinessUnit, summery.WindowName, summery.Param1, summery.Param2)

			log.Println("totTimeEventName: ", totTimeEventName)
			log.Println("maxTimeEventName: ", maxTimeEventName)
			log.Println("thresholdEventName: ", thresholdEventName)

			client.PipeAppend("get", key)
			client.PipeAppend("get", totTimeEventName)
			client.PipeAppend("get", maxTimeEventName)
			client.PipeAppend("get", thresholdEventName)

			totCount, _ := client.PipeResp().Int()
			totTime, _ := client.PipeResp().Int()
			maxTime, _ := client.PipeResp().Int()
			threshold, _ := client.PipeResp().Int()

			log.Println("totCount: ", totCount)
			log.Println("totTime: ", totTime)
			log.Println("maxTime: ", maxTime)
			log.Println("threshold: ", threshold)

			summery.TotalCount = totCount
			summery.TotalTime = totTime + currentTime
			summery.MaxTime = maxTime
			summery.ThresholdValue = threshold
			summery.SummaryDate = time.Now().Local()

			todaySummary = append(todaySummary, summery)
		}
	}

	if len(todaySummary) > 0 {
		go PersistDailySummaries(todaySummary)
	}

	fmt.Println("Company Data ++++++++ ", companyInfoData)
}

func OnSetDailyThresholdBreakDown(company int, tenant int) {
	var client *redis.Client
	var err error

	thresholdEventSearch := fmt.Sprintf("THRESHOLDBREAKDOWN:%d:%d:*", tenant, company)
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in OnSetDailyThesholdBreakDown", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHandler("OnSetDailyThesholdBreakDown", "getConnFromSentinel", err)
		//defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHandler("OnSetDailyThesholdBreakDown", "getConnFromPool", err)
		//defer redisPool.Put(client)
	}

	thresholdRecords := make([]ThresholdBreakDownDetail, 0)
	thresholdEventKeys := ScanAndGetKeys(thresholdEventSearch)

	for _, key := range thresholdEventKeys {
		fmt.Println("Key: ", key)
		keyItems := strings.Split(key, ":")

		if len(keyItems) >= 10 {
			summery := ThresholdBreakDownDetail{}
			//tenant, _ := strconv.Atoi(keyItems[1])
			//company, _ := strconv.Atoi(keyItems[2])
			hour, _ := strconv.Atoi(keyItems[7])
			summery.Tenant = tenant
			summery.Company = company
			summery.BusinessUnit = keyItems[3]
			summery.WindowName = keyItems[4]
			summery.Param1 = keyItems[5]
			summery.Param2 = keyItems[6]
			summery.BreakDown = fmt.Sprintf("%s-%s", keyItems[8], keyItems[9])
			summery.Hour = hour

			thCount, thCountErr := client.Cmd("get", key).Int()
			errHandler("OnSetDailyThesholdBreakDown", "Cmd", thCountErr)
			summery.ThresholdCount = thCount
			summery.SummaryDate = time.Now().Local()

			thresholdRecords = append(thresholdRecords, summery)
		}
	}

	if len(thresholdRecords) > 0 {
		go PersistThresholdBreakDown(thresholdRecords)
	}
}

func OnReset(company int, tenant int) {

	var client *redis.Client
	var err error

	_searchName := fmt.Sprintf("META:*:FLUSH")
	fmt.Println("Search Windows to Flush: ", _searchName)

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in OnReset", r)
		}

		if client != nil {
			if redisMode == "sentinel" {
				sentinelPool.PutMaster(redisClusterName, client)
			} else {
				redisPool.Put(client)
			}
		} else {
			fmt.Println("Cannot Put invalid connection")
		}
	}()

	if redisMode == "sentinel" {
		client, err = sentinelPool.GetMaster(redisClusterName)
		errHandler("OnReset", "getConnFromSentinel", err)
		//defer sentinelPool.PutMaster(redisClusterName, client)
	} else {
		client, err = redisPool.Get()
		errHandler("OnReset", "getConnFromPool", err)
		//defer redisPool.Put(client)
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

		concEventSearch := fmt.Sprintf("CONCURRENT:%d:%d:%s:*", tenant, company, window)
		concEventSearch_bu := fmt.Sprintf("CONCURRENT:%d:%d:*:%s:*", tenant, company, window)
		sessEventSearch := fmt.Sprintf("SESSION:%d:%d:%s:*", tenant, company, window)
		sessEventSearch_bu := fmt.Sprintf("SESSION:%d:%d:*:%s:*", tenant, company, window)
		sessParamsEventSearch := fmt.Sprintf("SESSIONPARAMS:%d:%d:%s:*", tenant, company, window)
		sessParamsEventSearch_bu := fmt.Sprintf("SESSIONPARAMS:%d:%d:*:%s:*", tenant, company, window)
		totTimeEventSearch := fmt.Sprintf("TOTALTIME:%d:%d:%s:*", tenant, company, window)
		totTimeEventSearch_bu := fmt.Sprintf("TOTALTIME:%d:%d:*:%s:*", tenant, company, window)
		totCountEventSearch := fmt.Sprintf("TOTALCOUNT:%d:%d:%s:*", tenant, company, window)
		totCountEventSearch_bu := fmt.Sprintf("TOTALCOUNT:%d:%d:*:%s:*", tenant, company, window)
		totCountHr := fmt.Sprintf("TOTALCOUNTHR:%d:%d:%s:*", tenant, company, window)
		totCountHr_bu := fmt.Sprintf("TOTALCOUNTHR:%d:%d:*:%s:*", tenant, company, window)
		maxTimeEventSearch := fmt.Sprintf("MAXTIME:%d:%d:%s:*", tenant, company, window)
		maxTimeEventSearch_bu := fmt.Sprintf("MAXTIME:%d:%d:*:%s:*", tenant, company, window)
		thresholdEventSearch := fmt.Sprintf("THRESHOLD:%d:%d:%s:*", tenant, company, window)
		thresholdEventSearch_bu := fmt.Sprintf("THRESHOLD:%d:%d:*:%s:*", tenant, company, window)
		thresholdBDEventSearch := fmt.Sprintf("THRESHOLDBREAKDOWN:%d:%d:%s:*", tenant, company, window)
		thresholdBDEventSearch_bu := fmt.Sprintf("THRESHOLDBREAKDOWN:%d:%d:*:%s:*", tenant, company, window)

		concEventNameWithoutParams := fmt.Sprintf("CONCURRENTWOPARAMS:%d:%d:%s", tenant, company, window)
		concEventNameWithoutParams_bu := fmt.Sprintf("CONCURRENTWOPARAMS:%d:%d:*:%s", tenant, company, window)
		totTimeEventNameWithoutParams := fmt.Sprintf("TOTALTIMEWOPARAMS:%d:%d:%s", tenant, company, window)
		totTimeEventNameWithoutParams_bu := fmt.Sprintf("TOTALTIMEWOPARAMS:%d:%d:*:%s", tenant, company, window)
		totCountEventNameWithoutParams := fmt.Sprintf("TOTALCOUNTWOPARAMS:%d:%d:%s", tenant, company, window)
		totCountEventNameWithoutParams_bu := fmt.Sprintf("TOTALCOUNTWOPARAMS:%d:%d:*:%s", tenant, company, window)

		concEventNameWithSingleParam := fmt.Sprintf("CONCURRENTWSPARAM:%d:%d:%s:*", tenant, company, window)
		concEventNameWithSingleParam_bu := fmt.Sprintf("CONCURRENTWSPARAM:%d:%d:*:%s:*", tenant, company, window)
		totTimeEventNameWithSingleParam := fmt.Sprintf("TOTALTIMEWSPARAM:%d:%d:%s:*", tenant, company, window)
		totTimeEventNameWithSingleParam_bu := fmt.Sprintf("TOTALTIMEWSPARAM:%d:%d:*:%s:*", tenant, company, window)
		totCountEventNameWithSingleParam := fmt.Sprintf("TOTALCOUNTWSPARAM:%d:%d:%s:*", tenant, company, window)
		totCountEventNameWithSingleParam_bu := fmt.Sprintf("TOTALCOUNTWSPARAM:%d:%d:*:%s:*", tenant, company, window)

		concEventNameWithLastParam := fmt.Sprintf("CONCURRENTWLPARAM:%d:%d:%s:*", tenant, company, window)
		concEventNameWithLastParam_bu := fmt.Sprintf("CONCURRENTWLPARAM:%d:%d:*:%s:*", tenant, company, window)
		totTimeEventNameWithLastParam := fmt.Sprintf("TOTALTIMEWLPARAM:%d:%d:%s:*", tenant, company, window)
		totTimeEventNameWithLastParam_bu := fmt.Sprintf("TOTALTIMEWLPARAM:%d:%d:*:%s:*", tenant, company, window)
		totCountEventNameWithLastParam := fmt.Sprintf("TOTALCOUNTWLPARAM:%d:%d:%s:*", tenant, company, window)
		totCountEventNameWithLastParam_bu := fmt.Sprintf("TOTALCOUNTWLPARAM:%d:%d:*:%s:*", tenant, company, window)

		concVal := ScanAndGetKeys(concEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, concVal)

		concVal_bu := ScanAndGetKeys(concEventSearch_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, concVal_bu)

		sessParamsVal := ScanAndGetKeys(sessParamsEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, sessParamsVal)

		sessParamsVal_bu := ScanAndGetKeys(sessParamsEventSearch_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, sessParamsVal_bu)

		sessVal := ScanAndGetKeys(sessEventSearch)
		for _, sess := range sessVal {
			sessItems := strings.Split(sess, ":")
			if len(sessItems) >= 5 && (sessItems[4] == "LOGIN" || sessItems[4] == "INBOUND" || sessItems[4] == "OUTBOUND") {
				_loginSessions = AppendIfMissing(_loginSessions, sess)
			} else if len(sessItems) >= 4 && sessItems[4] == "PRODUCTIVITY" {
				_productivitySessions = AppendIfMissing(_productivitySessions, sess)
			} else {
				_keysToRemove = AppendIfMissing(_keysToRemove, sess)
			}
		}

		sessVal_bu := ScanAndGetKeys(sessEventSearch_bu)
		for _, sess := range sessVal_bu {
			sessItems := strings.Split(sess, ":")
			if len(sessItems) >= 5 && (sessItems[4] == "LOGIN" || sessItems[4] == "INBOUND" || sessItems[4] == "OUTBOUND") {
				_loginSessions = AppendIfMissing(_loginSessions, sess)
			} else if len(sessItems) >= 4 && sessItems[4] == "PRODUCTIVITY" {
				_productivitySessions = AppendIfMissing(_productivitySessions, sess)
			} else {
				_keysToRemove = AppendIfMissing(_keysToRemove, sess)
			}
		}

		totTimeVal := ScanAndGetKeys(totTimeEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, totTimeVal)

		totTimeVal_bu := ScanAndGetKeys(totTimeEventSearch_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, totTimeVal_bu)

		totCountVal := ScanAndGetKeys(totCountEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, totCountVal)

		totCountVal_bu := ScanAndGetKeys(totCountEventSearch_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, totCountVal_bu)

		totCountHrVal := ScanAndGetKeys(totCountHr)
		_keysToRemove = AppendListIfMissing(_keysToRemove, totCountHrVal)

		totCountHrVal_bu := ScanAndGetKeys(totCountHr_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, totCountHrVal_bu)

		maxTimeVal := ScanAndGetKeys(maxTimeEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, maxTimeVal)

		maxTimeVal_bu := ScanAndGetKeys(maxTimeEventSearch_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, maxTimeVal_bu)

		thresholdCountVal := ScanAndGetKeys(thresholdEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, thresholdCountVal)

		thresholdCountVal_bu := ScanAndGetKeys(thresholdEventSearch_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, thresholdCountVal_bu)

		thresholdBDCountVal := ScanAndGetKeys(thresholdBDEventSearch)
		_keysToRemove = AppendListIfMissing(_keysToRemove, thresholdBDCountVal)

		thresholdBDCountVal_bu := ScanAndGetKeys(thresholdBDEventSearch_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, thresholdBDCountVal_bu)

		cewop := ScanAndGetKeys(concEventNameWithoutParams)
		_keysToRemove = AppendListIfMissing(_keysToRemove, cewop)

		cewop_bu := ScanAndGetKeys(concEventNameWithoutParams_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, cewop_bu)

		ttwop := ScanAndGetKeys(totTimeEventNameWithoutParams)
		_keysToRemove = AppendListIfMissing(_keysToRemove, ttwop)

		ttwop_bu := ScanAndGetKeys(totTimeEventNameWithoutParams_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, ttwop_bu)

		tcewop := ScanAndGetKeys(totCountEventNameWithoutParams)
		_keysToRemove = AppendListIfMissing(_keysToRemove, tcewop)

		tcewop_bu := ScanAndGetKeys(totCountEventNameWithoutParams_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, tcewop_bu)

		cewsp := ScanAndGetKeys(concEventNameWithSingleParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, cewsp)

		cewsp_bu := ScanAndGetKeys(concEventNameWithSingleParam_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, cewsp_bu)

		ttwsp := ScanAndGetKeys(totTimeEventNameWithSingleParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, ttwsp)

		ttwsp_bu := ScanAndGetKeys(totTimeEventNameWithSingleParam_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, ttwsp_bu)

		tcwsp := ScanAndGetKeys(totCountEventNameWithSingleParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, tcwsp)

		tcwsp_bu := ScanAndGetKeys(totCountEventNameWithSingleParam_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, tcwsp_bu)

		cewlp := ScanAndGetKeys(concEventNameWithLastParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, cewlp)

		cewlp_bu := ScanAndGetKeys(concEventNameWithLastParam_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, cewlp_bu)

		ttwlp := ScanAndGetKeys(totTimeEventNameWithLastParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, ttwlp)

		ttwlp_bu := ScanAndGetKeys(totTimeEventNameWithLastParam_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, ttwlp_bu)

		tcwlp := ScanAndGetKeys(totCountEventNameWithLastParam)
		_keysToRemove = AppendListIfMissing(_keysToRemove, tcwlp)

		tcwlp_bu := ScanAndGetKeys(totCountEventNameWithLastParam_bu)
		_keysToRemove = AppendListIfMissing(_keysToRemove, tcwlp_bu)

	}
	tm := time.Now()
	for _, remove := range _keysToRemove {
		rKey := fmt.Sprintf("remove_: %s", remove)
		errHandler("OnReset", rKey, client.Cmd("del", remove).Err)
	}
	for _, session := range _loginSessions {
		fmt.Println("readdSession: ", session)
		errHandler("OnReset", "Cmd", client.Cmd("hset", session, "time", tm.Format(layout)).Err)
		sessItemsL := strings.Split(session, ":")

		if len(sessItemsL) >= 7 {
			LsessParamEventName := fmt.Sprintf("SESSIONPARAMS:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[5])
			LtotTimeEventName := fmt.Sprintf("TOTALTIME:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[6], sessItemsL[7])
			LtotCountEventName := fmt.Sprintf("TOTALCOUNT:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[6], sessItemsL[7])
			LtotTimeEventNameWithoutParams := fmt.Sprintf("TOTALTIMEWOPARAMS:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4])
			LtotCountEventNameWithoutParams := fmt.Sprintf("TOTALCOUNTWOPARAMS:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4])
			LtotTimeEventNameWithSingleParam := fmt.Sprintf("TOTALTIMEWSPARAM:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[6])
			LtotCountEventNameWithSingleParam := fmt.Sprintf("TOTALCOUNTWSPARAM:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[6])
			LtotTimeEventNameWithLastParam := fmt.Sprintf("TOTALTIMEWLPARAM:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[7])
			LtotCountEventNameWithLastParam := fmt.Sprintf("TOTALCOUNTWLPARAM:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[4], sessItemsL[7])

			LtotTimeEventName_BusinessUnit := fmt.Sprintf("TOTALTIME:%s:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4], sessItemsL[6], sessItemsL[7])
			LtotCountEventName_BusinessUnit := fmt.Sprintf("TOTALCOUNT:%s:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4], sessItemsL[6], sessItemsL[7])
			LtotTimeEventNameWithoutParams_BusinessUnit := fmt.Sprintf("TOTALTIMEWOPARAMS:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4])
			LtotCountEventNameWithoutParams_BusinessUnit := fmt.Sprintf("TOTALCOUNTWOPARAMS:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4])
			LtotTimeEventNameWithSingleParam_BusinessUnit := fmt.Sprintf("TOTALTIMEWSPARAM:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4], sessItemsL[6])
			LtotCountEventNameWithSingleParam_BusinessUnit := fmt.Sprintf("TOTALCOUNTWSPARAM:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4], sessItemsL[6])
			LtotTimeEventNameWithLastParam_BusinessUnit := fmt.Sprintf("TOTALTIMEWLPARAM:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4], sessItemsL[7])
			LtotCountEventNameWithLastParam_BusinessUnit := fmt.Sprintf("TOTALCOUNTWLPARAM:%s:%s:%s:%s:%s", sessItemsL[1], sessItemsL[2], sessItemsL[3], sessItemsL[4], sessItemsL[7])

			errHandler("OnReset", "Cmd", client.Cmd("hmset", LsessParamEventName, "businessUnit", sessItemsL[3], "param1", sessItemsL[6], "param2", sessItemsL[7]).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotTimeEventName, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotCountEventName, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithoutParams, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithoutParams, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithSingleParam, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithSingleParam, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithLastParam, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithLastParam, 0).Err)

			errHandler("OnReset", "Cmd", client.Cmd("set", LtotTimeEventName_BusinessUnit, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotCountEventName_BusinessUnit, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithoutParams_BusinessUnit, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithoutParams_BusinessUnit, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithSingleParam_BusinessUnit, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithSingleParam_BusinessUnit, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotTimeEventNameWithLastParam_BusinessUnit, 0).Err)
			errHandler("OnReset", "Cmd", client.Cmd("set", LtotCountEventNameWithLastParam_BusinessUnit, 0).Err)
		}
	}

	//totCountEventSearch := fmt.Sprintf("TOTALCOUNT:*")
	//totalEventKeys := ScanAndGetKeys(totCountEventSearch)

	for _, key := range companyInfoData {

		go DoPublish(key.Company, key.Tenant, "all", "DASHBOARD", "RESETALL", "RESETALL")
	}

}

func DoPublish(company, tenant int, businessUnit, window, param1, param2 string) {
	authToken := fmt.Sprintf("Bearer %s", accessToken)
	internalAuthToken := fmt.Sprintf("%d:%d", tenant, company)
	serviceurl := fmt.Sprintf("http://%s/DashboardEvent/Publish/%s/%s/%s/%s", CreateHost(dashboardServiceHost, dashboardServicePort), businessUnit, window, param1, param2)
	fmt.Println("URL:>", serviceurl)

	var jsonData = []byte("")
	req, err := http.NewRequest("POST", serviceurl, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authorization", authToken)
	req.Header.Set("companyinfo", internalAuthToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		//panic(err)
		//return false
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	//body, _ := ioutil.ReadAll(resp.Body)
	//result := string(body)
	fmt.Println("response CODE::", string(resp.StatusCode))
	fmt.Println("End======================================:: ", time.Now().UTC())
	if resp.StatusCode == 200 {
		fmt.Println("Return true")
		//return true
	}

	fmt.Println("Return false")
	//return false
}

func CreateHost(_ip, _port string) string {

	fmt.Println("IP:>", _ip)
	fmt.Println("Port:>", _port)

	testIp := net.ParseIP(_ip)
	if testIp.To4() == nil {
		return _ip
	} else {
		return fmt.Sprintf("%s:%s", _ip, _port)
	}
}
