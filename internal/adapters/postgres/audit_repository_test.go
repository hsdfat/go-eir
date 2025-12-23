package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/stretchr/testify/assert"
)

func TestNewAuditRepository(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)
	assert.NotNil(t, repo)
}

func TestLogCheck_Success(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)
	ctx := context.Background()

	imei := "490154203237518"
	imeisv := "4901542032375189"
	originHost := "mme.example.com"
	originRealm := "example.com"
	userName := "user123"
	supi := "imsi-123456789012345"
	gpsi := "msisdn-1234567890"
	sessionID := "session-abc123"
	resultCode := int32(2001)

	audit := &models.AuditLog{
		IMEI:          imei,
		IMEISV:        &imeisv,
		Status:        models.EquipmentStatusWhitelisted,
		CheckTime:     time.Now(),
		OriginHost:    &originHost,
		OriginRealm:   &originRealm,
		UserName:      &userName,
		SUPI:          &supi,
		GPSI:          &gpsi,
		RequestSource: "DIAMETER_S13",
		SessionID:     &sessionID,
		ResultCode:    &resultCode,
	}

	mock.ExpectPrepare("INSERT INTO audit_log").
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	err := repo.LogCheck(ctx, audit)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), audit.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogCheck_PrepareError(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)
	ctx := context.Background()

	audit := &models.AuditLog{
		IMEI:          "490154203237518",
		Status:        models.EquipmentStatusWhitelisted,
		CheckTime:     time.Now(),
		RequestSource: "DIAMETER_S13",
	}

	mock.ExpectPrepare("INSERT INTO audit_log").
		WillReturnError(errors.New("prepare statement failed"))

	err := repo.LogCheck(ctx, audit)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to prepare statement")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogCheck_InsertError(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)
	ctx := context.Background()

	audit := &models.AuditLog{
		IMEI:          "490154203237518",
		Status:        models.EquipmentStatusWhitelisted,
		CheckTime:     time.Now(),
		RequestSource: "DIAMETER_S13",
	}

	mock.ExpectPrepare("INSERT INTO audit_log").
		ExpectQuery().
		WillReturnError(errors.New("insert failed"))

	err := repo.LogCheck(ctx, audit)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to log check")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAuditsByIMEI_Success(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)
	ctx := context.Background()
	imei := "490154203237518"

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "imei", "imeisv", "status", "check_time", "origin_host", "origin_realm",
		"user_name", "supi", "gpsi", "request_source", "session_id", "result_code",
	}).
		AddRow(1, imei, nil, models.EquipmentStatusWhitelisted, now, nil, nil, nil, nil, nil, "DIAMETER_S13", nil, nil).
		AddRow(2, imei, nil, models.EquipmentStatusWhitelisted, now.Add(-1*time.Hour), nil, nil, nil, nil, nil, "HTTP_5G", nil, nil)

	mock.ExpectQuery("SELECT (.+) FROM audit_log WHERE imei = (.+) ORDER BY check_time DESC LIMIT (.+) OFFSET (.+)").
		WithArgs(imei, 10, 0).
		WillReturnRows(rows)

	result, err := repo.GetAuditsByIMEI(ctx, imei, 0, 10)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, imei, result[0].IMEI)
	assert.Equal(t, "DIAMETER_S13", result[0].RequestSource)
	assert.Equal(t, "HTTP_5G", result[1].RequestSource)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAuditsByIMEI_NoResults(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)
	ctx := context.Background()
	imei := "999999999999999"

	rows := sqlmock.NewRows([]string{
		"id", "imei", "imeisv", "status", "check_time", "origin_host", "origin_realm",
		"user_name", "supi", "gpsi", "request_source", "session_id", "result_code",
	})

	mock.ExpectQuery("SELECT (.+) FROM audit_log WHERE imei = (.+) ORDER BY check_time DESC LIMIT (.+) OFFSET (.+)").
		WithArgs(imei, 10, 0).
		WillReturnRows(rows)

	result, err := repo.GetAuditsByIMEI(ctx, imei, 0, 10)

	assert.NoError(t, err)
	assert.Empty(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAuditsByIMEI_Error(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)
	ctx := context.Background()
	imei := "490154203237518"

	mock.ExpectQuery("SELECT (.+) FROM audit_log WHERE imei = (.+) ORDER BY check_time DESC LIMIT (.+) OFFSET (.+)").
		WithArgs(imei, 10, 0).
		WillReturnError(errors.New("database error"))

	result, err := repo.GetAuditsByIMEI(ctx, imei, 0, 10)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get audits by IMEI")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAuditsByTimeRange_Success(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)
	ctx := context.Background()
	startTime := "2024-01-01T00:00:00Z"
	endTime := "2024-01-31T23:59:59Z"

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "imei", "imeisv", "status", "check_time", "origin_host", "origin_realm",
		"user_name", "supi", "gpsi", "request_source", "session_id", "result_code",
	}).
		AddRow(1, "490154203237518", nil, models.EquipmentStatusWhitelisted, now, nil, nil, nil, nil, nil, "DIAMETER_S13", nil, nil).
		AddRow(2, "490154203237519", nil, models.EquipmentStatusBlacklisted, now.Add(-1*time.Hour), nil, nil, nil, nil, nil, "HTTP_5G", nil, nil)

	mock.ExpectQuery("SELECT (.+) FROM audit_log WHERE check_time >= (.+) AND check_time <= (.+) ORDER BY check_time DESC LIMIT (.+) OFFSET (.+)").
		WithArgs(startTime, endTime, 10, 0).
		WillReturnRows(rows)

	result, err := repo.GetAuditsByTimeRange(ctx, startTime, endTime, 0, 10)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAuditsByTimeRange_NoResults(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)
	ctx := context.Background()
	startTime := "2025-01-01T00:00:00Z"
	endTime := "2025-01-01T01:00:00Z"

	rows := sqlmock.NewRows([]string{
		"id", "imei", "imeisv", "status", "check_time", "origin_host", "origin_realm",
		"user_name", "supi", "gpsi", "request_source", "session_id", "result_code",
	})

	mock.ExpectQuery("SELECT (.+) FROM audit_log WHERE check_time >= (.+) AND check_time <= (.+) ORDER BY check_time DESC LIMIT (.+) OFFSET (.+)").
		WithArgs(startTime, endTime, 10, 0).
		WillReturnRows(rows)

	result, err := repo.GetAuditsByTimeRange(ctx, startTime, endTime, 0, 10)

	assert.NoError(t, err)
	assert.Empty(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAuditsByTimeRange_Error(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)
	ctx := context.Background()
	startTime := "2024-01-01T00:00:00Z"
	endTime := "2024-01-31T23:59:59Z"

	mock.ExpectQuery("SELECT (.+) FROM audit_log WHERE check_time >= (.+) AND check_time <= (.+) ORDER BY check_time DESC LIMIT (.+) OFFSET (.+)").
		WithArgs(startTime, endTime, 10, 0).
		WillReturnError(errors.New("database error"))

	result, err := repo.GetAuditsByTimeRange(ctx, startTime, endTime, 0, 10)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get audits by time range")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAuditsByTimeRange_WithPagination(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)
	ctx := context.Background()
	startTime := "2024-01-01T00:00:00Z"
	endTime := "2024-01-31T23:59:59Z"
	offset := 20
	limit := 5

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "imei", "imeisv", "status", "check_time", "origin_host", "origin_realm",
		"user_name", "supi", "gpsi", "request_source", "session_id", "result_code",
	}).
		AddRow(21, "490154203237520", nil, models.EquipmentStatusGreylisted, now, nil, nil, nil, nil, nil, "DIAMETER_S13", nil, nil)

	mock.ExpectQuery("SELECT (.+) FROM audit_log WHERE check_time >= (.+) AND check_time <= (.+) ORDER BY check_time DESC LIMIT (.+) OFFSET (.+)").
		WithArgs(startTime, endTime, limit, offset).
		WillReturnRows(rows)

	result, err := repo.GetAuditsByTimeRange(ctx, startTime, endTime, offset, limit)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(21), result[0].ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditLog_AllFields(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)
	ctx := context.Background()

	imei := "490154203237518"
	imeisv := "4901542032375189"
	originHost := "mme.example.com"
	originRealm := "example.com"
	userName := "user123"
	supi := "imsi-123456789012345"
	gpsi := "msisdn-1234567890"
	sessionID := "session-abc123"
	resultCode := int32(2001)

	audit := &models.AuditLog{
		IMEI:          imei,
		IMEISV:        &imeisv,
		Status:        models.EquipmentStatusBlacklisted,
		CheckTime:     time.Now(),
		OriginHost:    &originHost,
		OriginRealm:   &originRealm,
		UserName:      &userName,
		SUPI:          &supi,
		GPSI:          &gpsi,
		RequestSource: "HTTP_5G",
		SessionID:     &sessionID,
		ResultCode:    &resultCode,
	}

	mock.ExpectPrepare("INSERT INTO audit_log").
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	err := repo.LogCheck(ctx, audit)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), audit.ID)
	assert.Equal(t, imei, audit.IMEI)
	assert.Equal(t, imeisv, *audit.IMEISV)
	assert.Equal(t, models.EquipmentStatusBlacklisted, audit.Status)
	assert.Equal(t, originHost, *audit.OriginHost)
	assert.Equal(t, "HTTP_5G", audit.RequestSource)
	assert.NoError(t, mock.ExpectationsWereMet())
}
