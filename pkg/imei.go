package pkg

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hsdfat8/eir/config"
	"github.com/hsdfat8/eir/models"
	"github.com/hsdfat8/eir/pkg/repository"
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
		return "", fmt.Errorf("IMEI not found")
	}
	return bestColor, nil
}

func CheckImei(imei string, status models.SystemStatus) models.CheckResult {
	imei = normalizeImei(imei)
	if utils.IsOverLoad(status) {
		return models.CheckResult{
			Status: "error",
			IMEI:   imei,
			Color:  "overload",
		}
	}
	color, err := lookupImeiInfo(imei)
	if err != nil {
		return models.CheckResult{
			Status: "error",
			IMEI:   imei,
			Color:  "unkown",
		}
	}

	return models.CheckResult{
		Status: "ok",
		IMEI:   imei,
		Color:  color,
	}
}

func validateAddImei(imei string, color string) error {
	if imei == "" {
		return errors.New("invalid_parameter")
	}
	for _, character := range imei {
		if character < '0' || character > '9' {
			return errors.New("invalid_value")
		}
	}

	if len(imei) > imeiMaxLength {
		return errors.New("invalid_length")
	}
	switch color {
	case "b", "g", "w":
	default:
		return errors.New("invalid_color")
	}
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

func InsertImei(repo repository.ImeiRepository, imei string, color string, status models.SystemStatus) models.InsertImeiResult {

	config.LoadEnv()
	imeiMaxLength = utils.GetImeiMaxLength()
	imeiCheckLength = utils.GetImeiCheckLength()

	if utils.IsOverLoad(status) {
		return models.InsertImeiResult{
			Status: "error",
			IMEI:   imei,
			Error:  "overload",
		}
	}

	if err := validateAddImei(imei, color); err != nil {
		return models.InsertImeiResult{
			Status: "error",
			IMEI:   imei,
			Error:  err.Error(),
		}
	}

	start, end := normalizeImeiForInsert(imei)
	if info, ok := repo.Lookup(start); ok {
		if info.Color != color {
			return models.InsertImeiResult{
				Status: "error",
				IMEI:   imei,
				Error:  "color_conflict",
			}
		}

		for _, e := range info.EndIMEI {
			if e == end {
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

		_ = repo.Save(info)
		return models.InsertImeiResult{
			Status: "ok",
			IMEI:   imei,
		}
	}

	_ = repo.Save(&models.ImeiInfo{
		StartIMEI: start,
		EndIMEI:   []string{end},
		Color:     color,
	})
	return models.InsertImeiResult{
		Status: "ok",
		IMEI:   imei,
	}
}
