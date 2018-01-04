package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
)

func ReloadAllMetaData() bool {
	fmt.Println("-------------------------ReloadAllMetaData----------------------")
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in ReloadAllMetaData", r)
		}
	}()
	var result bool
	conStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=disable", pgUser, pgPassword, pgDbname, pgHost, pgPort)
	db, err := sql.Open("postgres", conStr)
	if err != nil {
		fmt.Println(err.Error())
		result = false
	}

	var EventClass string
	var EventType string
	var EventCategory string
	var WindowName string
	var Count int
	var FlushEnable bool
	var PersistSession bool
	var UseSession bool
	var ThresholdEnable bool
	var ThresholdValue int

	dataRows, err1 := db.Query("SELECT \"EventClass\", \"EventType\", \"EventCategory\", \"WindowName\", \"Count\", \"FlushEnable\", \"UseSession\", \"PersistSession\", \"ThresholdEnable\", \"ThresholdValue\" FROM \"Dashboard_MetaData\"")
	switch {
	case err1 == sql.ErrNoRows:
		fmt.Println("No metaData with that ID.")
		result = false
	case err1 != nil:
		fmt.Println(err1.Error())
		result = false
	default:
		dashboardMetaInfo = make([]MetaData, 0)
		for dataRows.Next() {
			dataRows.Scan(&EventClass, &EventType, &EventCategory, &WindowName, &Count, &FlushEnable, &UseSession, &PersistSession, &ThresholdEnable, &ThresholdValue)

			fmt.Printf("EventClass is %s\n", EventClass)
			fmt.Printf("EventType is %s\n", EventType)
			fmt.Printf("EventCategory is %s\n", EventCategory)
			fmt.Printf("WindowName is %s\n", WindowName)
			fmt.Printf("Count is %d\n", Count)
			fmt.Printf("FlushEnable is %t\n", FlushEnable)
			fmt.Printf("UseSession is %t\n", UseSession)
			fmt.Printf("PersistSession is %t\n", PersistSession)
			fmt.Printf("ThresholdEnable is %t\n", ThresholdEnable)
			fmt.Printf("ThresholdValue is %d\n", ThresholdValue)

			var mData MetaData
			mData.EventClass = EventClass
			mData.EventType = EventType
			mData.EventCategory = EventCategory
			mData.Count = Count
			mData.FlushEnable = FlushEnable
			mData.ThresholdEnable = ThresholdEnable
			mData.ThresholdValue = ThresholdValue
			mData.UseSession = UseSession
			mData.PersistSession = PersistSession
			mData.WindowName = WindowName

			dashboardMetaInfo = append(dashboardMetaInfo, mData)
		}
		dataRows.Close()
		result = true
	}
	db.Close()
	fmt.Println("DashBoard MetaData:: ", dashboardMetaInfo)
	return result
}

func PersistDailySummaries(summaryRecords []SummeryDetail) {

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in PersistDailySummaries", r)
		}
	}()

	connString := fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=%s sslmode=disable", pgUser, pgPassword, pgHost, pgPort, pgDbname)
	db, err := sql.Open("postgres", connString)
	if err != nil {
		log.Println(err)
	}

	defer db.Close()

	txn, err := db.Begin()
	if err != nil {
		log.Println(err)
	}

	stmt, err := txn.Prepare(pq.CopyIn("Dashboard_DailySummaries", "Company", "Tenant", "BusinessUnit", "WindowName", "Param1", "Param2", "MaxTime", "TotalCount", "TotalTime", "ThresholdValue", "SummaryDate", "createdAt", "updatedAt"))
	if err != nil {
		log.Println(err)
	}

	for _, summaryRecord := range summaryRecords {
		tmNow := time.Now()

		insertValue := fmt.Sprintf("INSERT INTO \"Dashboard_DailySummaries\"(\"Company\", \"Tenant\", \"WindowName\", \"BusinessUnit\", \"Param1\", \"Param2\", \"MaxTime\", \"TotalCount\", \"TotalTime\", \"ThresholdValue\", \"SummaryDate\", \"createdAt\", \"updatedAt\") VALUES ('%d', '%d', '%s', '%s', '%s', '%s', '%d', '%d', '%d', '%d', '%s', '%s', '%s')", summaryRecord.Company, summaryRecord.Tenant, summaryRecord.BusinessUnit, summaryRecord.WindowName, summaryRecord.Param1, summaryRecord.Param2, summaryRecord.MaxTime, summaryRecord.TotalCount, summaryRecord.TotalTime, summaryRecord.ThresholdValue, summaryRecord.SummaryDate, tmNow, tmNow)
		log.Println(insertValue)

		_, err = stmt.Exec(summaryRecord.Company, summaryRecord.Tenant, summaryRecord.BusinessUnit, summaryRecord.WindowName, summaryRecord.Param1, summaryRecord.Param2, summaryRecord.MaxTime, summaryRecord.TotalCount, summaryRecord.TotalTime, summaryRecord.ThresholdValue, summaryRecord.SummaryDate, tmNow, tmNow)
		if err != nil {
			log.Println(err)
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		log.Println(err)
	}

	err = stmt.Close()
	if err != nil {
		log.Println(err)
	}

	err = txn.Commit()
	if err != nil {
		log.Println(err)
	}

	log.Println("--Done--")

}

func PersistThresholdBreakDown(thresholdRecords []ThresholdBreakDownDetail) {

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in PersistsThresholdBreakDown", r)
		}
	}()

	connString := fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=%s sslmode=disable", pgUser, pgPassword, pgHost, pgPort, pgDbname)
	db, err := sql.Open("postgres", connString)
	if err != nil {
		log.Println(err)
	}

	defer db.Close()

	txn, err := db.Begin()
	if err != nil {
		log.Println(err)
	}

	stmt, err := txn.Prepare(pq.CopyIn("Dashboard_ThresholdBreakDowns", "Company", "Tenant", "BusinessUnit", "WindowName", "Param1", "Param2", "BreakDown", "ThresholdCount", "SummaryDate", "Hour", "createdAt", "updatedAt"))
	if err != nil {
		log.Println(err)
	}

	for _, thresholdRecord := range thresholdRecords {
		tmNow := time.Now()

		insertValue := fmt.Sprintf("INSERT INTO \"Dashboard_ThresholdBreakDowns\"(\"Company\", \"Tenant\", \"BusinessUnit\", \"WindowName\", \"Param1\", \"Param2\", \"BreakDown\", \"ThresholdCount\", \"SummaryDate\", \"Hour\", \"createdAt\", \"updatedAt\") VALUES ('%d', '%d', '%s', '%s', '%s', '%s', '%s', '%d', '%s', '%d', '%s', '%s')", thresholdRecord.Company, thresholdRecord.Tenant, thresholdRecord.WindowName, thresholdRecord.Param1, thresholdRecord.Param2, thresholdRecord.BreakDown, thresholdRecord.ThresholdCount, thresholdRecord.SummaryDate, thresholdRecord.Hour, tmNow, tmNow)
		log.Println(insertValue)

		_, err = stmt.Exec(thresholdRecord.Company, thresholdRecord.Tenant, thresholdRecord.BusinessUnit, thresholdRecord.WindowName, thresholdRecord.Param1, thresholdRecord.Param2, thresholdRecord.BreakDown, thresholdRecord.ThresholdCount, thresholdRecord.SummaryDate, thresholdRecord.Hour, tmNow, tmNow)
		if err != nil {
			log.Fatal(err)
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		log.Println(err)
	}

	err = stmt.Close()
	if err != nil {
		log.Println(err)
	}

	err = txn.Commit()
	if err != nil {
		log.Println(err)
	}

	log.Println("--Done--")

}
