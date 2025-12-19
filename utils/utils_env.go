package utils

import (
	"os"
	"strconv"
)

func GetImeiMaxLength() int {
	v := os.Getenv("IMEI_MAX_LENGTH")
	if v == "" {
		return 16 //default
	}
	n, _ := strconv.Atoi(v)
	return n
}

func GetImeiCheckLength() int {
	v := os.Getenv("IMEI_CHECK_LENGTH")
	if v == "" {
		return 14 //default
	}
	n, _ := strconv.Atoi(v)
	return n
}

func GetTacMaxLength() int {
	v := os.Getenv("TAC_MAX_LENGTH")
	if v == "" {
		return 16 //default
	}
	n, _ := strconv.Atoi(v)
	return n
}
