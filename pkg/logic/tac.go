package logic

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"log"

	"github.com/hsdfat8/eir/config"
	"github.com/hsdfat8/eir/models"
	"github.com/hsdfat8/eir/pkg/repository"
	"github.com/hsdfat8/eir/utils"
)

var tacMaxLength int

const maxByteCharacter = 'ÿ'
const maxByteString = "ÿ"

func normalizeTacBytes(s string) []byte {
	buf := make([]byte, 0, tacMaxLength)

	for _, r := range s {
		if r == maxByteCharacter {
			buf = append(buf, 0xFF)
		} else {
			buf = append(buf, byte(r))
		}
		if len(buf) == tacMaxLength {
			break
		}
	}

	for len(buf) < tacMaxLength {
		buf = append(buf, ' ')
	}
	return buf
}

func normalizeTac(imei string) []byte {
	b := []byte(imei)

	if len(b) >= tacMaxLength {
		return b[:tacMaxLength]
	}
	buf := make([]byte, tacMaxLength)
	copy(buf, b)
	for i := len(b); i < tacMaxLength; i++ {
		buf[i] = ' '
	}
	return buf
}

func buildImeiSearch(imeiConvert []byte) []byte {
	maxRange := bytes.Repeat([]byte{0xFF}, tacMaxLength)
	return append(append([]byte{}, imeiConvert...), maxRange...)
}

func etsPrev(tacData []models.TacInfo, imeiSearch []byte) (models.TacInfo, error) {
	var best models.TacInfo
	found := false

	for _, tac := range tacData {
		start := normalizeTacBytes(tac.StartRangeTac)

		if bytes.Compare(start, imeiSearch) < 0 {
			if !found || bytes.Compare(start, normalizeTacBytes(best.StartRangeTac)) > 0 {
				best = tac
				found = true
			}
		}
	}

	if !found {
		return models.TacInfo{}, fmt.Errorf("$end_of_table")
	}

	return best, nil
}

func etsLookup(tacData []models.TacInfo, startRange string) (models.TacInfo, error) {
	for _, tac := range tacData {
		if tac.StartRangeTac == startRange {
			return tac, nil
		}
	}
	return models.TacInfo{}, fmt.Errorf("not found")
}

func CheckTac(imei string, status models.SystemStatus) (models.CheckResult, models.TacInfo) {
	config.LoadEnv()

	tacMaxLength = utils.GetTacMaxLength()

	imeiConvert := normalizeTac(imei)
	imeiSearch := buildImeiSearch(imeiConvert)

	tacInfo, err := etsPrev(utils.TacSampleData, imeiSearch)
	if err != nil {
		return models.CheckResult{
			Status: "error",
			IMEI:   imei,
			Color:  "unknown",
		}, models.TacInfo{}
	}

	if bytes.Compare([]byte(tacInfo.EndRangeTac), imeiConvert) >= 0 {
		return models.CheckResult{
			Status: "ok",
			IMEI:   imei,
			Color:  tacInfo.Color,
		}, tacInfo
	}

	for tacInfo.PrevLink != nil {
		tacInfo, err = etsLookup(utils.TacSampleData, *tacInfo.PrevLink)
		if err != nil {
			return models.CheckResult{
				Status: "error",
				IMEI:   imei,
				Color:  "unknown",
			}, models.TacInfo{}
		}
		if bytes.Compare([]byte(tacInfo.EndRangeTac), imeiConvert) >= 0 {
			return models.CheckResult{
				Status: "ok",
				IMEI:   imei,
				Color:  tacInfo.Color,
			}, tacInfo
		}
	}
	return models.CheckResult{
		Status: "error",
		IMEI:   imei,
		Color:  "unknown",
	}, models.TacInfo{}
}

func isValidColor(c string) bool {
	switch c {
	case "black", "grey", "white":
		return true
	}
	return false
}

func fillRight(s string, pad rune) string {
	cur := utf8.RuneCountInString(s)
	if cur >= tacMaxLength {
		return s
	}
	return s + strings.Repeat(string(pad), tacMaxLength-cur)
}

func InsertTac(repo repository.TacRepository, tacInfo models.TacInfo) models.InsertTacResult {
	log.Println("Start InsertTac")
	config.LoadEnv()
	tacMaxLength = utils.GetTacMaxLength()
	if len(tacInfo.StartRangeTac) == 0 || len(tacInfo.StartRangeTac) > tacMaxLength {
		return models.InsertTacResult{
			Status:  "error",
			Error:   "invalid_length",
			TacInfo: tacInfo,
		}
	}

	newStart := fillRight(tacInfo.StartRangeTac, ' ')
	var newEnd string
	if tacInfo.EndRangeTac == "" {
		newEnd = newStart
	} else if len(tacInfo.EndRangeTac) > tacMaxLength {
		return models.InsertTacResult{
			Status:  "error",
			Error:   "invalid_length",
			TacInfo: tacInfo,
		}
	} else {
		newEnd = fillRight(tacInfo.EndRangeTac, maxByteCharacter)
	}
	if !isValidColor(tacInfo.Color) {
		return models.InsertTacResult{
			Status:  "error",
			Error:   "invalid_color",
			TacInfo: tacInfo,
		}
	}
	if newEnd < newStart {
		return models.InsertTacResult{
			Status:  "error",
			Error:   "invalid_value",
			TacInfo: tacInfo,
		}
	}
	startRangeSearch := newStart + "-" + newEnd

	if lookup, ok := repo.Lookup(startRangeSearch); ok {
		log.Println("Lookup: ", lookup)
		return models.InsertTacResult{
			Status:  "error",
			Error:   "range_exist",
			TacInfo: tacInfo,
		}
	}

	var finalPrevLink *string = nil
	prev, ok := repo.Prev(startRangeSearch)
	log.Println("prev: ", prev)
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
				prev, ok = repo.Lookup(*prev.PrevLink)
				continue
			}
			break
		}

		return models.InsertTacResult{Status: "error", Error: "range_exist", TacInfo: tacInfo}
	}

	var listUpdate []*models.TacInfo
	next, ok := repo.Next(startRangeSearch)
	log.Println("Next: ", next)
	for ok {
		if next.StartRangeTac >= newStart && next.EndRangeTac <= newEnd {
			newKeyPtr := startRangeSearch
			updatedNext := *next
			updatedNext.PrevLink = &newKeyPtr
			listUpdate = append(listUpdate, &updatedNext)

			next, ok = repo.Next(next.KeyTac)
		} else {
			break
		}
	}

	for _, u := range listUpdate {
		_ = repo.Save(u)
	}

	tacInfoInsert := &models.TacInfo{
		KeyTac:        startRangeSearch,
		StartRangeTac: newStart,
		EndRangeTac:   newEnd,
		Color:         tacInfo.Color,
		PrevLink:      finalPrevLink,
	}

	_ = repo.Save(tacInfoInsert)

	return models.InsertTacResult{
		Status:  "ok",
		TacInfo: tacInfo,
	}
}
