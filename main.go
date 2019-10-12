package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-ini/ini"
	"github.com/jlaffaye/ftp"
)

// ftp 配置
type ftpConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string
	RemotePath string
	LocalPath  string
}

// ftpSetting App配置
var ftpSetting = &ftpConfig{}

var cfg *ini.File

var c *ftp.ServerConn

func init() {
	var err error
	cfg, err = ini.Load("ftp.ini")
	if err != nil {
		log.Panicln("配置加载失败:", err)
	}
	// 将读操作提升大约 50-70% 的性能
	cfg.BlockMode = false

	cfg.Section("").MapTo(ftpSetting)
}

func main() {
	var err error
	c, err = ftp.Dial(fmt.Sprintf("%s:%d", ftpSetting.Host, ftpSetting.Port), ftp.DialWithTimeout(30*time.Second))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Login(ftpSetting.Username, ftpSetting.Password)
	if err != nil {
		log.Fatal(err)
	}

	err = uploadAllFiles(ftpSetting.LocalPath)
	if err != nil {
		log.Panic(err)
	}

	if err := c.Quit(); err != nil {
		log.Fatal(err)
	}
}

func upload(loachPath, fileName string) error {
	data, err := os.Open(loachPath)
	if err != nil {
		return err
	}
	remotePath := strings.Replace(loachPath, ftpSetting.LocalPath, ftpSetting.RemotePath, 1)
	log.Println("Local:", loachPath, "   >>>>>>>>>   ", "Remote:", remotePath)
	err = c.Stor(remotePath, data)
	if err != nil {
		return err
	}
	return nil
}

func makeDir(loachPath string) {
	remotePath := strings.Replace(loachPath, ftpSetting.LocalPath, ftpSetting.RemotePath, 1)
	c.MakeDir(remotePath)
}

// uploadAllFiles 上传指定目录下的所有文件,包含子目录下的文件
func uploadAllFiles(dirPth string) error {
	dirs, err := ioutil.ReadDir(dirPth)
	if err != nil {
		return err
	}
	makeDir(dirPth)
	pathSep := "/"
	for _, fi := range dirs {
		path := fmt.Sprintf("%s%s%s", dirPth, pathSep, fi.Name())
		if fi.IsDir() { // 目录, 递归遍历
			err := uploadAllFiles(path)
			if err != nil {
				return err
			}
		} else {
			err := upload(path, fi.Name())
			if err != nil {
				return err
			}
		}
	}
	return nil
}
