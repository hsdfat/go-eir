package postgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	return sqlxDB, mock
}

func TestNewIMEIRepository(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	assert.NotNil(t, repo)
}

func TestGetByIMEI_Success(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()

	imei := "490154203237518"
	imeisv := "4901542032375189"
	reason := "Test equipment"
	metadata := `{"test": "data"}`
	tac := "49015420"
	manufacturer := "TestCorp"
	lastCheckTime := time.Now()

	expectedEquipment := &models.Equipment{
		ID:               1,
		IMEI:             imei,
		IMEISV:           &imeisv,
		Status:           models.EquipmentStatusWhitelisted,
		Reason:           &reason,
		LastUpdated:      time.Now(),
		LastCheckTime:    &lastCheckTime,
		CheckCount:       5,
		AddedBy:          "admin",
		Metadata:         &metadata,
		ManufacturerTAC:  &tac,
		ManufacturerName: &manufacturer,
	}

	rows := sqlmock.NewRows([]string{
		"id", "imei", "imeisv", "status", "reason", "last_updated", "last_check_time",
		"check_count", "added_by", "metadata", "manufacturer_tac", "manufacturer_name",
	}).AddRow(
		expectedEquipment.ID,
		expectedEquipment.IMEI,
		expectedEquipment.IMEISV,
		expectedEquipment.Status,
		expectedEquipment.Reason,
		expectedEquipment.LastUpdated,
		expectedEquipment.LastCheckTime,
		expectedEquipment.CheckCount,
		expectedEquipment.AddedBy,
		expectedEquipment.Metadata,
		expectedEquipment.ManufacturerTAC,
		expectedEquipment.ManufacturerName,
	)

	mock.ExpectQuery("SELECT (.+) FROM equipment WHERE imei = (.+)").
		WithArgs(imei).
		WillReturnRows(rows)

	result, err := repo.GetByIMEI(ctx, imei)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedEquipment.ID, result.ID)
	assert.Equal(t, expectedEquipment.IMEI, result.IMEI)
	assert.Equal(t, expectedEquipment.Status, result.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByIMEI_NotFound(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()
	imei := "999999999999999"

	mock.ExpectQuery("SELECT (.+) FROM equipment WHERE imei = (.+)").
		WithArgs(imei).
		WillReturnError(sql.ErrNoRows)

	result, err := repo.GetByIMEI(ctx, imei)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, ErrNotFound))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByIMEI_DatabaseError(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()
	imei := "490154203237518"

	mock.ExpectQuery("SELECT (.+) FROM equipment WHERE imei = (.+)").
		WithArgs(imei).
		WillReturnError(errors.New("database connection failed"))

	result, err := repo.GetByIMEI(ctx, imei)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get equipment")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByIMEISV_Success(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()

	imei := "490154203237518"
	imeisv := "4901542032375189"
	reason := "Test equipment"

	rows := sqlmock.NewRows([]string{
		"id", "imei", "imeisv", "status", "reason", "last_updated", "last_check_time",
		"check_count", "added_by", "metadata", "manufacturer_tac", "manufacturer_name",
	}).AddRow(
		1, imei, &imeisv, models.EquipmentStatusWhitelisted, &reason,
		time.Now(), nil, 0, "admin", nil, nil, nil,
	)

	mock.ExpectQuery("SELECT (.+) FROM equipment WHERE imeisv = (.+)").
		WithArgs(imeisv).
		WillReturnRows(rows)

	result, err := repo.GetByIMEISV(ctx, imeisv)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, imei, result.IMEI)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByIMEISV_NotFound(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()
	imeisv := "9999999999999999"

	mock.ExpectQuery("SELECT (.+) FROM equipment WHERE imeisv = (.+)").
		WithArgs(imeisv).
		WillReturnError(sql.ErrNoRows)

	result, err := repo.GetByIMEISV(ctx, imeisv)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, ErrNotFound))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreate_Success(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()

	equipment := &models.Equipment{
		IMEI:        "490154203237518",
		Status:      models.EquipmentStatusWhitelisted,
		LastUpdated: time.Now(),
		CheckCount:  0,
		AddedBy:     "admin",
	}

	mock.ExpectPrepare("INSERT INTO equipment").
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	err := repo.Create(ctx, equipment)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), equipment.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreate_Error(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()

	equipment := &models.Equipment{
		IMEI:        "490154203237518",
		Status:      models.EquipmentStatusWhitelisted,
		LastUpdated: time.Now(),
		AddedBy:     "admin",
	}

	mock.ExpectPrepare("INSERT INTO equipment").
		ExpectQuery().
		WillReturnError(errors.New("duplicate key violation"))

	err := repo.Create(ctx, equipment)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create equipment")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_Success(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()

	equipment := &models.Equipment{
		IMEI:        "490154203237518",
		Status:      models.EquipmentStatusBlacklisted,
		LastUpdated: time.Now(),
	}

	mock.ExpectExec("UPDATE equipment").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Update(ctx, equipment)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_NotFound(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()

	equipment := &models.Equipment{
		IMEI:        "999999999999999",
		Status:      models.EquipmentStatusBlacklisted,
		LastUpdated: time.Now(),
	}

	mock.ExpectExec("UPDATE equipment").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Update(ctx, equipment)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_DatabaseError(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()

	equipment := &models.Equipment{
		IMEI:        "490154203237518",
		Status:      models.EquipmentStatusBlacklisted,
		LastUpdated: time.Now(),
	}

	mock.ExpectExec("UPDATE equipment").
		WillReturnError(errors.New("database error"))

	err := repo.Update(ctx, equipment)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update equipment")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDelete_Success(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()
	imei := "490154203237518"

	mock.ExpectExec("DELETE FROM equipment WHERE imei = (.+)").
		WithArgs(imei).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Delete(ctx, imei)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDelete_NotFound(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()
	imei := "999999999999999"

	mock.ExpectExec("DELETE FROM equipment WHERE imei = (.+)").
		WithArgs(imei).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Delete(ctx, imei)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestList_Success(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{
		"id", "imei", "imeisv", "status", "reason", "last_updated", "last_check_time",
		"check_count", "added_by", "metadata", "manufacturer_tac", "manufacturer_name",
	}).
		AddRow(1, "490154203237518", nil, models.EquipmentStatusWhitelisted, nil, time.Now(), nil, 0, "admin", nil, nil, nil).
		AddRow(2, "490154203237519", nil, models.EquipmentStatusBlacklisted, nil, time.Now(), nil, 0, "admin", nil, nil, nil)

	mock.ExpectQuery("SELECT (.+) FROM equipment ORDER BY last_updated DESC LIMIT (.+) OFFSET (.+)").
		WithArgs(10, 0).
		WillReturnRows(rows)

	result, err := repo.List(ctx, 0, 10)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestList_Empty(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{
		"id", "imei", "imeisv", "status", "reason", "last_updated", "last_check_time",
		"check_count", "added_by", "metadata", "manufacturer_tac", "manufacturer_name",
	})

	mock.ExpectQuery("SELECT (.+) FROM equipment ORDER BY last_updated DESC LIMIT (.+) OFFSET (.+)").
		WithArgs(10, 0).
		WillReturnRows(rows)

	result, err := repo.List(ctx, 0, 10)

	assert.NoError(t, err)
	assert.Empty(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListByStatus_Success(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()
	status := models.EquipmentStatusBlacklisted

	rows := sqlmock.NewRows([]string{
		"id", "imei", "imeisv", "status", "reason", "last_updated", "last_check_time",
		"check_count", "added_by", "metadata", "manufacturer_tac", "manufacturer_name",
	}).
		AddRow(1, "490154203237518", nil, status, nil, time.Now(), nil, 0, "admin", nil, nil, nil).
		AddRow(2, "490154203237519", nil, status, nil, time.Now(), nil, 0, "admin", nil, nil, nil)

	mock.ExpectQuery("SELECT (.+) FROM equipment WHERE status = (.+) ORDER BY last_updated DESC LIMIT (.+) OFFSET (.+)").
		WithArgs(status, 10, 0).
		WillReturnRows(rows)

	result, err := repo.ListByStatus(ctx, status, 0, 10)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, status, result[0].Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListByStatus_NoResults(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()
	status := models.EquipmentStatusGreylisted

	rows := sqlmock.NewRows([]string{
		"id", "imei", "imeisv", "status", "reason", "last_updated", "last_check_time",
		"check_count", "added_by", "metadata", "manufacturer_tac", "manufacturer_name",
	})

	mock.ExpectQuery("SELECT (.+) FROM equipment WHERE status = (.+) ORDER BY last_updated DESC LIMIT (.+) OFFSET (.+)").
		WithArgs(status, 10, 0).
		WillReturnRows(rows)

	result, err := repo.ListByStatus(ctx, status, 0, 10)

	assert.NoError(t, err)
	assert.Empty(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIncrementCheckCount_Success(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()
	imei := "490154203237518"

	mock.ExpectExec("SELECT increment_equipment_check_count").
		WithArgs(imei).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.IncrementCheckCount(ctx, imei)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIncrementCheckCount_Error(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()

	repo := NewIMEIRepository(db)
	ctx := context.Background()
	imei := "490154203237518"

	mock.ExpectExec("SELECT increment_equipment_check_count").
		WithArgs(imei).
		WillReturnError(errors.New("function not found"))

	err := repo.IncrementCheckCount(ctx, imei)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to increment check count")
	assert.NoError(t, mock.ExpectationsWereMet())
}
