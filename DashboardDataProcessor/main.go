package main

import (
	//"bufio"
	"fmt"
	//"os"
	"time"
)

const layout = "2006-01-02T15:04:05Z07:00"

func main() {

	LoadConfiguration()
	ReloadAllMetaData()
	InitiateRedis()

	//	for {
	//		location, _ := time.LoadLocation("Asia/Colombo")
	//		fmt.Println("location:: " + location.String())

	//		localtime := time.Now().Local()
	//		fmt.Println("localtime:: " + localtime.String())

	//		tmNow := time.Now().In(location)
	//		fmt.Println("tmNow:: " + tmNow.String())

	//		clerTime := time.Date(tmNow.Year(), tmNow.Month(), tmNow.Day(), 23, 59, 59, 0, location)
	//		fmt.Println("Next Clear Time:: " + clerTime.String())

	//		reader := bufio.NewReader(os.Stdin)
	//		fmt.Print("Enter text: ")
	//		text, errRead := reader.ReadString('\n')

	//		if errRead != nil {
	//			fmt.Println(errRead)
	//		}
	//		fmt.Println(text)

	//		if text == "reset" {
	//			OnSetDailySummary(clerTime)
	//			OnSetDailyThresholdBreakDown(clerTime)
	//			OnReset()
	//		}

	//		timer2 := time.NewTimer(time.Duration(time.Second * 5))
	//		<-timer2.C
	//	}

	for {
		fmt.Println("----------Start ClearData----------------------")
		location, _ := time.LoadLocation("Asia/Colombo")
		fmt.Println("location:: " + location.String())

		localtime := time.Now().Local()
		fmt.Println("localtime:: " + localtime.String())

		tmNow := time.Now().In(location)
		fmt.Println("tmNow:: " + tmNow.String())

		clerTime := time.Date(tmNow.Year(), tmNow.Month(), tmNow.Day(), 14, 58, 59, 0, location)
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
