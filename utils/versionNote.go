package utils

import (
	"encoding/xml"
	"os"
	"strconv"
	"strings"
)

type VersionRoot struct {
	XMLName      xml.Name      `xml:"root"`
	VersionItems []VersionItem `xml:"item"`
}
type VersionItem struct {
	XMLName       xml.Name `xml:"item"`
	Id            string   `xml:"id,attr"`
	Version       string   `xml:"version,attr"`
	Time          string   `xml:"time,attr"`
	Index         string   `xml:"index,attr"`
	InitialVesion string   `xml:"initialVesion,attr"`
}

func ReadVersionNoteCfg(cfg *VersionRoot, path string) {
	if strings.LastIndex(path, "\\") != len(path)-1 {
		path += "\\"
	}

	// 读取version.xml文件中的svn起始版本号
	data, err := os.ReadFile(path + "versionNote.xml")
	if err != nil {
		return
	}
	err = xml.Unmarshal(data, &cfg)
	if err != nil {
		return
	}

	// 对index进行排序
	for i := 0; i < len(cfg.VersionItems); i++ {
		for j := i; j < len(cfg.VersionItems); j++ {
			a, err2 := strconv.Atoi(cfg.VersionItems[i].Index)
			if err2 != nil {
				CheckErr(err2)
			}
			b, err2 := strconv.Atoi(cfg.VersionItems[j].Index)
			if err2 != nil {
				CheckErr(err2)
			}
			if a < b {
				cfg.VersionItems[i], cfg.VersionItems[j] = cfg.VersionItems[j], cfg.VersionItems[i]
			}
		}
	}
}

func SaveVersionNoteCfg(cfg *VersionRoot, path string) {
	if strings.LastIndex(path, "\\") != len(path)-1 {
		path += "\\"
	}

	data, err := xml.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return
	}
	err = os.WriteFile(path+"versionNote.xml", data, 0666)
	if err != nil {
		return
	}
}

func SortVersionItems(cfg *VersionRoot) {
	for i := 0; i < len(cfg.VersionItems); i++ {
		for j := i; j < len(cfg.VersionItems); j++ {
			a, err2 := strconv.Atoi(cfg.VersionItems[i].Index)
			if err2 != nil {
				CheckErr(err2)
			}
			b, err2 := strconv.Atoi(cfg.VersionItems[j].Index)
			if err2 != nil {
				CheckErr(err2)
			}
			if a < b {
				cfg.VersionItems[i], cfg.VersionItems[j] = cfg.VersionItems[j], cfg.VersionItems[i]
			}
		}
	}
}
