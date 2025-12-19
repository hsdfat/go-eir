package utils

import "github.com/hsdfat8/eir/models"

func IsOverLoad(status models.SystemStatus) bool {
	return false
}

var ImeiSampleData = map[string]*models.ImeiInfo{
	"9": {
		StartIMEI: "9              ",
		EndIMEI:   []string{""},
		Color:     "b",
	},
	"91": {
		StartIMEI: "91             ",
		EndIMEI:   []string{""},
		Color:     "w",
	},
	"912": {
		StartIMEI: "912            ",
		EndIMEI:   []string{""},
		Color:     "g",
	},
	"9123": {
		StartIMEI: "9123           ",
		EndIMEI:   []string{""},
		Color:     "w",
	},
}

var TacSampleData = []models.TacInfo{
	{StartRangeTac: "9100000000000000", EndRangeTac: "9899999999999999", Color: "grey", PrevLink: nil},
}
