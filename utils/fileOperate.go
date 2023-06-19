package utils

import "os"

func CopyFile(src, dst, fn string) error {
	// dst目录不存在则创建
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		err := os.MkdirAll(dst, os.ModePerm)
		if err != nil {
			return err
		}
	}
	// 拷贝文件到dist
	srcFile, err := os.Open(src + "/" + fn)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dst + "/" + fn)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	return nil
}
