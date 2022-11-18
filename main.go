package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-ini/ini"
	"github.com/jlaffaye/ftp"
	"github.com/schollz/progressbar/v3"
	log "golang.org/x/exp/slog"
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
var (
	ftpSetting   = &ftpConfig{}
	cfg          *ini.File
	c            *ftp.ServerConn
	ch           chan *ftpCmd
	totalCount   int64
	currentCount int
	bar          *progressbar.ProgressBar
	timeLine     time.Time
	wg           sync.WaitGroup
)

type ftpCmd struct {
	Type      int
	LocalPath string
}

func init() {
	var err error
	cfg, err = ini.Load("ftp.ini")
	if err != nil {
		log.Error("配置加载失败:", err)
		return
	}
	// 将读操作提升大约 50-70% 的性能
	cfg.BlockMode = false

	_ = cfg.Section("").MapTo(ftpSetting)
	timeLine, _ = time.Parse("2006-01-02 15:04:05", "1975-01-1 00:00:00")

}

func main() {
	var (
		err      error
		interval int
	)

	flag.IntVar(&interval, "s", 300, "上传时间区间(秒), 默认上传300秒内修改的文件,为0上传所有文件")
	flag.Parse()
	if interval != 0 {
		timeLine = time.Now().Add(-time.Duration(interval) * time.Second)
	}
	c, err = ftp.Dial(fmt.Sprintf("%s:%d", ftpSetting.Host, ftpSetting.Port), ftp.DialWithTimeout(30*time.Second))
	if err != nil {
		log.Error("FTP连接失败:", err)
		return
	}
	log.Info("FTP连接成功...")
	err = c.Login(ftpSetting.Username, ftpSetting.Password)
	if err != nil {
		log.Error("FTP登录失败:", err)
		return
	}
	log.Info("FTP登录成功...")
	wg.Add(1)
	go work()

	ch = make(chan *ftpCmd, 1000)
	log.Info("上传开始...")
	bar = progressbar.Default(1, "上传进度")
	err = uploadAllFiles(ftpSetting.LocalPath)
	close(ch)
	if err != nil {
		log.Error("\n文件上传失败:", err)
		return
	}

	wg.Wait()

	if err := c.Quit(); err != nil {
		log.Error("\nFTP登出失败:", err)
		return
	}
	log.Info("FTP登出成功...")
	log.Info("OK, 上传完成...")
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
			log.Info(cmd.LocalPath)
			panic(err)
		}
		if err := bar.Add(1); err != nil {
			log.Error("进度错误", err)
		}
	}
}

func makeDir(loachPath string) {
	remotePath := strings.Replace(loachPath, ftpSetting.LocalPath, ftpSetting.RemotePath, 1)
	_ = c.MakeDir(remotePath)
}

// uploadAllFiles 上传指定目录下的所有文件,包含子目录下的文件
func uploadAllFiles(dirPth string) error {
	dirs, err := os.ReadDir(dirPth)
	if err != nil {
		return err
	}
	ch <- &ftpCmd{
		Type:      0,
		LocalPath: dirPth,
	}
	pathSep := "/"
	for _, dir := range dirs {
		path := fmt.Sprintf("%s%s%s", dirPth, pathSep, dir.Name())
		if dir.IsDir() { // 目录, 递归遍历
			err := uploadAllFiles(path)
			if err != nil {
				return err
			}
		} else {
			fi, _ := dir.Info()
			checkFile(path, fi)
		}
	}
	return nil
}

// checkFile 检查文件修改日期，在时间线之后即上传
func checkFile(localPath string, file os.FileInfo) {
	if file.ModTime().Before(timeLine) {
		return
	}
	ch <- &ftpCmd{
		Type:      1,
		LocalPath: localPath,
	}
	totalCount++
	bar.ChangeMax64(totalCount)
}
