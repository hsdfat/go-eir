package http

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hsdfat/go-eir/internal/adapters/postgres"
	"github.com/hsdfat/go-eir/internal/domain/models"
	"github.com/hsdfat/go-eir/internal/domain/ports"
	"github.com/hsdfat/go-eir/internal/logger"
	legacyModels "github.com/hsdfat/go-eir/models"
	"github.com/hsdfat/go-eir/pkg/logic"
	"github.com/hsdfat/go-eir/utils"
	"github.com/jmoiron/sqlx"
)

/*
==========================
IMEI
- go test -v -run TestImei ./internal/adapters/http/
- CREATE TABLE IMEI_INFO (
     StartIMEI VARCHAR(16) PRIMARY KEY,
     EndIMEI TEXT[] DEFAULT '{}',
     Color CHAR(1) NOT NULL CHECK (Color IN ('w', 'b', 'g'))
  );
- go test -v -run "TestImei" ./internal/adapters/http/
- go test -v -run "TestImei/Insert_IMEI_valid" ./internal/adapters/http/

Need check:
- TestImei/Eir_Add_6 & 7: insert same imei and difference color
- Overload testcase: Eir_Add_11 -> Eir_Add_14(handler overload and support overload fields)
- CheckImei function, current use ImeiSampleData with fixed values
- Has support to insert list value?
- Eir_Add_22: value return
- Eir_Add_29:
	insertImeiReqList := []ports.ImeiInfoInsert{
			ports.ImeiInfoInsert{
				Imei:  "12345678901234",
				Color: "w",
			},
			ports.ImeiInfoInsert{
				Imei:  "1234567890123456",
				Color: "b",
			},
		}
	=> FAIL -> need check
	- Step 2: FAIL with unexpected output:   expected: imei_exist, can not insert

==========================
TAC
CREATE TABLE TAC_INFO (
    KeyTAC VARCHAR(64) PRIMARY KEY,
    StartRangeTAC VARCHAR(20) NOT NULL,
    EndRangeTAC VARCHAR(20) NOT NULL,
    Color VARCHAR(10) NOT NULL CHECK (Color IN ('black', 'white', 'grey')),
    PrevLink VARCHAR(64) REFERENCES TAC_INFO(KeyTAC) ON DELETE SET NULL
);
- Eir_Add_63: range_exist error

==========================
Check Imei
- Xem lai phuong thuc: CheckImei() trong file: server_test.go:
	+ ko nen de phuong thuc nghiep vu trong file unit-test
	+ neu trong truong hop co loi: error tra ve phai co gia tri, ko nen de nil
	+ tham so usage ctx
	+ lookupImeiInfo(imei): vong for cho 1 const
*/

func createEirService() (*mockEIRService, func()) {
	// Create and return an instance of EirService
	dbURL := "host=localhost port=5433 user=eir password=eir_password dbname=eir sslmode=disable"
	//dbURL := "host=14.225.198.206 port=5432 user=adong password=adong123 dbname=adongfoodv4 sslmode=disable"
	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		panic(err)
	}

	// Clean up function to close the database connection
	cleanup := func() {
		db.Close()
	}
	return &mockEIRService{
		imeiRepo: postgres.NewIMEIRepository(db),
	}, cleanup
}

func TestEirInsert(t *testing.T) {
	t.Log("Get IMEI TAC Info")

	//create object to connect db
	eirService, cleanup := createEirService()
	defer cleanup()
	log := logger.New("eir", "info")
	server := NewServer(ServerConfig{
		ListenAddr: "127.0.0.1:8080", // ip loopback
	}, eirService, &mockStatsCollector{}, log)

	//create EIR server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start EIR server: %v", err)
	}
	defer server.Stop()
	time.Sleep(200 * time.Millisecond)
	t.Logf("Created EIR server: %v", server.GetAddr())

	// Unit test 01: Eir_Add_1 Insert IMEI valid
	t.Run("Eir_Add_1", func(t *testing.T) {
		t.Log("Eir_Add_1 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		//Insert IMEI with black color
		insertReq := ports.ImeiInfoInsert{
			Imei:  "1",
			Color: "b",
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		insertResult := logic.InsertImei(eirService.imeiRepo, insertReq.Imei, insertReq.Color, legacyStatus)

		if insertResult.Error != "" {
			t.Fatal("Insert imei to db failed")
		}

		//verify IMEI inserted
		//checkResult := logic.CheckImei(insertReq.Imei, legacyStatus) //todo: need check lookupImeiInfo() in this func
		//if checkResult.Status != "ok" {
		//	t.Fatal("Check imei failed after inserted with color: ", checkResult.Color)
		//}
		imeiCheckLength := utils.GetImeiCheckLength()
		var startImeiExt string
		if len(insertReq.Imei) < imeiCheckLength {
			startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength, insertReq.Imei) //The * in the format string tells fmt.Sprintf to read the width from the next argument
		}
		VerifyImeiInDb(eirService, insertReq, startImeiExt, t)
	})

	// Unit test 02:
	t.Run("Eir_Add_2", func(t *testing.T) {
		t.Log("Eir_Add_2 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		insertReq := ports.ImeiInfoInsert{
			Imei:  "123456789012345",
			Color: "w",
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		insertResult := logic.InsertImei(eirService.imeiRepo, insertReq.Imei, insertReq.Color, legacyStatus)

		if insertResult.Error != "" {
			t.Fatal("Insert imei to db failed")
		}
		imeiCheckLength := utils.GetImeiCheckLength()
		t.Log("IMEI_CHECK_LENGTH:", imeiCheckLength)
		if len(insertReq.Imei) > imeiCheckLength {
			startImeiExt := insertReq.Imei[:imeiCheckLength]
			VerifyImeiInDb(eirService, insertReq, startImeiExt, t)
		}
	})

	// Unit test 03:
	t.Run("Eir_Add_3", func(t *testing.T) {
		t.Log("Eir_Add_3 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		insertReq := ports.ImeiInfoInsert{
			Imei:  "12345678901234",
			Color: "w",
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		insertResult := logic.InsertImei(eirService.imeiRepo, insertReq.Imei, insertReq.Color, legacyStatus)
		if insertResult.Error != "" {
			t.Fatal("Insert imei to db failed")
		}
		var startImeiExt string
		imeiCheckLength := utils.GetImeiCheckLength()
		if len(insertReq.Imei) > imeiCheckLength {
			startImeiExt = insertReq.Imei[:imeiCheckLength]
		} else {
			startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength, insertReq.Imei)
		}
		VerifyImeiInDb(eirService, insertReq, startImeiExt, t)
	})

	//Unit test 04
	t.Run("Eir_Add_4", func(t *testing.T) {
		t.Log("Eir_Add_4 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		insertReq := ports.ImeiInfoInsert{
			Imei:  "12345678901234567",
			Color: "w",
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		err := os.Setenv("IMEI_MAX_LENGTH", "17")
		if err != nil {
			t.Fatal(err)
		}
		insertResult := logic.InsertImei(eirService.imeiRepo, insertReq.Imei, insertReq.Color, legacyStatus)
		if insertResult.Error != "" {
			t.Fatal("Insert imei to db failed with error: ", insertResult.Error)
		}
		var startImeiExt string
		imeiCheckLength := utils.GetImeiCheckLength()
		if len(insertReq.Imei) > imeiCheckLength {
			startImeiExt = insertReq.Imei[:imeiCheckLength]
		} else {
			startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength, insertReq.Imei)
		}
		VerifyImeiInDb(eirService, insertReq, startImeiExt, t)
	})

	//Unit test 05: todo: Need check
	t.Run("Eir_Add_5", func(t *testing.T) {
		t.Log("Eir_Add_5 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		err := os.Setenv("IMEI_MAX_LENGTH", "17")
		if err != nil {
			t.Fatal(err)
		}
		insertImeiReqList := []ports.ImeiInfoInsert{
			ports.ImeiInfoInsert{
				Imei:  "12345678901234",
				Color: "b",
			},
			ports.ImeiInfoInsert{
				Imei:  "123456789012341",
				Color: "b", //todo: need check: must to same color with first item.
			},
			ports.ImeiInfoInsert{
				Imei:  "1234567890123411",
				Color: "b",
			},
			ports.ImeiInfoInsert{
				Imei:  "12345678901234111",
				Color: "b",
			},
			ports.ImeiInfoInsert{
				Imei:  "12345678901234112",
				Color: "b",
			},
			ports.ImeiInfoInsert{
				Imei:  "12345678901234222",
				Color: "b",
			},
			ports.ImeiInfoInsert{
				Imei:  "12345678901234333",
				Color: "b",
			},
			ports.ImeiInfoInsert{
				Imei:  "12345678901234444",
				Color: "b",
			},
		}

		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		for _, imeiValInsert := range insertImeiReqList {
			insertResult := logic.InsertImei(eirService.imeiRepo, imeiValInsert.Imei, imeiValInsert.Color, legacyStatus)
			if insertResult.Error != "" {
				t.Fatal("Insert imei to db failed with error: ", insertResult.Error)
			}
			//verify
			var startImeiExt string
			imeiCheckLength := utils.GetImeiCheckLength()
			if len(imeiValInsert.Imei) > imeiCheckLength {
				startImeiExt = imeiValInsert.Imei[:imeiCheckLength]
			} else {
				startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength, imeiValInsert.Imei)
			}
			VerifyImeiInDb(eirService, imeiValInsert, startImeiExt, t)
		}
	})

	//Unit test 06: todo: need check conflict color
	t.Run("Eir_Add_6", func(t *testing.T) {
		t.Log("Eir_Add_6 testcase")
		eirService.ClearImeiInfo()
		err := os.Setenv("IMEI_CHECK_LENGTH", "14")
		if err != nil {
			t.Fatal(err)
		}
		err2 := os.Setenv("IMEI_MAX_LENGTH", "14")
		if err2 != nil {
			t.Fatal(err2)
		}
		insertImeiReqList := []ports.ImeiInfoInsert{
			ports.ImeiInfoInsert{
				Imei:  "12345678901234",
				Color: "w",
			},
			ports.ImeiInfoInsert{
				Imei:  "12345678901234",
				Color: "b",
			},
			ports.ImeiInfoInsert{
				Imei:  "12345678901234",
				Color: "g",
			},
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		for _, imeiValInsert := range insertImeiReqList {
			insertResult := logic.InsertImei(eirService.imeiRepo, imeiValInsert.Imei, imeiValInsert.Color, legacyStatus)
			if insertResult.Error != "" {
				t.Fatal("Insert imei to db failed with error: ", insertResult.Error)
			}
			//verify
			var startImeiExt string
			imeiCheckLength := utils.GetImeiCheckLength()
			if len(imeiValInsert.Imei) > imeiCheckLength {
				startImeiExt = imeiValInsert.Imei[:imeiCheckLength]
			} else {
				startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength, imeiValInsert.Imei)
			}
			VerifyImeiInDb(eirService, imeiValInsert, startImeiExt, t)
		}
	})

	//Unit test 07: todo: need check color conflict
	t.Run("Eir_Add_7", func(t *testing.T) {
		t.Log("Eir_Add_7 testcase")
		eirService.ClearImeiInfo()
		err := os.Setenv("IMEI_CHECK_LENGTH", "14")
		if err != nil {
			t.Fatal(err)
		}
		err2 := os.Setenv("IMEI_MAX_LENGTH", "10")
		if err2 != nil {
			t.Fatal(err2)
		}
		insertImeiReqList := []ports.ImeiInfoInsert{
			ports.ImeiInfoInsert{
				Imei:  "1",
				Color: "w",
			},
			ports.ImeiInfoInsert{
				Imei:  "2",
				Color: "b",
			},
			ports.ImeiInfoInsert{
				Imei:  "1234567890",
				Color: "g",
			},
			ports.ImeiInfoInsert{ //todo: need check color conflict
				Imei:  "1",
				Color: "b",
			},
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		for _, imeiValInsert := range insertImeiReqList {
			insertResult := logic.InsertImei(eirService.imeiRepo, imeiValInsert.Imei, imeiValInsert.Color, legacyStatus)
			if insertResult.Error != "" {
				t.Fatal("Insert imei to db failed with error: ", insertResult.Error)
			}
			//verify
			var startImeiExt string
			imeiCheckLength := utils.GetImeiCheckLength()
			if len(imeiValInsert.Imei) > imeiCheckLength {
				startImeiExt = imeiValInsert.Imei[:imeiCheckLength]
			} else {
				startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength, imeiValInsert.Imei)
			}
			VerifyImeiInDb(eirService, imeiValInsert, startImeiExt, t)
		}
	})

	//todo: need check scenario
	t.Run("Eir_Add_8", func(t *testing.T) {
		t.Log("Eir_Add_8 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		err := os.Setenv("IMEI_CHECK_LENGTH", "14")
		if err != nil {
			t.Fatal(err)
		}
		err2 := os.Setenv("IMEI_MAX_LENGTH", "16")
		if err2 != nil {
			t.Fatal(err2)
		}
		insertReq := ports.ImeiInfoInsert{
			Imei:  "223456789012345",
			Color: "b",
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		insertResult := logic.InsertImei(eirService.imeiRepo, insertReq.Imei, insertReq.Color, legacyStatus)
		if insertResult.Error != "" {
			t.Fatal("Insert imei to db failed with error: ", insertResult.Error)
		}
		var startImeiExt string
		imeiCheckLength := utils.GetImeiCheckLength()
		if len(insertReq.Imei) > imeiCheckLength {
			startImeiExt = insertReq.Imei[:imeiCheckLength]
		} else {
			startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength, insertReq.Imei)
		}
		VerifyImeiInDb(eirService, insertReq, startImeiExt, t)
		//#####################
		errC2 := os.Setenv("IMEI_CHECK_LENGTH", "15")
		if errC2 != nil {
			t.Fatal(errC2)
		}
		insertReq2 := ports.ImeiInfoInsert{
			Imei:  "223456789012345",
			Color: "b",
		}
		insertResult2 := logic.InsertImei(eirService.imeiRepo, insertReq2.Imei, insertReq2.Color, legacyStatus)
		if insertResult2.Error != "" {
			t.Fatal("Insert imei to db failed with error: ", insertResult2.Error)
		}
		var startImeiExt2 string
		imeiCheckLength2 := utils.GetImeiCheckLength()
		if len(insertReq2.Imei) > imeiCheckLength2 {
			startImeiExt2 = insertReq2.Imei[:imeiCheckLength2]
		} else {
			startImeiExt2 = fmt.Sprintf("%-*s", imeiCheckLength2, insertReq2.Imei)
		}
		VerifyImeiInDb(eirService, insertReq2, startImeiExt2, t)

		//#####################
		errC3 := os.Setenv("IMEI_CHECK_LENGTH", "1")
		if errC3 != nil {
			t.Fatal(errC3)
		}
		insertReq3 := ports.ImeiInfoInsert{
			Imei:  "223456789012345",
			Color: "b",
		}
		insertResult3 := logic.InsertImei(eirService.imeiRepo, insertReq3.Imei, insertReq3.Color, legacyStatus)
		if insertResult3.Error != "" {
			t.Fatal("Insert imei to db failed with error: ", insertResult3.Error)
		}
		var startImeiExt3 string
		imeiCheckLength3 := utils.GetImeiCheckLength()
		if len(insertReq3.Imei) > imeiCheckLength3 {
			startImeiExt3 = insertReq3.Imei[:imeiCheckLength3]
		} else {
			startImeiExt3 = fmt.Sprintf("%-*s", imeiCheckLength3, insertReq3.Imei)
		}
		VerifyImeiInDb(eirService, insertReq3, startImeiExt3, t)

		//#####################
		errC4 := os.Setenv("IMEI_CHECK_LENGTH", "16")
		if errC4 != nil {
			t.Fatal(errC4)
		}
		insertReq4 := ports.ImeiInfoInsert{
			Imei:  "2234567890123456",
			Color: "b",
		}
		insertResult4 := logic.InsertImei(eirService.imeiRepo, insertReq4.Imei, insertReq4.Color, legacyStatus)
		if insertResult4.Error != "" {
			t.Fatal("Insert imei to db failed with error: ", insertResult4.Error)
		}
		var startImeiExt4 string
		imeiCheckLength4 := utils.GetImeiCheckLength()
		if len(insertReq4.Imei) > imeiCheckLength4 {
			startImeiExt4 = insertReq4.Imei[:imeiCheckLength4]
		} else {
			startImeiExt4 = fmt.Sprintf("%-*s", imeiCheckLength4, insertReq4.Imei)
		}
		VerifyImeiInDb(eirService, insertReq4, startImeiExt4, t)
	})

	//todo: need check imei_check_length can > imei_max_length ?
	t.Run("Eir_Add_9", func(t *testing.T) {
		t.Log("Eir_Add_9 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		os.Setenv("IMEI_MAX_LENGTH", "1")

		insertReq1 := ports.ImeiInfoInsert{
			Imei:  "1",
			Color: "w",
		}

		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}

		insertResult1 := logic.InsertImei(eirService.imeiRepo, insertReq1.Imei, insertReq1.Color, legacyStatus)
		if insertResult1.Error != "" {
			t.Fatal("Insert imei to db failed with error: ", insertResult1.Error)
		}
		var startImeiExt string
		imeiCheckLength1 := utils.GetImeiCheckLength()
		if len(insertReq1.Imei) > imeiCheckLength1 {
			startImeiExt = insertReq1.Imei[:imeiCheckLength1]
		} else {
			startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength1, insertReq1.Imei)
		}
		VerifyImeiInDb(eirService, insertReq1, startImeiExt, t)

		insertReq2 := ports.ImeiInfoInsert{
			Imei:  "2",
			Color: "w",
		}
		insertResult2 := logic.InsertImei(eirService.imeiRepo, insertReq2.Imei, insertReq2.Color, legacyStatus)
		if insertResult2.Error != "" {
			t.Fatal("Insert imei to db failed with error: ", insertResult2.Error)
		}
		var startImeiExt2 string
		imeiCheckLength2 := utils.GetImeiCheckLength()
		if len(insertReq2.Imei) > imeiCheckLength2 {
			startImeiExt2 = insertReq2.Imei[:imeiCheckLength2]
		} else {
			startImeiExt2 = fmt.Sprintf("%-*s", imeiCheckLength2, insertReq2.Imei)
		}
		VerifyImeiInDb(eirService, insertReq2, startImeiExt2, t)

		insertReq3 := ports.ImeiInfoInsert{
			Imei:  "3",
			Color: "w",
		}
		insertResult3 := logic.InsertImei(eirService.imeiRepo, insertReq3.Imei, insertReq3.Color, legacyStatus)
		if insertResult3.Error != "" {
			t.Fatal("Insert imei to db failed with error: ", insertResult3.Error)
		}
		var startImeiExt3 string
		imeiCheckLength3 := utils.GetImeiCheckLength()
		if len(insertReq3.Imei) > imeiCheckLength3 {
			startImeiExt3 = insertReq3.Imei[:imeiCheckLength3]
		} else {
			startImeiExt3 = fmt.Sprintf("%-*s", imeiCheckLength3, insertReq3.Imei)
		}
		VerifyImeiInDb(eirService, insertReq3, startImeiExt3, t)
	})

	//todo: tc 10

	//Unit test 11: todo: need support flag_overload_ram_cpu level
	t.Run("Eir_Add_11", func(t *testing.T) {
		t.Log("Eir_Add_11 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		insertReq := ports.ImeiInfoInsert{
			Imei:  "123456789012345",
			Color: "w",
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 1,
			TPSOverload:   false,
		}
		err := os.Setenv("IMEI_MAX_LENGTH", "17")
		if err != nil {
			t.Fatal(err)
		}
		insertResult := logic.InsertImei(eirService.imeiRepo, insertReq.Imei, insertReq.Color, legacyStatus)
		if insertResult.Error != "" {
			t.Fatal("Insert imei to db failed with error: ", insertResult.Error)
		}
		var startImeiExt string
		imeiCheckLength := utils.GetImeiCheckLength()
		if len(insertReq.Imei) > imeiCheckLength {
			startImeiExt = insertReq.Imei[:imeiCheckLength]
		} else {
			startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength, insertReq.Imei)
		}
		VerifyImeiInDb(eirService, insertReq, startImeiExt, t)
	})

	//Unit test 15
	t.Run("Eir_Add_15", func(t *testing.T) {
		t.Log("Eir_Add_15 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		insertReq := ports.ImeiInfoInsert{
			Imei:  "",
			Color: "b",
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		err := os.Setenv("IMEI_MAX_LENGTH", "17")
		if err != nil {
			t.Fatal(err)
		}
		insertResult := logic.InsertImei(eirService.imeiRepo, insertReq.Imei, insertReq.Color, legacyStatus)
		if insertResult.Error == "invalid_parameter" {
			t.Skip("PASS with error: ", insertResult.Error)
		}
		t.Fatal("Result return not exactly")
	})

	//Unit test 16
	t.Run("Eir_Add_16", func(t *testing.T) {
		t.Log("Eir_Add_16 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		err := os.Setenv("IMEI_MAX_LENGTH", "17")
		if err != nil {
			t.Fatal(err)
		}
		err2 := os.Setenv("IMEI_CHECK_LENGTH", "14")
		if err2 != nil {
			t.Fatal(err2)
		}
		insertReq := ports.ImeiInfoInsert{
			Imei:  "123456789012345678",
			Color: "b",
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		insertResult := logic.InsertImei(eirService.imeiRepo, insertReq.Imei, insertReq.Color, legacyStatus)
		if insertResult.Error == "invalid_length" {
			t.Skip("PASS with error: ", insertResult.Error)
		}
		t.Fatal("Result return not exactly")
	})

	//Unit test 17
	t.Run("Eir_Add_17", func(t *testing.T) {
		t.Log("Eir_Add_17 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		err := os.Setenv("IMEI_MAX_LENGTH", "10")
		if err != nil {
			t.Fatal(err)
		}
		err2 := os.Setenv("IMEI_CHECK_LENGTH", "14")
		if err2 != nil {
			t.Fatal(err2)
		}
		insertReq := ports.ImeiInfoInsert{
			Imei:  "123456789012345",
			Color: "b",
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		insertResult := logic.InsertImei(eirService.imeiRepo, insertReq.Imei, insertReq.Color, legacyStatus)
		insertReq2 := ports.ImeiInfoInsert{
			Imei:  "1234567890123",
			Color: "b",
		}
		insertResult2 := logic.InsertImei(eirService.imeiRepo, insertReq2.Imei, insertReq2.Color, legacyStatus)

		if insertResult.Error == "invalid_length" && insertResult2.Error == "invalid_length" {
			t.Skip("PASS with error: ", insertResult.Error)
		}
		t.Fatal("Result return not exactly")
	})

	//Unit test 18
	t.Run("Eir_Add_18", func(t *testing.T) {
		t.Log("Eir_Add_18 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		err := os.Setenv("IMEI_MAX_LENGTH", "0")
		if err != nil {
			t.Fatal(err)
		}
		err2 := os.Setenv("IMEI_CHECK_LENGTH", "14")
		if err2 != nil {
			t.Fatal(err2)
		}
		insertReq := ports.ImeiInfoInsert{
			Imei:  "223456789012345",
			Color: "b",
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		insertResult := logic.InsertImei(eirService.imeiRepo, insertReq.Imei, insertReq.Color, legacyStatus)
		if insertResult.Error == "invalid_length" {
			t.Skip("PASS with error: ", insertResult.Error)
		}
		t.Fatal("Result return not exactly with error: ", insertResult.Error)
	})

	//Unit test 19: need check scenario
	t.Run("Eir_Add_19", func(t *testing.T) {
		t.Log("Eir_Add_19 testcase")
		eirService.ClearImeiInfo()
		err := os.Setenv("IMEI_MAX_LENGTH", "16")
		if err != nil {
			t.Fatal(err)
		}
		err2 := os.Setenv("IMEI_CHECK_LENGTH", "14")
		if err2 != nil {
			t.Fatal(err2)
		}
		insertImeiReqList := []ports.ImeiInfoInsert{
			ports.ImeiInfoInsert{
				Imei:  "12345678901234",
				Color: "w",
			},
			ports.ImeiInfoInsert{
				Imei:  "1234567890123412",
				Color: "w",
			},
		}

		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		for _, imeiValInsert := range insertImeiReqList {
			insertResult := logic.InsertImei(eirService.imeiRepo, imeiValInsert.Imei, imeiValInsert.Color, legacyStatus)
			if insertResult.Error != "" {
				t.Fatal("Insert imei to db failed with error: ", insertResult.Error)
			}
			//verify
			var startImeiExt string
			imeiCheckLength := utils.GetImeiCheckLength()
			if len(imeiValInsert.Imei) > imeiCheckLength {
				startImeiExt = imeiValInsert.Imei[:imeiCheckLength]
			} else {
				startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength, imeiValInsert.Imei)
			}
			VerifyImeiInDb(eirService, imeiValInsert, startImeiExt, t)
		}
	})

	//Unit test 20:
	t.Run("Eir_Add_20", func(t *testing.T) {
		t.Log("Eir_Add_20 testcase")
		eirService.ClearImeiInfo()
		insertImeiReqList := []ports.ImeiInfoInsert{
			ports.ImeiInfoInsert{
				Imei:  "12345678901234",
				Color: "w",
			},
			ports.ImeiInfoInsert{
				Imei:  "12345678901234567",
				Color: "w",
			},
		}

		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		for _, imeiValInsert := range insertImeiReqList {
			insertResult := logic.InsertImei(eirService.imeiRepo, imeiValInsert.Imei, imeiValInsert.Color, legacyStatus)
			if insertResult.Error != "" && insertResult.Error != "invalid_length" {
				t.Fatal("Insert imei to db failed with error: ", insertResult.Error)
			}
			//verify
			var startImeiExt string
			imeiCheckLength := utils.GetImeiCheckLength()
			if len(imeiValInsert.Imei) > imeiCheckLength {
				startImeiExt = imeiValInsert.Imei[:imeiCheckLength]
			} else {
				startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength, imeiValInsert.Imei)
			}
			VerifyImeiInDb(eirService, imeiValInsert, startImeiExt, t)
		}
	})

	//Unit test 21:
	t.Run("Eir_Add_21", func(t *testing.T) {
		t.Log("Eir_Add_21 testcase")
		eirService.ClearImeiInfo()
		insertReq := ports.ImeiInfoInsert{
			Imei:  "1234567890",
			Color: "w",
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		insertResult := logic.InsertImei(eirService.imeiRepo, insertReq.Imei, insertReq.Color, legacyStatus)
		if insertResult.Error != "" {
			t.Fatal("Insert imei to db failed with error: ", insertResult.Error)
		}
		var startImeiExt string
		imeiCheckLength := utils.GetImeiCheckLength()
		if len(insertReq.Imei) > imeiCheckLength {
			startImeiExt = insertReq.Imei[:imeiCheckLength]
		} else {
			startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength, insertReq.Imei)
		}
		VerifyImeiInDb(eirService, insertReq, startImeiExt, t)

		insertReqColorInvalid1 := ports.ImeiInfoInsert{
			Imei:  "1234567890",
			Color: "",
		}
		insertResult1 := logic.InsertImei(eirService.imeiRepo, insertReqColorInvalid1.Imei, insertReqColorInvalid1.Color, legacyStatus)

		insertReqColorInvalid2 := ports.ImeiInfoInsert{
			Imei:  "1234567890",
			Color: "",
		}
		insertResult2 := logic.InsertImei(eirService.imeiRepo, insertReqColorInvalid2.Imei, insertReqColorInvalid2.Color, legacyStatus)
		if insertResult1.Error == "invalid_color" && insertResult2.Error == "invalid_color" {
			t.Skip("PASS with error: ", insertResult1.Error)
		}
		t.Fatal("FAIL")
	})
	//todo: need check
	t.Run("Eir_Add_22", func(t *testing.T) {
		t.Log("Eir_Add_22 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		insertImeiReqList := []ports.ImeiInfoInsert{
			ports.ImeiInfoInsert{
				Imei:  "123456789012aA@",
				Color: "b",
			},
			ports.ImeiInfoInsert{
				Imei:  "123456 789012345",
				Color: "b",
			},
			ports.ImeiInfoInsert{
				Imei:  "123456 7890123456",
				Color: "b",
			},
		}

		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		for _, imeiValInsert := range insertImeiReqList {
			insertResult := logic.InsertImei(eirService.imeiRepo, imeiValInsert.Imei, imeiValInsert.Color, legacyStatus)
			if insertResult.Error != "invalid_parameter" {
				t.Fatal("FAIL with err: ", insertResult.Error)
			}
		}
	})

	t.Run("Eir_Add_29", func(t *testing.T) {
		t.Log("Eir_Add_29 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		// insert success
		insertImeiReqList := []ports.ImeiInfoInsert{
			ports.ImeiInfoInsert{
				Imei:  "12345678901234",
				Color: "w",
			},
			ports.ImeiInfoInsert{
				Imei:  "1234567890123456",
				Color: "w",
			},
		}
		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		for _, imeiValInsert := range insertImeiReqList {
			t.Log("Start insert with imei: ", imeiValInsert)
			insertResult := logic.InsertImei(eirService.imeiRepo, imeiValInsert.Imei, imeiValInsert.Color, legacyStatus)
			if insertResult.Error != "" && insertResult.Error != "invalid_length" {
				t.Fatal("Insert imei to db failed with error: ", insertResult.Error)
			}
			//verify
			var startImeiExt string
			imeiCheckLength := utils.GetImeiCheckLength()
			if len(imeiValInsert.Imei) > imeiCheckLength {
				startImeiExt = imeiValInsert.Imei[:imeiCheckLength]
			} else {
				startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength, imeiValInsert.Imei)
			}
			VerifyImeiInDb(eirService, imeiValInsert, startImeiExt, t)
		}

		// insert fail
		insertImeiReqListFail := []ports.ImeiInfoInsert{
			ports.ImeiInfoInsert{
				Imei:  "12345678901234",
				Color: "w",
			},
			ports.ImeiInfoInsert{
				Imei:  "12345678901234",
				Color: "b",
			},
			ports.ImeiInfoInsert{
				Imei:  "123456789012345",
				Color: "g",
			},
		}
		for _, imeiValInsert := range insertImeiReqListFail {
			t.Log("Start insert with imei: ", imeiValInsert)
			insertResult2 := logic.InsertImei(eirService.imeiRepo, imeiValInsert.Imei, imeiValInsert.Color, legacyStatus)
			if (insertResult2.Error == "imei_exist") || (insertResult2.Error == "invalid_color") {
				t.Skip("PASS with err: ", insertResult2.Error)
			}

			t.Fatal("FAIL with unexpected output: ", insertResult2.Error, "expected: imei_exist, can not insert")
		}
	})

	//turn off db -> handle later
	t.Run("Eir_Add_30", func(t *testing.T) {
		t.Log("Eir_Add_30 testcase")
		eirService.ClearImeiInfo()
	})

	//Unit test 31:
	t.Run("Eir_Add_31", func(t *testing.T) {
		t.Log("Eir_Add_31 testcase")
		eirService.ClearImeiInfo()
		insertImeiReqList := []ports.ImeiInfoInsert{
			ports.ImeiInfoInsert{
				Imei:  "9",
				Color: "w",
			},
			ports.ImeiInfoInsert{
				Imei:  "3234567890123456",
				Color: "b",
			},
			ports.ImeiInfoInsert{
				Imei:  "223456789012345",
				Color: "g",
			},
			ports.ImeiInfoInsert{
				Imei:  "5",
				Color: "w",
			},
			ports.ImeiInfoInsert{
				Imei:  "42345678901234",
				Color: "g",
			},
			ports.ImeiInfoInsert{
				Imei:  "1234567890123456",
				Color: "b",
			},
			ports.ImeiInfoInsert{
				Imei:  "22345678901234",
				Color: "g",
			},
			ports.ImeiInfoInsert{
				Imei:  "1",
				Color: "b",
			},
		}

		legacyStatus := legacyModels.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		for _, imeiValInsert := range insertImeiReqList {
			insertResult := logic.InsertImei(eirService.imeiRepo, imeiValInsert.Imei, imeiValInsert.Color, legacyStatus)
			if insertResult.Error != "" && insertResult.Error != "invalid_length" {
				t.Fatal("Insert imei to db failed with error: ", insertResult.Error)
			}
			//verify
			var startImeiExt string
			imeiCheckLength := utils.GetImeiCheckLength()
			if len(imeiValInsert.Imei) > imeiCheckLength {
				startImeiExt = imeiValInsert.Imei[:imeiCheckLength]
			} else {
				startImeiExt = fmt.Sprintf("%-*s", imeiCheckLength, imeiValInsert.Imei)
			}
			VerifyImeiInDb(eirService, imeiValInsert, startImeiExt, t)
		}
	})

	//todo: TAC_INFO
	//Unit test 63: need add verify function
	t.Run("Eir_Add_63", func(t *testing.T) {
		eirService.ClearTacInfo()
		tacList := []legacyModels.TacInfo{
			{
				KeyTac:        "1134567890123456-1134567890123456",
				StartRangeTac: "1134567890123456",
				EndRangeTac:   "1134567890123456",
				Color:         "white",
			},
			{
				KeyTac:        "2-2",
				StartRangeTac: "2",
				EndRangeTac:   "2",
				Color:         "white",
			},
		}
		for _, tac := range tacList {
			insertResult := logic.InsertTac(eirService.imeiRepo, tac)
			if insertResult.Error != "" {
				t.Fatal("Insert tac to db failed with error: ", insertResult.Error)
			}
			t.Log("insert tac success with : ", insertResult.Status)
		}
	})

	//Unit test 64: need add verify function
	t.Run("Eir_Add_64", func(t *testing.T) {
		eirService.ClearTacInfo()
		tacList := []legacyModels.TacInfo{
			{
				KeyTac:        "111-1222", //todo: why need ?
				StartRangeTac: "11",
				EndRangeTac:   "1222",
				Color:         "white",
			},
			{
				KeyTac:        "1223-13",
				StartRangeTac: "1223",
				EndRangeTac:   "13",
				Color:         "white",
			},
			{
				KeyTac:        "123456789012345-123456789012349",
				StartRangeTac: "123456789012345",
				EndRangeTac:   "123456789012349",
				Color:         "white",
			},
			{
				KeyTac:        "1-9",
				StartRangeTac: "1",
				EndRangeTac:   "9",
				Color:         "white",
			},
			//{
			//	KeyTac:        "4-4234567890123456",
			//	StartRangeTac: "1",
			//	EndRangeTac:   "9",
			//	Color:         "white",
			//},
			//{
			//	KeyTac:        "1234567890123456-1234567890123457",
			//	StartRangeTac: "1234567890123456",
			//	EndRangeTac:   "1234567890123457",
			//	Color:         "white",
			//},
		}
		for _, tac := range tacList {
			insertResult := logic.InsertTac(eirService.imeiRepo, tac)
			if insertResult.Error != "" {
				t.Fatal("Insert tac to db failed with error: ", insertResult.Error)
			}
			t.Log("insert tac success with : ", insertResult.Status)
		}
	})
}

func VerifyImeiInDb(eirService *mockEIRService, insertReq ports.ImeiInfoInsert, startImeiExt string, t *testing.T) {
	//Verify IMEI inserted
	imeiInfo, found := eirService.imeiRepo.LookupImeiInfo(context.Background(), startImeiExt)
	if !found {
		t.Fatalf("Inserted IMEI not found in EIR service")
	}
	if imeiInfo.StartIMEI != startImeiExt {
		t.Fatalf("Inserted IMEI start range mismatch: got %s, want %s", imeiInfo.StartIMEI, startImeiExt)
	}
	// if imeiInfo.EndIMEI != endImeiExt {

	// }
	if imeiInfo.Color != insertReq.Color {
		t.Fatalf("Inserted IMEI color mismatch: got %s, want %s", imeiInfo.Color, "w")
	}
	t.Logf("Verified success inserted IMEI: %s with color: %s", imeiInfo.StartIMEI, imeiInfo.Color)
}

// check imei-tac testcases
func TestEirCheckImeiAndTac(t *testing.T) {
	t.Log("Get IMEI TAC Info")

	//create object to connect db
	eirService, cleanup := createEirService()
	defer cleanup()
	log := logger.New("eir", "info")
	server := NewServer(ServerConfig{
		ListenAddr: "127.0.0.1:8080", // ip loopback
	}, eirService, &mockStatsCollector{}, log)

	//create EIR server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start EIR server: %v", err)
	}
	defer server.Stop()
	time.Sleep(200 * time.Millisecond)
	t.Logf("Created EIR server: %v", server.GetAddr())

	// Unit test 01: EIR_01 Insert IMEI valid
	t.Run("EIR_01", func(t *testing.T) {
		t.Log("EIR_01 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB

		imsi := "452040000000001"
		status := models.SystemStatus{
			OverloadLevel: 0,
			TPSOverload:   false,
		}
		checkResult, err := eirService.CheckImei(nil, imsi, status)
		if checkResult.Status != "ok" {
			t.Fatalf("CheckImei error: got %s, want %s", checkResult.Status, "ok")
		}
		if err != nil {
			t.Fatal("CheckImei error: ", err)
		}
		t.Log("CheckImei success: ", checkResult)
	})
}
