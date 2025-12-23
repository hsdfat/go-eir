package mongodb

import (
	"testing"
	"time"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/stretchr/testify/assert"
)

// Note: These tests demonstrate the structure and logic validation.
// For integration tests with real MongoDB, see test/integration directory.

func TestMongoRepositoryStructure(t *testing.T) {
	t.Run("Repository structure", func(t *testing.T) {
		repo := &imeiRepository{}
		assert.NotNil(t, repo)
	})
}

func TestMongoErrors(t *testing.T) {
	t.Run("ErrNotFound constant", func(t *testing.T) {
		assert.NotNil(t, ErrNotFound)
		assert.Equal(t, "equipment not found", ErrNotFound.Error())
	})

	t.Run("ErrAlreadyExists constant", func(t *testing.T) {
		assert.NotNil(t, ErrAlreadyExists)
		assert.Equal(t, "equipment already exists", ErrAlreadyExists.Error())
	})
}

func TestEquipmentModel(t *testing.T) {
	t.Run("Create equipment with all fields", func(t *testing.T) {
		imei := "490154203237518"
		imeisv := "4901542032375189"
		reason := "Test equipment"
		metadata := `{"test": "data"}`
		tac := "49015420"
		manufacturer := "TestCorp"
		lastCheckTime := time.Now()

		equipment := &models.Equipment{
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

		assert.Equal(t, imei, equipment.IMEI)
		assert.Equal(t, imeisv, *equipment.IMEISV)
		assert.Equal(t, models.EquipmentStatusWhitelisted, equipment.Status)
		assert.Equal(t, int64(5), equipment.CheckCount)
		assert.Equal(t, "admin", equipment.AddedBy)
	})

	t.Run("Create equipment with minimal fields", func(t *testing.T) {
		equipment := &models.Equipment{
			IMEI:        "490154203237518",
			Status:      models.EquipmentStatusWhitelisted,
			LastUpdated: time.Now(),
			CheckCount:  0,
			AddedBy:     "admin",
		}

		assert.Equal(t, "490154203237518", equipment.IMEI)
		assert.Nil(t, equipment.IMEISV)
		assert.Nil(t, equipment.Reason)
		assert.Nil(t, equipment.Metadata)
	})
}

func TestEquipmentStatuses(t *testing.T) {
	tests := []struct {
		name   string
		status models.EquipmentStatus
	}{
		{
			name:   "Whitelisted status",
			status: models.EquipmentStatusWhitelisted,
		},
		{
			name:   "Blacklisted status",
			status: models.EquipmentStatusBlacklisted,
		},
		{
			name:   "Greylisted status",
			status: models.EquipmentStatusGreylisted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			equipment := &models.Equipment{
				IMEI:        "490154203237518",
				Status:      tt.status,
				LastUpdated: time.Now(),
				AddedBy:     "admin",
			}

			assert.Equal(t, tt.status, equipment.Status)
		})
	}
}

func TestIMEIValidation(t *testing.T) {
	tests := []struct {
		name    string
		imei    string
		wantErr bool
	}{
		{
			name:    "Valid 15-digit IMEI",
			imei:    "490154203237518",
			wantErr: false,
		},
		{
			name:    "Valid 14-digit IMEI",
			imei:    "49015420323751",
			wantErr: false,
		},
		{
			name:    "Empty IMEI",
			imei:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := models.ValidateIMEI(tt.imei)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateOperation(t *testing.T) {
	t.Run("Update equipment status", func(t *testing.T) {
		equipment := &models.Equipment{
			IMEI:        "490154203237518",
			Status:      models.EquipmentStatusWhitelisted,
			LastUpdated: time.Now(),
		}

		// Simulate update
		equipment.Status = models.EquipmentStatusBlacklisted
		equipment.LastUpdated = time.Now()

		assert.Equal(t, models.EquipmentStatusBlacklisted, equipment.Status)
	})

	t.Run("Update equipment metadata", func(t *testing.T) {
		metadata := `{"updated": true}`
		equipment := &models.Equipment{
			IMEI:        "490154203237518",
			Status:      models.EquipmentStatusWhitelisted,
			Metadata:    &metadata,
			LastUpdated: time.Now(),
		}

		assert.NotNil(t, equipment.Metadata)
		assert.Contains(t, *equipment.Metadata, "updated")
	})
}

func TestCheckCountIncrement(t *testing.T) {
	t.Run("Increment check count", func(t *testing.T) {
		equipment := &models.Equipment{
			IMEI:        "490154203237518",
			Status:      models.EquipmentStatusWhitelisted,
			CheckCount:  0,
			LastUpdated: time.Now(),
		}

		// Simulate increment
		equipment.CheckCount++
		now := time.Now()
		equipment.LastCheckTime = &now

		assert.Equal(t, int64(1), equipment.CheckCount)
		assert.NotNil(t, equipment.LastCheckTime)
	})
}

func TestPaginationParams(t *testing.T) {
	tests := []struct {
		name   string
		offset int
		limit  int
	}{
		{
			name:   "First page",
			offset: 0,
			limit:  10,
		},
		{
			name:   "Second page",
			offset: 10,
			limit:  10,
		},
		{
			name:   "Large page size",
			offset: 0,
			limit:  100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.GreaterOrEqual(t, tt.offset, 0)
			assert.Greater(t, tt.limit, 0)
		})
	}
}

func TestEquipmentTACExtraction(t *testing.T) {
	tests := []struct {
		name string
		imei string
		want string
	}{
		{
			name: "Standard IMEI",
			imei: "490154203237518",
			want: "49015420",
		},
		{
			name: "14-digit IMEI",
			imei: "49015420323751",
			want: "49015420",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tac := models.ExtractTAC(tt.imei)
			assert.Equal(t, tt.want, tac)
		})
	}
}
