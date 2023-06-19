package utils

import (
	"encoding/xml"
	"fmt"
	"os"
)

type Root struct {
	XMLName         xml.Name          `xml:"root"`
	Platforms       []Platform        `xml:"platform"`
	VersionTemplate []VersionTemplate `xml:"versionTemplate"`
}
type Platform struct {
	XMLName xml.Name `xml:"platform"`
	Items   []Item   `xml:"item"`
}
type Item struct {
	XMLName       xml.Name `xml:"item"`
	Id            string   `xml:"id,attr"`
	Name          string   `xml:"name,attr"`
	Username      string   `xml:"username"`
	Password      string   `xml:"password"`
	ReadPath      string   `xml:"readPath"`
	SavePath      string   `xml:"savePath"`
	VesionPath    string   `xml:"vesionPath"`
	VersionNote   string   `xml:"versionNote"`
	SvnPath       string   `xml:"svnPath"`
	InitialVesion string   `xml:"initialVesion"`
	RootPath      string   `xml:"rootPath"`
}

type VersionTemplate struct {
	XMLName       xml.Name       `xml:"versionTemplate"`
	TemplateItems []TemplateItem `xml:"item"`
}
type TemplateItem struct {
	XMLName xml.Name `xml:"item"`
	Item    string   `xml:",chardata"`
	Exc     string   `xml:"exc,attr"`
	Share   string   `xml:"share,attr"`
	Dir     string   `xml:"dir,attr"`
	Exclude string   `xml:"extend,attr"`
}

func ReadCfg(cfg *Root) {
	// 读取version.xml文件中的svn起始版本号
	data, err := os.ReadFile("config.xml") // For read access.
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}
	err = xml.Unmarshal(data, &cfg)
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}
}
