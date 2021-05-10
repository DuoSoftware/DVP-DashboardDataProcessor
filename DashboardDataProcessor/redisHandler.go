package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

var connectionOptions redis.UniversalOptions
var redisCtx = context.Background()
var rdb redis.UniversalClient


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



	log.Println("RedisMode:", redisMode)
	log.Println("RedisIp:", redisIp)
	log.Println("RedisDb:", redisDb)
	log.Println("SentinelHosts:", sentinelHosts)
	log.Println("SentinelPort:", sentinelPort)


	if redisMode == "sentinel" {

		sentinelIps := strings.Split(sentinelHosts, ",")
		var ips []string;

		if len(sentinelIps) > 1 {

			for _, ip := range sentinelIps{
				ipPortArray := strings.Split(ip, ":")
				sentinelIp := ip;
				if(len(ipPortArray) > 1){
					sentinelIp = fmt.Sprintf("%s:%s", ipPortArray[0], ipPortArray[1])
				}else{
					sentinelIp = fmt.Sprintf("%s:%s", ip, sentinelPort)
				}
				ips = append(ips, sentinelIp)
				
			}

			connectionOptions.Addrs = ips
			connectionOptions.MasterName = redisClusterName

		} else {
			fmt.Println("Not enough sentinel servers")
			os.Exit(0)
		}


	} else {

		redisIps := strings.Split(redisIp, ",")
		var ips []string;
		if len(redisIps) > 0 {

			for _, ip := range redisIps{
				ipPortArray := strings.Split(ip, ":")
				redisAddr := ip;
				if(len(ipPortArray) > 1){
					redisAddr = fmt.Sprintf("%s:%s", ipPortArray[0], ipPortArray[1])
				}else{
					redisAddr = fmt.Sprintf("%s:%s", ip, redisPort)
				}
				ips = append(ips, redisAddr)
				
			}

			connectionOptions.Addrs = ips

		} else {
			fmt.Println("Not enough redis servers")
			os.Exit(0)
		}
	}

	connectionOptions.DB, _ = strconv.Atoi(redisDb) 
	connectionOptions.Password = redisPassword

	rdb = redis.NewUniversalClient(&connectionOptions)


}


func ScanAndGetKeys(pattern string) []string {


	defer func() {

		if r := recover(); r != nil {
			fmt.Println("Recovered in ScanAndGetKeys", r)
		}
	}()

	matchingKeys := make([]string, 0)

	log.Println("Start ScanAndGetKeys:: ", pattern)

	var ctx = context.TODO()
	iter := rdb.Scan(ctx, 0, pattern, 1000).Iterator()
	for iter.Next(ctx) {
		
		AppendIfMissing(matchingKeys, iter.Val())
	}
	if err := iter.Err(); err != nil {

		fmt.Println("ScanAndGetKeys","SCAN",err)
	}


	return matchingKeys
}


func Contains(a []CompanyInfo, c int, t int) bool {
	for _, n := range a {
		if c == n.Company && t == n.Tenant {
			return true
		}
	}
	return false
}

func OnSetDailySummary(_date time.Time) {
	
	

	totCountEventSearch := fmt.Sprintf("TOTALCOUNT:*")
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in OnSetDailySummary", r)
		}
	}()



	todaySummary := make([]SummeryDetail, 0)
	totalEventKeys := ScanAndGetKeys(totCountEventSearch)

	for _, key := range totalEventKeys {
		fmt.Println("Key: ", key)
		keyItems := strings.Split(key, ":")

		if len(keyItems) >= 7 {
			summery := SummeryDetail{}
			cmpData := CompanyInfo{}

			tenant, _ := strconv.Atoi(keyItems[1])
			company, _ := strconv.Atoi(keyItems[2])
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
					tmx, tmxErr := rdb.HGet(context.TODO(),sessEvents[0], "time").Result()
					errHandler("OnSetDailySummary", "Cmd", tmxErr)
					tm2, _ := time.Parse(layout, tmx)
					currentTime = int(_date.Sub(tm2.Local()).Seconds())
					fmt.Println("currentTime: ", currentTime)
				}
			}
			totTimeEventName := fmt.Sprintf("TOTALTIME:%d:%d:%s:%s:%s:%s", tenant, company, summery.BusinessUnit, summery.WindowName, summery.Param1, summery.Param2)
			maxTimeEventName := fmt.Sprintf("MAXTIME:%d:%d:%s:%s:%s:%s", tenant, company, summery.BusinessUnit, summery.WindowName, summery.Param1, summery.Param2)
			thresholdEventName := fmt.Sprintf("THRESHOLD:%d:%d:%s:%s:%s:%s", tenant, company, summery.BusinessUnit, summery.WindowName, summery.Param1, summery.Param2)

			log.Println("totTimeEventName: ", totTimeEventName)
			log.Println("maxTimeEventName: ", maxTimeEventName)
			log.Println("thresholdEventName: ", thresholdEventName)

			pipe := rdb.TxPipeline()

			getResp := pipe.Get(context.TODO(), key)
			getTotalTimeEventNameResponse := pipe.Get(context.TODO(),totTimeEventName) 
			getMaxTimeEventName :=  pipe.Get(context.TODO(),maxTimeEventName) 
			getThresholdEventName := pipe.Get(context.TODO(),thresholdEventName)

			_, err := pipe.Exec(context.TODO())
			errHandler("OnSetDailySummary", "Pipe", err)
			
			

			totCount, _ := getResp.Int()
			totTime, _ := getTotalTimeEventNameResponse.Int()
			maxTime, _ := getMaxTimeEventName.Int()
			threshold, _ := getThresholdEventName.Int()


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
	}

	if len(todaySummary) > 0 {
		go PersistDailySummaries(todaySummary)
	}

	fmt.Println("Company Data ++++++++ ", companyInfoData)
}

func OnSetDailyThresholdBreakDown(_date time.Time) {



	thresholdEventSearch := fmt.Sprintf("THRESHOLDBREAKDOWN:*")
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in OnSetDailyThesholdBreakDown", r)
		}

	}()

	

	thresholdRecords := make([]ThresholdBreakDownDetail, 0)
	thresholdEventKeys := ScanAndGetKeys(thresholdEventSearch)

	for _, key := range thresholdEventKeys {
		fmt.Println("Key: ", key)
		keyItems := strings.Split(key, ":")

		if len(keyItems) >= 10 {
			summery := ThresholdBreakDownDetail{}
			tenant, _ := strconv.Atoi(keyItems[1])
			company, _ := strconv.Atoi(keyItems[2])
			hour, _ := strconv.Atoi(keyItems[7])
			summery.Tenant = tenant
			summery.Company = company
			summery.BusinessUnit = keyItems[3]
			summery.WindowName = keyItems[4]
			summery.Param1 = keyItems[5]
			summery.Param2 = keyItems[6]
			summery.BreakDown = fmt.Sprintf("%s-%s", keyItems[8], keyItems[9])
			summery.Hour = hour

			thCount, thCountErr := rdb.Get(context.TODO(), key).Int()
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


	_windowList := make([]string, 0)
	_keysToRemove := make([]string, 0)
	_loginSessions := make([]string, 0)
	_productivitySessions := make([]string, 0)

	fmt.Println("---------------------Use Memory----------------------")
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
		rKey := fmt.Sprintf("remove_: %s", remove)
		errHandler("OnReset", rKey, rdb.Del(context.TODO(), remove).Err())
	}
	for _, session := range _loginSessions {
		fmt.Println("readdSession: ", session)
		errHandler("OnReset", "Cmd", rdb.HSet(context.TODO(), session, "time", tm.Format(layout)).Err())
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


			pipe := rdb.TxPipeline()

			pipe.HMSet(context.TODO(), LsessParamEventName, "businessUnit", sessItemsL[3], "param1", sessItemsL[6], "param2", sessItemsL[7])
			pipe.Set(context.TODO(), LtotTimeEventName, 0, 0)
			pipe.Set(context.TODO(), LtotCountEventName, 0, 0)
			pipe.Set(context.TODO(), LtotTimeEventNameWithoutParams, 0, 0)
			pipe.Set(context.TODO(), LtotCountEventNameWithoutParams, 0, 0)
			pipe.Set(context.TODO(), LtotTimeEventNameWithSingleParam, 0, 0)
			pipe.Set(context.TODO(), LtotCountEventNameWithSingleParam, 0, 0)
			pipe.Set(context.TODO(), LtotTimeEventNameWithLastParam, 0, 0)
			pipe.Set(context.TODO(), LtotCountEventNameWithLastParam, 0, 0)
			pipe.Set(context.TODO(), LtotTimeEventName_BusinessUnit, 0, 0)
			pipe.Set(context.TODO(), LtotCountEventName_BusinessUnit, 0, 0)
			pipe.Set(context.TODO(), LtotTimeEventNameWithoutParams_BusinessUnit, 0, 0)
			pipe.Set(context.TODO(), LtotCountEventNameWithoutParams_BusinessUnit, 0, 0)
			pipe.Set(context.TODO(), LtotTimeEventNameWithSingleParam_BusinessUnit, 0, 0)
			pipe.Set(context.TODO(), LtotCountEventNameWithSingleParam_BusinessUnit, 0, 0)
			pipe.Set(context.TODO(), LtotTimeEventNameWithLastParam_BusinessUnit, 0, 0)
			pipe.Set(context.TODO(), LtotCountEventNameWithLastParam_BusinessUnit, 0, 0)


			resp, err := pipe.Exec(rdb.Context())
			fmt.Println(resp)
			errHandler("OnReset", "Pipe", err)
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
