package utils

import "log"

var (
	WarningLogger *log.Logger
	InfoLogger    *log.Logger
	ErrorLogger   *log.Logger
)

// 封装的checkErr
func CheckErr(err error) (err2 error) {
	if err != nil {
		log.Panic(err)
		return err
	} else {
		return nil
	}
}
