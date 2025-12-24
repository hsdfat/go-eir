package logic

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/hsdfat8/eir/config"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/logger"
	"github.com/hsdfat8/eir/models"
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
	logger.Log.Debugw("etsPrev started", "imei_search_len", len(imeiSearch))
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
		logger.Log.Debugw("etsPrev end of table", "imei_search_len", len(imeiSearch))
		return models.TacInfo{}, fmt.Errorf("$end_of_table")
	}

	logger.Log.Debugw("etsPrev found", "start_range", best.StartRangeTac, "color", best.Color)
	return best, nil
}

func etsLookup(tacData []models.TacInfo, startRange string) (models.TacInfo, error) {
	logger.Log.Debugw("etsLookup started", "start_range", startRange)
	for _, tac := range tacData {
		if tac.StartRangeTac == startRange {
			logger.Log.Debugw("etsLookup found", "start_range", startRange, "color", tac.Color)
			return tac, nil
		}
	}
	logger.Log.Debugw("etsLookup not found", "start_range", startRange)
	return models.TacInfo{}, fmt.Errorf("not found")
}

func CheckTac(imei string, status models.SystemStatus) (models.CheckResult, models.TacInfo) {
	logger.Log.Debugw("CheckTac logic started", "imei", imei)

	config.LoadEnv()

	tacMaxLength = utils.GetTacMaxLength()

	imeiConvert := normalizeTac(imei)
	imeiSearch := buildImeiSearch(imeiConvert)

	tacInfo, err := etsPrev(utils.TacSampleData, imeiSearch)
	if err != nil {
		logger.Log.Warnw("CheckTac etsPrev failed", "imei", imei, "error", err)
		return models.CheckResult{
			Status: "error",
			IMEI:   imei,
			Color:  "unknown",
		}, models.TacInfo{}
	}

	if bytes.Compare([]byte(tacInfo.EndRangeTac), imeiConvert) >= 0 {
		logger.Log.Debugw("CheckTac logic completed - match found", "imei", imei, "color", tacInfo.Color, "key_tac", tacInfo.KeyTac)
		return models.CheckResult{
			Status: "ok",
			IMEI:   imei,
			Color:  tacInfo.Color,
		}, tacInfo
	}

	logger.Log.Debugw("CheckTac checking prev links", "imei", imei, "current_key", tacInfo.KeyTac)
	for tacInfo.PrevLink != nil {
		tacInfo, err = etsLookup(utils.TacSampleData, *tacInfo.PrevLink)
		if err != nil {
			logger.Log.Warnw("CheckTac etsLookup failed during prev link traversal", "imei", imei, "error", err)
			return models.CheckResult{
				Status: "error",
				IMEI:   imei,
				Color:  "unknown",
			}, models.TacInfo{}
		}
		if bytes.Compare([]byte(tacInfo.EndRangeTac), imeiConvert) >= 0 {
			logger.Log.Debugw("CheckTac logic completed - match found via prev link", "imei", imei, "color", tacInfo.Color, "key_tac", tacInfo.KeyTac)
			return models.CheckResult{
				Status: "ok",
				IMEI:   imei,
				Color:  tacInfo.Color,
			}, tacInfo
		}
	}
	logger.Log.Warnw("CheckTac logic completed - no match found", "imei", imei)
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

func InsertTac(repo ports.IMEIRepository, tacInfo models.TacInfo) models.InsertTacResult {
	logger.Log.Infow("InsertTac logic started", "start_range", tacInfo.StartRangeTac, "end_range", tacInfo.EndRangeTac, "color", tacInfo.Color)

	// config.LoadEnv()
	tacMaxLength = utils.GetTacMaxLength()
	logger.Log.Debugw("tacMaxLength: ", tacMaxLength)
	if len(tacInfo.StartRangeTac) == 0 || len(tacInfo.StartRangeTac) > tacMaxLength {
		logger.Log.Warnw("InsertTac invalid start range length", "start_range", tacInfo.StartRangeTac, "length", len(tacInfo.StartRangeTac), "max_length", tacMaxLength)
		return models.InsertTacResult{
			Status:  "error",
			Error:   "invalid_length",
			TacInfo: tacInfo,
		}
	}

	for _, character := range tacInfo.StartRangeTac {
		if character < '0' || character > '9' {
			return models.InsertTacResult{
				Status:  "error",
				Error:   "invalid_value",
				TacInfo: tacInfo,
			}
		}
	}

	for _, character := range tacInfo.EndRangeTac {
		if character < '0' || character > '9' {
			return models.InsertTacResult{
				Status:  "error",
				Error:   "invalid_value",
				TacInfo: tacInfo,
			}
		}
	}

	newStart := fillRight(tacInfo.StartRangeTac, ' ')
	var newEnd string
	if tacInfo.EndRangeTac == "" {
		newEnd = newStart
	} else if len(tacInfo.EndRangeTac) > tacMaxLength {
		logger.Log.Warnw("InsertTac invalid end range length", "end_range", tacInfo.EndRangeTac, "length", len(tacInfo.EndRangeTac), "max_length", tacMaxLength)
		return models.InsertTacResult{
			Status:  "error",
			Error:   "invalid_length",
			TacInfo: tacInfo,
		}
	} else {
		newEnd = fillRight(tacInfo.EndRangeTac, maxByteCharacter)
	}
	if !isValidColor(tacInfo.Color) {
		logger.Log.Warnw("InsertTac invalid color", "color", tacInfo.Color)
		return models.InsertTacResult{
			Status:  "error",
			Error:   "invalid_color",
			TacInfo: tacInfo,
		}
	}
	if newEnd < newStart {
		logger.Log.Warnw("InsertTac invalid range", "new_start", newStart, "new_end", newEnd)
		return models.InsertTacResult{
			Status:  "error",
			Error:   "invalid_value",
			TacInfo: tacInfo,
		}
	}
	startRangeSearch := newStart + "-" + newEnd
	logger.Log.Debugw("InsertTac normalized ranges", "new_start", newStart, "new_end", newEnd, "key", startRangeSearch)
	ctx := context.Background()
	if lookup, ok := repo.LookupTacInfo(ctx, startRangeSearch); ok {
		logger.Log.Warnw("InsertTac range already exists", "key", startRangeSearch, "lookup", lookup)
		return models.InsertTacResult{
			Status:  "error",
			Error:   "range_exist",
			TacInfo: tacInfo,
		}
	}

	var bestParent *ports.TacInfo
	var listUpdate []*ports.TacInfo

	p, ok := repo.PrevTacInfo(ctx, startRangeSearch)
	for ok {
		isParent := p.StartRangeTac <= newStart && p.EndRangeTac >= newEnd
		isChild := p.StartRangeTac >= newStart && p.EndRangeTac <= newEnd

		if isParent {
			if bestParent == nil || (p.StartRangeTac >= bestParent.StartRangeTac && p.EndRangeTac <= bestParent.EndRangeTac) {
				bestParent = p
			}
		} else if isChild {
			u := *p
			u.PrevLink = &startRangeSearch
			listUpdate = append(listUpdate, &u)
		} else if p.EndRangeTac >= newStart {
			return models.InsertTacResult{Status: "error", Error: "range_exist", TacInfo: tacInfo}
		}

		if p.EndRangeTac < newStart {
			if p.PrevLink == nil || *p.PrevLink == "" {
				break
			}
			parent, found := repo.LookupTacInfo(ctx, *p.PrevLink)
			if found && parent.StartRangeTac <= newStart && parent.EndRangeTac >= newEnd {
				if bestParent == nil || (parent.StartRangeTac >= bestParent.StartRangeTac && parent.EndRangeTac <= bestParent.EndRangeTac) {
					bestParent = parent
				}
			}
			break
		}
		p, ok = repo.PrevTacInfo(ctx, p.KeyTac)
	}

	n, ok := repo.NextTacInfo(ctx, startRangeSearch)
	for ok {
		isParent := n.StartRangeTac <= newStart && n.EndRangeTac >= newEnd
		isChild := n.StartRangeTac >= newStart && n.EndRangeTac <= newEnd

		if isParent {
			if bestParent == nil || (n.StartRangeTac >= bestParent.StartRangeTac && n.EndRangeTac <= bestParent.EndRangeTac) {
				bestParent = n
			}
		} else if isChild {
			u := *n
			u.PrevLink = &startRangeSearch
			listUpdate = append(listUpdate, &u)
		} else if n.StartRangeTac <= newEnd {
			return models.InsertTacResult{Status: "error", Error: "range_exist", TacInfo: tacInfo}
		}

		if n.StartRangeTac > newEnd {
			break
		}
		n, ok = repo.NextTacInfo(ctx, n.KeyTac)
	}

	var finalPrevLink *string
	if bestParent != nil {
		k := bestParent.KeyTac
		finalPrevLink = &k
	}

	tacInsert := &ports.TacInfo{
		KeyTac: startRangeSearch, StartRangeTac: newStart, EndRangeTac: newEnd,
		Color: tacInfo.Color, PrevLink: finalPrevLink,
	}

	if err := repo.SaveTacInfo(ctx, tacInsert); err != nil {
		return models.InsertTacResult{Status: "error", Error: err.Error(), TacInfo: tacInfo}
	}

	for _, child := range listUpdate {
		_ = repo.SaveTacInfo(ctx, child)
	}

	return models.InsertTacResult{Status: "ok", TacInfo: tacInfo}
}

func ClearTacInfo(repo ports.IMEIRepository) {
	repo.ClearImeiInfo()
}
