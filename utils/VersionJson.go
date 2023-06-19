package utils

import (
	"encoding/json"
	"os"
	"strings"
)

func ReadVersionFile(fnPath string, fn string) (map[string]string, error) {
	// 读取version.json文件中的svn起始版本号
	if strings.LastIndex(fnPath, "\\") != len(fnPath)-1 {
		fnPath += "\\"
	}
	data, err := os.ReadFile(fnPath + fn)
	if err != nil {
		return nil, err
	}
	var m = make(map[string]string)
	err = json.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func SaveJsonVersionFile(fnPath string, fn string, d map[string]string) error {
	// fnPath不存在则创建
	if _, err := os.Stat(fnPath); os.IsNotExist(err) {
		err := os.MkdirAll(fnPath, os.ModePerm)
		if err != nil {
			return err
		}
	}
	if strings.LastIndex(fnPath, "\\") != len(fnPath)-1 {
		fnPath += "\\"
	}
	// 创建fn文件并把d数据写入文件内
	dstFile, err := os.Create(fnPath + fn)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	data, err := json.MarshalIndent(d, "", "\t")
	if err != nil {
		return err
	}
	_, err = dstFile.Write(data)
	if err != nil {
		return err
	}
	return nil
}
