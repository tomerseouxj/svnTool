package utils

import "log"

// 封装的checkErr
func CheckErr(err error) (err2 error) {
	if err != nil {
		log.Panic(err)
		return err
	} else {
		return nil
	}
}
