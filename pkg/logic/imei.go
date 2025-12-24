package logic

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hsdfat8/eir/config"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/logger"
	"github.com/hsdfat8/eir/models"
	"github.com/hsdfat8/eir/utils"
)

var imeiCheckLength int
var imeiMaxLength int

func normalizeImei(imei string) string {
	imeiLength := len(imei)
	if imeiLength > imeiCheckLength {
		return imei[:imeiCheckLength]
	} else if imeiLength == imeiCheckLength {
		return imei
	} else {
		return imei + strings.Repeat(" ", imeiCheckLength-imeiLength)
	}
}

func lookupImeiInfo(imei string) (string, error) {
	logger.Log.Debugw("lookupImeiInfo started", "imei", imei)
	bestLen := -1
	bestColor := ""

	for _, info := range utils.ImeiSampleData {
		if strings.HasPrefix(info.StartIMEI, imei) {
			if len(info.StartIMEI) > bestLen {
				bestLen = len(info.StartIMEI)
				bestColor = info.Color
			}
		}
	}
	if bestColor == "" {
		logger.Log.Debugw("lookupImeiInfo IMEI not found", "imei", imei)
		return "", fmt.Errorf("IMEI not found")
	}
	logger.Log.Debugw("lookupImeiInfo found color", "imei", imei, "color", bestColor)
	return bestColor, nil
}

func CheckImei(imei string, status models.SystemStatus) models.CheckResult {
	logger.Log.Debugw("CheckImei logic started", "imei", imei, "overload_level", status.OverloadLevel)

	imei = normalizeImei(imei)
	if utils.IsOverLoad(status) {
		logger.Log.Warnw("CheckImei system overloaded", "imei", imei, "overload_level", status.OverloadLevel)
		return models.CheckResult{
			Status: "error",
			IMEI:   imei,
			Color:  "overload",
		}
	}
	color, err := lookupImeiInfo(imei)
	if err != nil {
		logger.Log.Warnw("CheckImei lookup failed", "imei", imei, "error", err)
		return models.CheckResult{
			Status: "error",
			IMEI:   imei,
			Color:  "unkown",
		}
	}

	logger.Log.Debugw("CheckImei logic completed", "imei", imei, "color", color)
	return models.CheckResult{
		Status: "ok",
		IMEI:   imei,
		Color:  color,
	}
}

func validateAddImei(imei string, color string) error {
	logger.Log.Debugw("validateAddImei started", "imei", imei, "color", color)

	if imei == "" {
		logger.Log.Warnw("validateAddImei invalid parameter", "imei", imei)
		return errors.New("invalid_parameter")
	}
	for _, character := range imei {
		if character < '0' || character > '9' {
			logger.Log.Warnw("validateAddImei invalid value", "imei", imei)
			return errors.New("invalid_value")
		}
	}

	if len(imei) > imeiMaxLength {
		logger.Log.Warnw("validateAddImei invalid length", "imei", imei, "length", len(imei), "max_length", imeiMaxLength)
		return errors.New("invalid_length")
	}
	switch color {
	case "b", "g", "w":
	default:
		logger.Log.Warnw("validateAddImei invalid color", "imei", imei, "color", color)
		return errors.New("invalid_color")
	}
	logger.Log.Debugw("validateAddImei passed", "imei", imei, "color", color)
	return nil
}

func normalizeImeiForInsert(imei string) (start string, end string) {
	if len(imei) <= imeiCheckLength {
		start = imei + strings.Repeat(" ", imeiCheckLength-len(imei))
		end = " "
		return
	}

	start = imei[:imeiCheckLength]
	end = imei[imeiCheckLength:]
	if end == "" {
		end = " "
	}
	return
}

func InsertImei(repo ports.IMEIRepository, imei string, color string, status models.SystemStatus) models.InsertImeiResult {
	logger.Log.Infow("InsertImei logic started", "imei", imei, "color", color)

	config.LoadEnv()
	imeiMaxLength = utils.GetImeiMaxLength()
	imeiCheckLength = utils.GetImeiCheckLength()

	if utils.IsOverLoad(status) {
		logger.Log.Warnw("InsertImei system overloaded", "imei", imei, "overload_level", status.OverloadLevel)
		return models.InsertImeiResult{
			Status: "error",
			IMEI:   imei,
			Error:  "overload",
		}
	}

	if err := validateAddImei(imei, color); err != nil {
		logger.Log.Warnw("InsertImei validation failed", "imei", imei, "color", color, "error", err)
		return models.InsertImeiResult{
			Status: "error",
			IMEI:   imei,
			Error:  err.Error(),
		}
	}

	start, end := normalizeImeiForInsert(imei)
	logger.Log.Debugw("InsertImei normalized", "imei", imei, "start", start, "end", end)

	if info, ok := repo.LookupImeiInfo(start); ok {
		logger.Log.Debugw("InsertImei found existing start IMEI", "imei", imei, "start", start, "existing_color", info.Color)

		if info.Color != color {
			logger.Log.Warnw("InsertImei color conflict", "imei", imei, "requested_color", color, "existing_color", info.Color)
			return models.InsertImeiResult{
				Status: "error",
				IMEI:   imei,
				Error:  "color_conflict",
			}
		}

		for _, e := range info.EndIMEI {
			if e == end {
				logger.Log.Warnw("InsertImei IMEI already exists", "imei", imei, "start", start, "end", end)
				return models.InsertImeiResult{
					Status: "error",
					IMEI:   imei,
					Error:  "imei_exist",
				}
			}
		}

		if len(info.EndIMEI) == 1 && info.EndIMEI[0] == " " {
			info.EndIMEI = []string{end}
		} else {
			info.EndIMEI = append(info.EndIMEI, end)
		}

		logger.Log.Debugw("InsertImei updating existing entry", "imei", imei, "start", start, "end", end)
		_ = repo.SaveImeiInfo(info)
		logger.Log.Infow("InsertImei logic completed successfully (updated)", "imei", imei, "start", start)
		return models.InsertImeiResult{
			Status: "ok",
			IMEI:   imei,
		}
	}

	logger.Log.Debugw("InsertImei creating new entry", "imei", imei, "start", start, "end", end, "color", color)
	_ = repo.SaveImeiInfo(&ports.ImeiInfo{
		StartIMEI: start,
		EndIMEI:   []string{end},
		Color:     color,
	})
	logger.Log.Infow("InsertImei logic completed successfully (new)", "imei", imei, "start", start)
	return models.InsertImeiResult{
		Status: "ok",
		IMEI:   imei,
	}
}
