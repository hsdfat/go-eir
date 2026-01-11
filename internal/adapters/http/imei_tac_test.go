package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/hsdfat/go-eir/internal/adapters/postgres"
	"github.com/hsdfat/go-eir/internal/domain/ports"
	"github.com/hsdfat/go-eir/internal/logger"
	"github.com/jmoiron/sqlx"
)

/*
- go test -v -run TestImei ./internal/adapters/http/
- CREATE TABLE IMEI_INFO (
     StartIMEI VARCHAR(16) PRIMARY KEY,
     EndIMEI TEXT[] DEFAULT '{}',
     Color CHAR(1) NOT NULL CHECK (Color IN ('w', 'b', 'g'))
  );
- go test -v -run "TestImei" ./internal/adapters/http/
- go test -v -run "TestImei/Insert_IMEI_valid" ./internal/adapters/http/
*/

func createEirService() (*mockEIRService, func()) {
	// Create and return an instance of EirService
	dbURL := "host=localhost port=5433 user=eir password=eir_password dbname=eir sslmode=disable"
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

func TestImei(t *testing.T) {
	t.Log("Get IMEI TAC Info")

	//create object to connect db
	eirService, cleanup := createEirService()
	defer cleanup()
	log := logger.New("eir", "info")
	server := NewServer(ServerConfig{
		ListenAddr: "127.0.0.1:8080", // ip loopback
	}, eirService, log)

	//create EIR server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start EIR server: %v", err)
	}
	defer server.Stop()
	time.Sleep(200 * time.Millisecond)
	t.Logf("Created EIR server: %v", server.GetAddr())

	//create NR client
	dialer := &net.Dialer{}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				conn, err := dialer.DialContext(ctx, network, addr)
				if err != nil {
					return nil, err
				}
				return conn, nil
			},
		},
		Timeout: 5 * time.Second,
	}

	// Unit test 01: Eir_Add_1 Insert IMEI valid
	t.Run("Eir_Add_1", func(t *testing.T) {
		t.Log("Eir_Add_1 testcase")
		eirService.ClearImeiInfo() //delete all imei_info table in DB
		//Insert IMEI with black color
		insertReq := ports.ImeiInfoInsert{
			Imei:  "1",
			Color: "b",
		}
		startImeiExt := "1             "
		InsertImei(eirService, client, server, insertReq, startImeiExt, t)
	})

	// Unit test 02:
	t.Run("Eir_Add_2", func(t *testing.T) {
		t.Log("Eir_Add_2 testcase")
		insertReq := ports.ImeiInfoInsert{
			Imei:  "123456789012345",
			Color: "w",
		}
		startImeiExt := "12345678901234"
		InsertImei(eirService, client, server, insertReq, startImeiExt, t)
	})

	// Unit test 03:
	t.Run("Eir_Add_3", func(t *testing.T) {
		t.Log("Eir_Add_3 testcase")
		insertReq := ports.ImeiInfoInsert{
			Imei:  "12345678901234",
			Color: "w",
		}
		startImeiExt := "12345678901234"
		InsertImei(eirService, client, server, insertReq, startImeiExt, t)
	})

	// Unit test 04: need check imei_max_length
	t.Run("Eir_Add_4", func(t *testing.T) {
		t.Log("Eir_Add_4 testcase")
		insertReq := ports.ImeiInfoInsert{
			Imei:  "12345678901234567",
			Color: "w",
		}
		startImeiExt := "12345678901234"
		InsertImei(eirService, client, server, insertReq, startImeiExt, t)
	})
}

func InsertImei(eirService *mockEIRService, client *http.Client, server *Server, insertReq ports.ImeiInfoInsert, startImeiExt string, t *testing.T) {
	// eirService.ClearImeiInfo() //delete all imei_info table in DB
	body, _ := json.Marshal(insertReq)
	insertURL := fmt.Sprintf("http://%s/api/v1/insert-imei", server.GetAddr())
	//NF request insert IMEI to EIR
	resp, err := client.Post(insertURL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Insert IMEI request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Insert IMEI failed with status %d", resp.StatusCode)
	}
	t.Logf("Inserted IMEI: %s with color: %s", insertReq.Imei, insertReq.Color)

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
