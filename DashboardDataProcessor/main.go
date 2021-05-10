package main

import (
	//"bufio"
	"fmt"
)

const layout = "2006-01-02T15:04:05Z07:00"

func main() {

	LoadConfiguration()
	ReloadAllMetaData()
	InitiateRedis()

	for {

		task, err := GetTask()

		if err == nil {
			OnSetDailySummary(task.Company, task.Tenant)
			OnSetDailyThresholdBreakDown(task.Company, task.Tenant)
			OnReset(task.Company, task.Tenant)

		}
	}

}

func errHandler(errorFrom, command string, err error) {
	if err != nil {
		fmt.Println("error:", errorFrom, ":: ", command, ":: ", err)
	}
}
