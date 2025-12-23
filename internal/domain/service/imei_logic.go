package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	legacyModels "github.com/hsdfat8/eir/models"
	"github.com/hsdfat8/eir/pkg/repository"
)

// ImeiLogicService encapsulates IMEI-based business logic
type ImeiLogicService struct {
	imeiCheckLength int
	imeiMaxLength   int
	imeiRepo        repository.ImeiRepository
	sampleData      map[string]*legacyModels.ImeiInfo
}

// NewImeiLogicService creates a new IMEI logic service
func NewImeiLogicService(imeiCheckLength, imeiMaxLength int, imeiRepo repository.ImeiRepository, sampleData map[string]*legacyModels.ImeiInfo) *ImeiLogicService {
	return &ImeiLogicService{
		imeiCheckLength: imeiCheckLength,
		imeiMaxLength:   imeiMaxLength,
		imeiRepo:        imeiRepo,
		sampleData:      sampleData,
	}
}

func (s *ImeiLogicService) normalizeImei(imei string) string {
	imeiLength := len(imei)
	if imeiLength > s.imeiCheckLength {
		return imei[:s.imeiCheckLength]
	} else if imeiLength == s.imeiCheckLength {
		return imei
	} else {
		return imei + strings.Repeat(" ", s.imeiCheckLength-imeiLength)
	}
}

func (s *ImeiLogicService) lookupImeiInfo(imei string) (string, error) {
	bestLen := -1
	bestColor := ""

	for _, info := range s.sampleData {
		if strings.HasPrefix(info.StartIMEI, imei) {
			if len(info.StartIMEI) > bestLen {
				bestLen = len(info.StartIMEI)
				bestColor = info.Color
			}
		}
	}
	if bestColor == "" {
		return "", fmt.Errorf("IMEI not found")
	}
	return bestColor, nil
}

func (s *ImeiLogicService) isOverLoad(status models.SystemStatus) bool {
	// Simple implementation - can be extended based on business rules
	return status.TPSOverload || status.OverloadLevel > 0
}

// CheckImei performs IMEI-based equipment check
func (s *ImeiLogicService) CheckImei(imei string, status models.SystemStatus) ports.CheckImeiResult {
	normalizedImei := s.normalizeImei(imei)

	if s.isOverLoad(status) {
		return ports.CheckImeiResult{
			Status: "error",
			IMEI:   normalizedImei,
			Color:  "overload",
		}
	}

	color, err := s.lookupImeiInfo(normalizedImei)
	if err != nil {
		return ports.CheckImeiResult{
			Status: "error",
			IMEI:   normalizedImei,
			Color:  "unknown",
		}
	}

	return ports.CheckImeiResult{
		Status: "ok",
		IMEI:   normalizedImei,
		Color:  color,
	}
}

func (s *ImeiLogicService) validateAddImei(imei string, color string) error {
	if imei == "" {
		return errors.New("invalid_parameter")
	}
	for _, character := range imei {
		if character < '0' || character > '9' {
			return errors.New("invalid_value")
		}
	}

	if len(imei) > s.imeiMaxLength {
		return errors.New("invalid_length")
	}
	switch color {
	case "b", "g", "w":
	default:
		return errors.New("invalid_color")
	}
	return nil
}

func (s *ImeiLogicService) normalizeImeiForInsert(imei string) (start string, end string) {
	if len(imei) <= s.imeiCheckLength {
		start = imei + strings.Repeat(" ", s.imeiCheckLength-len(imei))
		end = " "
		return
	}

	start = imei[:s.imeiCheckLength]
	end = imei[s.imeiCheckLength:]
	if end == "" {
		end = " "
	}
	return
}

// InsertImei inserts an IMEI into the repository
func (s *ImeiLogicService) InsertImei(imei string, color string, status models.SystemStatus) ports.InsertImeiResult {
	if s.isOverLoad(status) {
		return ports.InsertImeiResult{
			Status: "error",
			IMEI:   imei,
			Error:  strPtr("overload"),
		}
	}

	if err := s.validateAddImei(imei, color); err != nil {
		return ports.InsertImeiResult{
			Status: "error",
			IMEI:   imei,
			Error:  strPtr(err.Error()),
		}
	}

	start, end := s.normalizeImeiForInsert(imei)
	if info, ok := s.imeiRepo.Lookup(start); ok {
		if info.Color != color {
			return ports.InsertImeiResult{
				Status: "error",
				IMEI:   imei,
				Error:  strPtr("color_conflict"),
			}
		}

		for _, e := range info.EndIMEI {
			if e == end {
				return ports.InsertImeiResult{
					Status: "error",
					IMEI:   imei,
					Error:  strPtr("imei_exist"),
				}
			}
		}

		if len(info.EndIMEI) == 1 && info.EndIMEI[0] == " " {
			info.EndIMEI = []string{end}
		} else {
			info.EndIMEI = append(info.EndIMEI, end)
		}

		_ = s.imeiRepo.Save(info)
		return ports.InsertImeiResult{
			Status: "ok",
			IMEI:   imei,
		}
	}

	_ = s.imeiRepo.Save(&legacyModels.ImeiInfo{
		StartIMEI: start,
		EndIMEI:   []string{end},
		Color:     color,
	})
	return ports.InsertImeiResult{
		Status: "ok",
		IMEI:   imei,
	}
}
