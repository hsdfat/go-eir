package models

import "fmt"

type SystemStatus struct {
	OverloadLevel int
	TPSOverload   bool
}

type CheckResult struct {
	Status string
	IMEI   string
	Color  string
}

type InsertImeiResult struct {
	Status string
	IMEI   string
	Error  string
}

type InsertTacResult struct {
	Status  string
	TacInfo TacInfo
	Error   string
}

type ImeiInfo struct {
	StartIMEI string
	EndIMEI   []string
	Color     string
}

type TacInfo struct {
	KeyTac        string
	StartRangeTac string
	EndRangeTac   string
	Color         string
	PrevLink      *string
}

func (t *TacInfo) String() string {
	return fmt.Sprintf(
		"Key=|%s|, Start=|%s|, End=|%s|, Color=|%s|, PrevLink=|%+v|\n",
		t.KeyTac, t.StartRangeTac, t.EndRangeTac, t.Color, *t.PrevLink,
	)
}
