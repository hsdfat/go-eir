package service

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	legacyModels "github.com/hsdfat8/eir/models"
	"github.com/hsdfat8/eir/pkg/repository"
)

const (
	maxByteCharacter = 'ÿ'
	maxByteString    = "ÿ"
)

// TacLogicService encapsulates TAC-based business logic
type TacLogicService struct {
	tacMaxLength int
	tacRepo      repository.TacRepository
	sampleData   []legacyModels.TacInfo
}

// NewTacLogicService creates a new TAC logic service
func NewTacLogicService(tacMaxLength int, tacRepo repository.TacRepository, sampleData []legacyModels.TacInfo) *TacLogicService {
	return &TacLogicService{
		tacMaxLength: tacMaxLength,
		tacRepo:      tacRepo,
		sampleData:   sampleData,
	}
}

func (s *TacLogicService) normalizeTacBytes(str string) []byte {
	buf := make([]byte, 0, s.tacMaxLength)

	for _, r := range str {
		if r == maxByteCharacter {
			buf = append(buf, 0xFF)
		} else {
			buf = append(buf, byte(r))
		}
		if len(buf) == s.tacMaxLength {
			break
		}
	}

	for len(buf) < s.tacMaxLength {
		buf = append(buf, ' ')
	}
	return buf
}

func (s *TacLogicService) normalizeTac(imei string) []byte {
	b := []byte(imei)

	if len(b) >= s.tacMaxLength {
		return b[:s.tacMaxLength]
	}
	buf := make([]byte, s.tacMaxLength)
	copy(buf, b)
	for i := len(b); i < s.tacMaxLength; i++ {
		buf[i] = ' '
	}
	return buf
}

func (s *TacLogicService) buildImeiSearch(imeiConvert []byte) []byte {
	maxRange := bytes.Repeat([]byte{0xFF}, s.tacMaxLength)
	return append(append([]byte{}, imeiConvert...), maxRange...)
}

func (s *TacLogicService) etsPrev(tacData []legacyModels.TacInfo, imeiSearch []byte) (legacyModels.TacInfo, error) {
	var best legacyModels.TacInfo
	found := false

	for _, tac := range tacData {
		start := s.normalizeTacBytes(tac.StartRangeTac)

		if bytes.Compare(start, imeiSearch) < 0 {
			if !found || bytes.Compare(start, s.normalizeTacBytes(best.StartRangeTac)) > 0 {
				best = tac
				found = true
			}
		}
	}

	if !found {
		return legacyModels.TacInfo{}, fmt.Errorf("$end_of_table")
	}

	return best, nil
}

func (s *TacLogicService) etsLookup(tacData []legacyModels.TacInfo, startRange string) (legacyModels.TacInfo, error) {
	for _, tac := range tacData {
		if tac.StartRangeTac == startRange {
			return tac, nil
		}
	}
	return legacyModels.TacInfo{}, fmt.Errorf("not found")
}

// CheckTac performs TAC-based equipment check
func (s *TacLogicService) CheckTac(imei string) (legacyModels.CheckResult, legacyModels.TacInfo) {
	imeiConvert := s.normalizeTac(imei)
	imeiSearch := s.buildImeiSearch(imeiConvert)

	tacInfo, err := s.etsPrev(s.sampleData, imeiSearch)
	if err != nil {
		return legacyModels.CheckResult{
			Status: "error",
			IMEI:   imei,
			Color:  "unknown",
		}, legacyModels.TacInfo{}
	}

	if bytes.Compare([]byte(tacInfo.EndRangeTac), imeiConvert) >= 0 {
		return legacyModels.CheckResult{
			Status: "ok",
			IMEI:   imei,
			Color:  tacInfo.Color,
		}, tacInfo
	}

	for tacInfo.PrevLink != nil {
		tacInfo, err = s.etsLookup(s.sampleData, *tacInfo.PrevLink)
		if err != nil {
			return legacyModels.CheckResult{
				Status: "error",
				IMEI:   imei,
				Color:  "unknown",
			}, legacyModels.TacInfo{}
		}
		if bytes.Compare([]byte(tacInfo.EndRangeTac), imeiConvert) >= 0 {
			return legacyModels.CheckResult{
				Status: "ok",
				IMEI:   imei,
				Color:  tacInfo.Color,
			}, tacInfo
		}
	}
	return legacyModels.CheckResult{
		Status: "error",
		IMEI:   imei,
		Color:  "unknown",
	}, legacyModels.TacInfo{}
}

func (s *TacLogicService) isValidColor(c string) bool {
	switch c {
	case "black", "grey", "white":
		return true
	}
	return false
}

func (s *TacLogicService) fillRight(str string, pad rune) string {
	cur := utf8.RuneCountInString(str)
	if cur >= s.tacMaxLength {
		return str
	}
	return str + strings.Repeat(string(pad), s.tacMaxLength-cur)
}

// InsertTac inserts a TAC range into the repository
func (s *TacLogicService) InsertTac(tacInfo ports.TacInfo) ports.InsertTacResult {
	if len(tacInfo.StartRangeTac) == 0 || len(tacInfo.StartRangeTac) > s.tacMaxLength {
		return ports.InsertTacResult{
			Status:  "error",
			Error:   strPtr("invalid_length"),
			TacInfo: &tacInfo,
		}
	}

	newStart := s.fillRight(tacInfo.StartRangeTac, ' ')
	var newEnd string
	if tacInfo.EndRangeTac == "" {
		newEnd = newStart
	} else if len(tacInfo.EndRangeTac) > s.tacMaxLength {
		return ports.InsertTacResult{
			Status:  "error",
			Error:   strPtr("invalid_length"),
			TacInfo: &tacInfo,
		}
	} else {
		newEnd = s.fillRight(tacInfo.EndRangeTac, maxByteCharacter)
	}

	if !s.isValidColor(tacInfo.Color) {
		return ports.InsertTacResult{
			Status:  "error",
			Error:   strPtr("invalid_color"),
			TacInfo: &tacInfo,
		}
	}

	if newEnd < newStart {
		return ports.InsertTacResult{
			Status:  "error",
			Error:   strPtr("invalid_value"),
			TacInfo: &tacInfo,
		}
	}

	startRangeSearch := newStart + "-" + newEnd

	if lookup, ok := s.tacRepo.Lookup(startRangeSearch); ok {
		_ = lookup
		return ports.InsertTacResult{
			Status:  "error",
			Error:   strPtr("range_exist"),
			TacInfo: &tacInfo,
		}
	}

	var finalPrevLink *string = nil
	prev, ok := s.tacRepo.Prev(startRangeSearch)
	for ok {
		isParent := prev.StartRangeTac <= newStart && prev.EndRangeTac >= newEnd
		isChild := newStart <= prev.StartRangeTac && newEnd >= prev.EndRangeTac

		if isParent || isChild {
			key := prev.KeyTac
			finalPrevLink = &key
			break
		}

		if prev.EndRangeTac < newStart {
			key := prev.KeyTac
			finalPrevLink = &key

			if prev.PrevLink != nil && *prev.PrevLink != "" {
				prev, ok = s.tacRepo.Lookup(*prev.PrevLink)
				continue
			}
			break
		}

		return ports.InsertTacResult{
			Status:  "error",
			Error:   strPtr("range_exist"),
			TacInfo: &tacInfo,
		}
	}

	var listUpdate []*legacyModels.TacInfo
	next, ok := s.tacRepo.Next(startRangeSearch)
	for ok {
		if next.StartRangeTac >= newStart && next.EndRangeTac <= newEnd {
			newKeyPtr := startRangeSearch
			updatedNext := *next
			updatedNext.PrevLink = &newKeyPtr
			listUpdate = append(listUpdate, &updatedNext)

			next, ok = s.tacRepo.Next(next.KeyTac)
		} else {
			break
		}
	}

	for _, u := range listUpdate {
		_ = s.tacRepo.Save(u)
	}

	tacInfoInsert := &legacyModels.TacInfo{
		KeyTac:        startRangeSearch,
		StartRangeTac: newStart,
		EndRangeTac:   newEnd,
		Color:         tacInfo.Color,
		PrevLink:      finalPrevLink,
	}

	_ = s.tacRepo.Save(tacInfoInsert)

	resultTacInfo := &ports.TacInfo{
		KeyTac:        tacInfoInsert.KeyTac,
		StartRangeTac: tacInfoInsert.StartRangeTac,
		EndRangeTac:   tacInfoInsert.EndRangeTac,
		Color:         tacInfoInsert.Color,
		PrevLink:      tacInfoInsert.PrevLink,
	}

	return ports.InsertTacResult{
		Status:  "ok",
		TacInfo: resultTacInfo,
	}
}

// ConvertSystemStatus converts domain SystemStatus to legacy format
func ConvertSystemStatus(status models.SystemStatus) legacyModels.SystemStatus {
	return legacyModels.SystemStatus{
		OverloadLevel: status.OverloadLevel,
		TPSOverload:   status.TPSOverload,
	}
}
