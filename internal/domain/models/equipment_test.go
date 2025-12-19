package models

import (
	"testing"
)

func TestValidateIMEI(t *testing.T) {
	tests := []struct {
		name    string
		imei    string
		wantErr bool
		errType error
	}{
		{
			name:    "Valid 15-digit IMEI with correct Luhn",
			imei:    "490154203237518",
			wantErr: false,
		},
		{
			name:    "Valid 14-digit IMEI",
			imei:    "49015420323751",
			wantErr: false,
		},
		{
			name:    "Valid 16-digit IMEISV",
			imei:    "4901542032375189",
			wantErr: false,
		},
		{
			name:    "Invalid - empty",
			imei:    "",
			wantErr: true,
			errType: ErrInvalidIMEI,
		},
		{
			name:    "Invalid - too short",
			imei:    "1234567890123",
			wantErr: true,
			errType: ErrIMEITooShort,
		},
		{
			name:    "Invalid - too long",
			imei:    "12345678901234567",
			wantErr: true,
			errType: ErrIMEITooLong,
		},
		{
			name:    "Invalid - non-numeric",
			imei:    "12345678901234A",
			wantErr: true,
			errType: ErrIMEINotNumeric,
		},
		{
			name:    "Invalid - failed Luhn check",
			imei:    "490154203237519",
			wantErr: true,
			errType: ErrInvalidLuhnCheck,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIMEI(tt.imei)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIMEI() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errType != nil && err != tt.errType {
				t.Errorf("ValidateIMEI() error = %v, want %v", err, tt.errType)
			}
		})
	}
}

func TestExtractTAC(t *testing.T) {
	tests := []struct {
		name string
		imei string
		want string
	}{
		{
			name: "Standard 15-digit IMEI",
			imei: "490154203237518",
			want: "49015420",
		},
		{
			name: "14-digit IMEI",
			imei: "49015420323751",
			want: "49015420",
		},
		{
			name: "Short IMEI (less than 8 digits)",
			imei: "1234567",
			want: "",
		},
		{
			name: "Empty IMEI",
			imei: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTAC(tt.imei)
			if got != tt.want {
				t.Errorf("ExtractTAC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  EquipmentStatus
		wantErr bool
	}{
		{
			name:    "Valid - WHITELISTED",
			status:  EquipmentStatusWhitelisted,
			wantErr: false,
		},
		{
			name:    "Valid - BLACKLISTED",
			status:  EquipmentStatusBlacklisted,
			wantErr: false,
		},
		{
			name:    "Valid - GREYLISTED",
			status:  EquipmentStatusGreylisted,
			wantErr: false,
		},
		{
			name:    "Invalid status",
			status:  EquipmentStatus("INVALID"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatus(tt.status)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToDialDialStatus(t *testing.T) {
	tests := []struct {
		name   string
		status EquipmentStatus
		want   DiameterEquipmentStatus
	}{
		{
			name:   "WHITELISTED",
			status: EquipmentStatusWhitelisted,
			want:   DiameterEquipmentStatusWhitelisted,
		},
		{
			name:   "BLACKLISTED",
			status: EquipmentStatusBlacklisted,
			want:   DiameterEquipmentStatusBlacklisted,
		},
		{
			name:   "GREYLISTED",
			status: EquipmentStatusGreylisted,
			want:   DiameterEquipmentStatusGreylisted,
		},
		{
			name:   "Invalid defaults to WHITELISTED",
			status: EquipmentStatus("INVALID"),
			want:   DiameterEquipmentStatusWhitelisted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToDialDialStatus(tt.status)
			if got != tt.want {
				t.Errorf("ToDialDialStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromDiameterStatus(t *testing.T) {
	tests := []struct {
		name       string
		diamStatus DiameterEquipmentStatus
		want       EquipmentStatus
	}{
		{
			name:       "WHITELISTED",
			diamStatus: DiameterEquipmentStatusWhitelisted,
			want:       EquipmentStatusWhitelisted,
		},
		{
			name:       "BLACKLISTED",
			diamStatus: DiameterEquipmentStatusBlacklisted,
			want:       EquipmentStatusBlacklisted,
		},
		{
			name:       "GREYLISTED",
			diamStatus: DiameterEquipmentStatusGreylisted,
			want:       EquipmentStatusGreylisted,
		},
		{
			name:       "Invalid defaults to WHITELISTED",
			diamStatus: DiameterEquipmentStatus(99),
			want:       EquipmentStatusWhitelisted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromDiameterStatus(tt.diamStatus)
			if got != tt.want {
				t.Errorf("FromDiameterStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Benchmark tests
func BenchmarkValidateIMEI(b *testing.B) {
	imei := "490154203237518"
	for i := 0; i < b.N; i++ {
		ValidateIMEI(imei)
	}
}

func BenchmarkExtractTAC(b *testing.B) {
	imei := "490154203237518"
	for i := 0; i < b.N; i++ {
		ExtractTAC(imei)
	}
}
