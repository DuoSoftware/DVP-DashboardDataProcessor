package main

import (
	"fmt"
	"time"
)

const layout = "2006-01-02T15:04:05Z07:00"

func main() {
	fmt.Println("Version 2.0")
	LoadConfiguration()
	ReloadAllMetaData()
	InitiateRedis()

	for {
		fmt.Println("----------Start ClearData----------------------")
		location, _ := time.LoadLocation("Asia/Colombo")
		fmt.Println("location:: " + location.String())

		localtime := time.Now().Local()
		fmt.Println("localtime:: " + localtime.String())

		tmNow := time.Now().In(location)
		fmt.Println("tmNow:: " + tmNow.String())

		clerTime := time.Date(tmNow.Year(), tmNow.Month(), tmNow.Day(), 15, 52, 59, 0, location)
		fmt.Println("Next Clear Time:: " + clerTime.String())

		timeToWait := clerTime.Sub(tmNow)
		fmt.Println("timeToWait:: " + timeToWait.String())
		//OnSetDailySummary(clerTime)
		//OnSetDailyThresholdBreakDown(clerTime)
		timer := time.NewTimer(timeToWait)
		<-timer.C
		OnSetDailySummary(clerTime)
		OnSetDailyThresholdBreakDown(clerTime)
		OnReset()

		fmt.Println("----------ClearData Wait after reset----------------------")
		timer2 := time.NewTimer(time.Duration(time.Minute * 5))
		<-timer2.C
		fmt.Println("----------End ClearData Wait after reset----------------------")
	}

}

func errHandler(errorFrom, command string, err error) {
	if err != nil {
		fmt.Println("error:", errorFrom, ":: ", command, ":: ", err)
	}
}
