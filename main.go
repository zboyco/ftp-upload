package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
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

var ch chan *ftpCmd

type ftpCmd struct {
	Type      int
	LocalPath string
}

var totalCount int
var currentCount int
var wg sync.WaitGroup

func init() {
	var err error
	cfg, err = ini.Load("ftp.ini")
	if err != nil {
		fmt.Println("配置加载失败:", err)
		return
	}
	// 将读操作提升大约 50-70% 的性能
	cfg.BlockMode = false

	cfg.Section("").MapTo(ftpSetting)
}

func main() {
	var err error
	c, err = ftp.Dial(fmt.Sprintf("%s:%d", ftpSetting.Host, ftpSetting.Port), ftp.DialWithTimeout(30*time.Second))
	if err != nil {
		fmt.Println("FTP连接失败:", err)
		return
	}
	fmt.Println("FTP连接成功...")
	err = c.Login(ftpSetting.Username, ftpSetting.Password)
	if err != nil {
		fmt.Println("FTP登录失败:", err)
		return
	}
	fmt.Println("FTP登录成功...")
	wg.Add(1)
	go work()

	ch = make(chan *ftpCmd, 1000)
	fmt.Println("上传开始...")
	err = uploadAllFiles(ftpSetting.LocalPath)
	close(ch)
	if err != nil {
		fmt.Println("文件上传失败:", err)
		return
	}

	wg.Wait()

	if err := c.Quit(); err != nil {
		fmt.Println("FTP登出失败:", err)
		return
	}
	fmt.Println("\n上传完成！！！")
}

func work() {
	defer wg.Done()
	for {
		cmd, ok := <-ch
		if !ok {
			return
		}
		if cmd.Type == 0 {
			makeDir(cmd.LocalPath)
			continue
		}
		currentCount++
		data, err := os.Open(cmd.LocalPath)
		if err != nil {
			panic(err)
		}
		remotePath := strings.Replace(cmd.LocalPath, ftpSetting.LocalPath, ftpSetting.RemotePath, 1)

		err = c.Stor(remotePath, data)
		if err != nil {
			fmt.Println(cmd.LocalPath)
			panic(err)
		}
		stdPrint()
	}
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
	ch <- &ftpCmd{
		Type:      0,
		LocalPath: dirPth,
	}
	pathSep := "/"
	for _, fi := range dirs {
		path := fmt.Sprintf("%s%s%s", dirPth, pathSep, fi.Name())
		if fi.IsDir() { // 目录, 递归遍历
			err := uploadAllFiles(path)
			if err != nil {
				return err
			}
		} else {
			ch <- &ftpCmd{
				Type:      1,
				LocalPath: path,
			}
			totalCount++
			stdPrint()
		}
	}
	return nil
}

func stdPrint() {
	fmt.Print("\r上传进度:", fmt.Sprintf("%d/%d", currentCount, totalCount))
}
