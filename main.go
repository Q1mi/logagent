package main

import (
	"fmt"
	"github.com/go-ini/ini"
	"github.com/sirupsen/logrus"
	"logagent/etcd"
	"logagent/kafka"
	"logagent/tailfile"
	"logagent/utils"
	"os"
	"strings"
	"time"
)

// logagent

var (
	log *logrus.Logger
)

type config struct {
	KafkaConfig   `ini:"kafka"`
	CollectConfig `ini:"collect"`
	EtcdConfig    `ini:"etcd"`
}

type KafkaConfig struct {
	Address  string `ini:"address"`
	ChanSize int    `ini:"chan_size"`
}

type CollectConfig struct {
	Logfile string `ini:"logfile"`
}

type EtcdConfig struct {
	Address    string `ini:"address"`
	CollectLogKey string `ini:"collect_log_key"`
}

func initLogger() {
	log = logrus.New()
	// 设置日志输出为os.Stdout
	log.Out = os.Stdout
	log.Level = logrus.DebugLevel
	// 可以设置像文件等任意`io.Writer`类型作为日志输出
	// file, err := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	// if err == nil {
	//  log.Out = file
	// } else {
	//  log.Info("Failed to log to file, using default stderr")
	// }

	log.Info("init log success")
}

// 业务逻辑处理
func run() (err error) {
	// 实时监控etcd中日志收集配置项的变化，对tailObj进行管理
	for {
		time.Sleep(time.Second)
	}
	return
}

func main() {
	initLogger()
	var cfg config // app config
	// 1. 初始化配置文件
	err := ini.MapTo(&cfg, "./conf/config.ini")
	if err != nil {
		panic(fmt.Sprintf("init config failed, err:%v", err))
	}
	// 2. 初始化kafka
	err = kafka.Init(strings.Split(cfg.KafkaConfig.Address, ","), cfg.KafkaConfig.ChanSize)
	if err != nil {
		panic(fmt.Sprintf("init kafka failed, err:%v", err))
	}

	// 3. 初始化etcd
	ip, err := utils.GetOutboundIP()
	if err != nil {
		panic(fmt.Sprintf("get local ip failed, err:%v", err))
	}
	collectLogKey := fmt.Sprintf(cfg.EtcdConfig.CollectLogKey, ip)
	err = etcd.Init(strings.Split(cfg.EtcdConfig.Address, ","), collectLogKey)
	if err != nil {
		panic(fmt.Sprintf("init etcd failed, err:%v", err))
	}
	log.Debug("init etcd success!")

	collectConf, err := etcd.GetConf(collectLogKey)
	if err != nil {
		panic(fmt.Sprintf("get collect conf from etcd failed, err:%v", err))
	}
	log.Debugf("%#v", collectConf)

	// 获取一个新日志配置项的chan
	newConfChan := etcd.WatchChan()
	// 4. 初始化tail
	err = tailfile.Init(collectConf, newConfChan) // 此处为修改后的Init
	if err != nil {
		panic(fmt.Sprintf("init tail failed, err:%v", err))
	}
	log.Debug("init tail success!")

	// 5. 开始干活
	err = run()
	if err != nil {
		panic(fmt.Sprintf("main.run failed, err:%v", err))
	}
	log.Debug("logagent exit")
}
