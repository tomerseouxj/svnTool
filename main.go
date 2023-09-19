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
	"svnTool/template"
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

var (
	WarningLogger *log.Logger
	InfoLogger    *log.Logger
	ErrorLogger   *log.Logger
)

func myExit() {
	fmt.Println("按任意键退出")
	fmt.Scanln()
}

func main() {
	start()
}

func start() {
	defer func() {
		err := recover()
		if err != nil {
			ErrorLogger.Println(err)
			myExit()
		}
	}()
	// 初始化日志
	file, err := os.OpenFile("logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	InfoLogger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	WarningLogger = log.New(file, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	// 读取配置文件
	cfgs := template.Root{}
	template.ReadCfg(&cfgs)
	fmt.Printf("读取配置成功 version: %v\n", cfgs.Platforms[0].Items[0].InitialVesion)

	selectIdx := 1
	if len(cfgs.Platforms[0].Items) > 1 {
		fmt.Printf("请选择要导出的版本：")
		// 打印输出item列表，让用户选择
		for i := 0; i < len(cfgs.Platforms[0].Items); i++ {
			fmt.Printf("%v. %v\n", i+1, cfgs.Platforms[0].Items[i].Name)
		}
		fmt.Scanln(&selectIdx)
		if selectIdx < 1 || selectIdx > len(cfgs.Platforms[0].Items) {
			ErrorLogger.Fatalf("输入错误")
		}
	}

	cfg := cfgs.Platforms[0].Items[selectIdx-1]
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
	// 将versions根据index排序
	var verMap map[string]string
	var index = 0
	if len(versions.VersionItems) == 0 {
		verMap = make(map[string]string)
	} else {
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

		index, err = strconv.Atoi(versions.VersionItems[0].Index)
		if err != nil {
			ErrorLogger.Fatalf("conv index error: %v", err)
		}

		// 加载旧版本json数据
		verMap, err = utils.ReadVersionFile(cfg.VesionPath, lastVersionId+".json")
		if err != nil {
			ErrorLogger.Fatalf("read version json error: %v", err)
		}
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
		myExit()
		return
	}

	//获取svn root目录
	svnRoot, err := utils.GetSvnRoot(cfg.ReadPath)
	if err != nil {
		ErrorLogger.Fatalf("get svn root error: %v", err)
	}
	fmt.Printf("svn root: %v\n", svnRoot)
	svnRootSub := cfg.SvnPath[len(svnRoot):]

	now := time.Now()
	endVersion := svnXmlLogs.Logentry[len(svnXmlLogs.Logentry)-1].Revision
	timeStr2 := now.Format("20060102")
	idStr := endVersion + "_" + timeStr2
	fmt.Printf("本次版本号id: %v\n\n", idStr)

	// 创建map,key为目录名，value为list
	shareMap := make(map[string]*list.List)
	dirMap := make(map[string]DirItem)
	increaseMap := make(map[string]bool)
	totalVersion := ""

	// 解析svn日志，更新修改的版本号，并拷贝文件到指定目录
	for _, svnXmlLog := range svnXmlLogs.Logentry {
		fmt.Printf("解析版本号：%v\n", svnXmlLog.Revision)
		totalVersion += svnXmlLog.Revision + ","
		for _, path := range svnXmlLog.Paths {
			if path.Kind != "template" {
				continue
			}
			fmt.Printf("开始处理文件：%v\n", path.Path)

			fnArr := strings.Split(path.Path, "/"+cfg.RootPath+"/")
			var fnA, fnPath string
			if len(fnArr) > 1 {
				fnA = path.Path[len(svnRootSub):]
				fnPath = "/" + fnA // /res/xxx/xxx/xxx.png
			} else {
				fnA = path.Path[len(svnRootSub):]
				fnPath = fnA
			}

			if path.Action == "D" { // 删除文件，不用管
				fmt.Printf("删除文件：%v\n", fnPath)
				continue
			}

			// 忽略文件
			continueFlag := false
			for _, v := range cfgs.VersionTemplate[0].TemplateItems {
				if v.Exc == "1" {
					if fnA == v.Item { // 此文件不作处理
						continueFlag = true
						break
					}
				}
			}
			if continueFlag {
				fmt.Printf("忽略文件：%v\n", fnPath)
				continue
			}

			idx := strings.LastIndex(fnPath, "/")
			var fnPathPrefix, fn string
			if idx == -1 {
				fnPathPrefix = ""
				fn = fnPath
			} else {
				fnPathPrefix = fnPath[:idx] // res及其下的路径
				fn = fnPath[idx+1:]         // 文件名
			}

			isNew := path.Action == "A" // path.Action == "M" || path.Action == "R"
			if isNew {
				// verMap存在该key，也默认该文件为新文件
				a := fnPathPrefix[1:] + "/"
				if _, ok := verMap[a]; ok {
					isNew = false
				}
				if strings.Index(fnPathPrefix, "map/400") > 0 {
					fmt.Printf("map/400文件：%v\n", fnPathPrefix)
				}
			}

			idx2 := strings.LastIndex(fn, "_")
			var fnPrefix string // 文件名前缀
			if idx2 != -1 {
				fnPrefix = fn[:idx2]
			} else {
				fnPrefix = fn
			}

			continueFlag = false
			for _, v := range cfgs.VersionTemplate[0].TemplateItems {
				if v.Share == "1" { // 同时修改此目录下，前缀同名文件
					if strings.Index(fnPathPrefix, v.Item) != 0 { // 不是此目录下的文件
						continue
					}

					// shareMap中key不包含v.Item，则创建list，否则添加到list中且放入map中
					if _, ok := shareMap[v.Item]; !ok {
						shareMap[v.Item] = list.New()
					}
					l := shareMap[v.Item]
					// 如果list中 不包含fnPrefix，则添加到list中
					if !utils.IsContain(l, fnPrefix) {
						l.PushBack(fnPrefix)
					}

					continueFlag = true
					break
				}

				if v.Dir == "1" { // exclude文件版本号要跟着修改的版本号走
					a := fnPathPrefix + "/"
					if strings.Index(a, v.Item) < 0 { // 不是此目录下的文件
						continue
					}

					if fnPathPrefix[1:]+"/" == v.Item {
						continue
					}
					if fnPathPrefix[:1] == "/" {
						dirMap[fnPathPrefix[1:]+"/"] = DirItem{v.Item, strings.Split(v.Exclude, ";")}
					} else {
						dirMap[fnPathPrefix+"/"] = DirItem{v.Item, strings.Split(v.Exclude, ";")}
					}
					continueFlag = true
					break
				}
			}
			if continueFlag { // 此文件不作处理
				continue
			}

			if !isNew {
				if _, ok := increaseMap[fnPath]; ok { // 该文件是增量文件，不作处理
					continue
				}
			} else {
				increaseMap[fnPath] = true
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
		pathParent := cfg.ReadPath + k
		for e := v.Front(); e != nil; e = e.Next() {
			fnPrefix := e.Value.(string)

			//遍历path目录下的所有文件
			err := filepath.Walk(pathParent, func(path string, f os.FileInfo, err error) error {
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
				CopyFileTo(cfg, k, f.Name(), idStr, false)

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
			if err != nil {
				ErrorLogger.Fatalf("处理dir标记的文件 error: %v", err)
				return
			}
		}
	}

	// 处理dir标记的文件
	fmt.Println("处理dir标记的文件")
	for k, v := range dirMap {
		parentPath := cfg.ReadPath + k
		fnPathPrefix := "/" + k
		//遍历path目录下的所有文件
		err := filepath.Walk(parentPath, func(path string, f os.FileInfo, err error) error {
			if f == nil {
				return err
			}
			if f.IsDir() { // 忽略目录
				return nil
			}
			tf := strings.ReplaceAll(path, "\\", "/")
			if strings.Index(tf, fnPathPrefix) < 0 { // 不是此目录下的文件
				return nil
			}

			// 拷贝文件到指定目录
			CopyFileTo(cfg, fnPathPrefix[:len(fnPathPrefix)-1], f.Name(), idStr, false)

			// 更新json中的版本号
			key := fnPathPrefix + f.Name()
			if key[0] == '/' {
				key = key[1:]
			}
			verMap[key] = idStr

			return nil
		})
		if err != nil {
			ErrorLogger.Fatalf("处理dir标记的文件 error: %v", err)
			return
		}

		// 把exclude的文件拷贝到指定目录并更新json中的版本号
		for _, excludeFile := range v.Excludes {
			if excludeFile == "" {
				continue
			}
			var a string
			if v.DirName[len(v.DirName)-1:] == "/" {
				a = v.DirName[:len(v.DirName)-1]
			} else {
				a = v.DirName
			}
			CopyFileTo(cfg, a, excludeFile, idStr, false)
			key := v.DirName + excludeFile
			if key[0] == '/' {
				key = key[1:]
			}
			// verMap存在key，则更新
			if _, ok := verMap[key]; ok {
				verMap[key] = idStr
			}
		}
	}

	fmt.Println("共处理版本号：" + totalVersion)
	// 更新并保存此次版本号
	index++
	indexStr := strconv.Itoa(index)
	timeStr := now.Format("2006-01-02 15:04:00")
	d := utils.VersionItem{Id: idStr, Version: endVersion, Time: timeStr, Index: indexStr, InitialVesion: cfg.InitialVesion}
	//将d插入到第一个位置
	versions.VersionItems = append([]utils.VersionItem{d}, versions.VersionItems...)
	utils.SortVersionItems(&versions)

	fmt.Println("保存些次版本Note文件...")
	//保存此次版本号
	utils.SaveVersionNoteCfg(&versions, cfg.VersionNote)
	// 保存版本json数据
	fmt.Println("保存版本json数据...")
	err = utils.SaveJsonVersionFile(cfg.VesionPath, idStr+".json", verMap)
	if err != nil {
		ErrorLogger.Fatalf("保存版本json数据 error: %v", err)
	}
	fmt.Println("done!")
	// 输入任意键退出
	myExit()
}

func exportSvnLog(cfg *template.Item, startVersion int) (string, error) {
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

func CopyFileTo(cfg template.Item, fnPathPrefix string, fn string, idStr string, isNew bool) {
	if len(fnPathPrefix) > 0 && fnPathPrefix[0] == '/' {
		fnPathPrefix = fnPathPrefix[1:]
	}

	readPath := cfg.ReadPath
	if readPath[len(readPath)-1] == '\\' {
		readPath = readPath[:len(readPath)-1]
	}

	srcPath := readPath + "/" + fnPathPrefix
	if _, err := os.Stat(srcPath + "/" + fn); os.IsNotExist(err) { // 源文件不存在，忽略
		WarningLogger.Println("源文件不存在 template=%s", srcPath+"/"+fn)
		return
	}
	// 默认路径目标目录rootPath
	var targePath string
	if isNew { // 修改或替换文件，放进版本号目录
		targePath = cfg.SavePath + fnPathPrefix
		fmt.Printf("新增文件 %s 到 %s\n", fnPathPrefix+fn, readPath)
		ErrorLogger.Println("新增文件: template=%v", fnPathPrefix+"/"+fn)
	} else {
		targePath = cfg.SavePath + idStr + "/" + fnPathPrefix
		fmt.Printf("修改文件 %s 到 %s\n", fnPathPrefix+"/"+fn, idStr)
		ErrorLogger.Println("修改文件: template=%v", fnPathPrefix+"/"+fn)
	}
	err := utils.CopyFile(srcPath, targePath, fn)
	if err != nil {
		ErrorLogger.Fatalf("拷贝失败: template=%v error=%v", fn, err)
	}
}
