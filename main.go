package main

import (
	"bytes"
	"container/list"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"svnTool/utils"
	"time"
)

const (
	DEFAULT_SMALLEST_TIME_STRING = "1000-03-20T08:38:17.428370Z"
	DATE_DAY                     = "2006-01-02"
	DATE_HOUR                    = "2006-01-02 15"
	DATE_SECOND                  = "2006-01-02T15:04:05Z"
)

type DirItem struct {
	DirName  string
	Excludes []string
}

func main() {
	// 初始化日志
	file, err := os.OpenFile("logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	InfoLogger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	WarningLogger = log.New(file, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	// 读取配置文件
	data := utils.Root{}
	utils.ReadCfg(&data)
	fmt.Printf("读取配置成功 version: %v\n", data.Platforms[0].Items[0].InitialVesion)

	selectIdx := 1
	if len(data.Platforms[0].Items) > 1 {
		fmt.Printf("请选择要导出的版本：")
		// 打印输出item列表，让用户选择
		for i := 0; i < len(data.Platforms[0].Items); i++ {
			fmt.Printf("%v. %v\n", i+1, data.Platforms[0].Items[i].Name)
		}
		fmt.Scanln(&selectIdx)
		if selectIdx < 1 || selectIdx > len(data.Platforms[0].Items) {
			ErrorLogger.Fatalf("输入错误")
		}
	}

	cfg := data.Platforms[0].Items[selectIdx-1]
	//判断源目录是否存在
	if _, err := os.Stat(cfg.ReadPath); os.IsNotExist(err) {
		ErrorLogger.Fatalf("readPath路径不存在 '%s'", cfg.ReadPath)
	}

	// 获取svn初始版本号
	versions := utils.VersionRoot{}
	utils.ReadVersionNoteCfg(&versions, cfg.VersionNote)
	startVersion, err := strconv.Atoi(cfg.InitialVesion)
	if err != nil {
		ErrorLogger.Fatalf("InitialVesionNote error: %v", err)
	}
	lastVersionData := versions.VersionItems[0]
	lastVersionId := lastVersionData.Id
	lastVersion, err := strconv.Atoi(lastVersionData.Version)
	if err != nil {
		ErrorLogger.Fatalf("convert version error: %v", err)
	}
	if startVersion < lastVersion {
		startVersion = lastVersion + 1
	}
	fmt.Printf("初始版本号：%v\n", startVersion)

	// 加载旧版本json数据
	verMap, err := utils.ReadVersionFile(cfg.VesionPath, lastVersionId+".json")
	if err != nil {
		ErrorLogger.Fatalf("read version json error: %v", err)
	}

	// 导出svn日志
	fmt.Printf("开始导出svn日志\n")
	logData, err := exportSvnLog(&cfg, startVersion)
	if err != nil {
		ErrorLogger.Fatalf("export svn log error: %v", err)
	}
	logDataBytes := []byte(logData)
	svnXmlLogs := utils.SvnXmlLogs{}
	err = xml.Unmarshal(logDataBytes, &svnXmlLogs)
	if err != nil {
		log.Fatal(err)
	}
	if len(svnXmlLogs.Logentry) == 0 {
		fmt.Printf("没有新版本\n")
		return
	}

	//获取svn root目录
	svnRoot, err := utils.GetSvnRoot(cfg.ReadPath)
	if err != nil {
		ErrorLogger.Fatalf("get svn root error: %v", err)
	}
	fmt.Printf("svn root: %v\n", svnRoot)

	now := time.Now()
	endVersion := svnXmlLogs.Logentry[0].Revision
	timeStr2 := now.Format("20060102")
	idStr := endVersion + "_" + timeStr2
	fmt.Printf("本次版本号id: %v\n\n", idStr)

	// 创建map,key为目录名，value为list
	shareMap := make(map[string]*list.List)
	dirMap := make(map[string]DirItem)
	increaseMap := make(map[string]bool)

	// 解析svn日志，更新修改的版本号，并拷贝文件到指定目录
	for _, svnXmlLog := range svnXmlLogs.Logentry {
		fmt.Printf("解析版本号：%v\n", svnXmlLog.Revision)
		for _, path := range svnXmlLog.Paths {
			if path.Kind != "file" {
				continue
			}

			fnArr := strings.Split(path.Path, "/"+cfg.RootPath+"/")
			fnPath := "/" + cfg.RootPath + "/" + fnArr[1] // /res/xxx/xxx/xxx.png

			if path.Action == "D" { // 删除文件，不用管
				fmt.Printf("删除文件：%v\n", fnPath)
				continue
			}

			isNew := path.Action == "M" || path.Action == "R"
			if !isNew {
				if _, ok := increaseMap[fnPath]; ok { // 该文件是增量文件，不作处理
					continue
				}
			} else {
				increaseMap[fnPath] = true
			}

			idx := strings.LastIndex(fnPath, "/")
			fnPathPrefix := fnPath[:idx] // res及其下的路径
			fn := fnPath[idx+1:]         // 文件名

			idx2 := strings.LastIndex(fn, "_")
			var fnPrefix string // 文件名前缀
			if idx2 != -1 {
				fnPrefix = fn[:idx2]
			} else {
				fnPrefix = fn
			}

			continueFlag := false
			for _, v := range data.VersionTemplate[0].TemplateItems {
				if v.Exc == "1" { // 不用管
					if fnArr[1] != v.Item { // 此文件不作处理
						continue
					}

					continueFlag = true
					break
				}

				if v.Share == "1" { // 同时修改此目录下，前缀同名文件
					if strings.Index(fnPathPrefix, v.Item) != 0 { // 不是此目录下的文件
						continue
					}

					// shareMap中key不包含v.Item，则创建list，否则添加到list中且放入map中
					if _, ok := shareMap[v.Item]; !ok {
						shareMap[v.Item] = list.New()
					}
					list := shareMap[v.Item]
					// 如果list中 不包含fnPrefix，则添加到list中
					if !utils.IsContain(list, fnPrefix) {
						list.PushBack(fnPrefix)
					}

					continueFlag = true
					break
				}

				if v.Dir == "1" { // exclude文件版本号要跟着修改的版本号走
					if strings.Index(fnPathPrefix, v.Item) != 0 { // 不是此目录下的文件
						continue
					}

					dirMap[v.Item] = DirItem{fnPrefix, strings.Split(v.Item, ";")}
					break
				}
			}
			if continueFlag { // 此文件不作处理
				continue
			}

			// 拷贝文件到指定目录
			CopyFileTo(cfg, fnPathPrefix, fn, idStr, isNew)

			// 更新json中的版本号
			if !isNew {
				if fnPath[0] == '/' {
					fnPath = fnPath[1:]
				}
				verMap[fnPath] = idStr
			}
		}
	}

	// 处理share标记的文件
	fmt.Println("处理share标记的文件")
	for k, v := range shareMap {
		pathParent := cfg.ReadPath + "/" + k
		for e := v.Front(); e != nil; e = e.Next() {
			fnPrefix := e.Value.(string)

			//遍历path目录下的所有文件
			filepath.Walk(pathParent, func(path string, f os.FileInfo, err error) error {
				if f == nil {
					return err
				}
				if f.IsDir() { // 忽略目录
					return nil
				}
				if strings.Index(f.Name(), fnPrefix) != 0 { // 不是此前缀的文件
					return nil
				}

				// 拷贝文件到指定目录
				isNew := false
				CopyFileTo(cfg, k, f.Name(), idStr, isNew)

				// 更新json中的版本号
				key := k + f.Name()
				if key[0] == '/' {
					key = key[1:]
				}
				// verMap存在key，则更新
				if _, ok := verMap[key]; ok {
					verMap[key] = idStr
				}

				return nil
			})
		}
	}

	// 处理dir标记的文件
	fmt.Println("处理dir标记的文件")
	for k, v := range dirMap {
		path := cfg.ReadPath + "/" + k + "/"
		fnPathPrefix := "/" + cfg.RootPath + "/" + k + "/"
		//遍历path目录下的所有文件
		filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
			if f == nil {
				return err
			}
			if f.IsDir() { // 忽略目录
				return nil
			}
			if strings.Index(path, v.DirName) != 0 { // 不是此目录下的文件
				return nil
			}

			// 拷贝文件到指定目录
			CopyFileTo(cfg, fnPathPrefix, f.Name(), idStr, false)

			// 更新json中的版本号
			key := fnPathPrefix + f.Name()
			if key[0] == '/' {
				key = key[1:]
			}
			verMap[key] = idStr

			return nil
		})

		// 把exclude的文件拷贝到指定目录并更新json中的版本号
		for _, excludeFile := range v.Excludes {
			CopyFileTo(cfg, fnPathPrefix, excludeFile, idStr, false)
			key := fnPathPrefix + excludeFile
			if key[0] == '/' {
				key = key[1:]
			}
			// verMap存在key，则更新
			if _, ok := verMap[key]; ok {
				verMap[key] = idStr
			}
		}
	}

	// 更新并保存此次版本号
	index, err := strconv.Atoi(versions.VersionItems[0].Index)
	if err != nil {
		ErrorLogger.Fatalf("conv index error: %v", err)
	}
	index++
	indexStr := strconv.Itoa(index)
	timeStr := now.Format("2006-01-02 15:04:00")
	d := utils.VersionItem{Id: idStr, Version: endVersion, Time: timeStr, Index: indexStr, InitialVesion: cfg.InitialVesion}
	//将d插入到第一个位置
	versions.VersionItems = append([]utils.VersionItem{d}, versions.VersionItems...)

	fmt.Println("保存些次版本Note文件...")
	//保存此次版本号
	utils.SaveVersionNoteCfg(&versions, cfg.VersionNote)
	// 保存版本json数据
	fmt.Println("保存版本json数据...")
	utils.SaveJsonVersionFile(cfg.VesionPath, idStr+".json", verMap)
	fmt.Println("done!")
	// 输入任意键退出
	fmt.Println("输入任意键退出...")
	var input string
	fmt.Scanln(&input)
}

func exportSvnLog(cfg *utils.Item, startVersion int) (string, error) {
	cmd := exec.Command("svn", "log", "-r", strconv.Itoa(startVersion)+":HEAD", "-v", cfg.SvnPath, "--xml")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	} else {
		return out.String(), nil
	}
}

func CopyFileTo(cfg utils.Item, fnPathPrefix string, fn string, idStr string, isNew bool) {
	srcPath := cfg.ReadPath + "/" + fnPathPrefix
	// 默认路径目标目录rootPath
	var targePath string
	if !isNew { // 修改或替换文件，放进版本号目录
		targePath = cfg.SavePath + idStr + fnPathPrefix
		fmt.Printf("拷贝文件 %s 到 %s\n", fnPathPrefix+fn, idStr)
	} else {
		targePath = cfg.SavePath + fnPathPrefix
		fmt.Printf("新增文件 %s 到 %s\n", fnPathPrefix+fn, cfg.RootPath)
	}
	err := utils.CopyFile(srcPath, targePath, fn)
	if err != nil {
		ErrorLogger.Fatalf("拷贝失败: file=%v error=%v", fn, err)
	}
}
