package tailfile

import (
	"context"
	"fmt"
	"github.com/hpcloud/tail"
	"github.com/sirupsen/logrus"
	"logagent/kafka"
	"os"
)

// tail line from log file
var (
	log *logrus.Logger
)

func init() {
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

	log.Info("etcd:init log success")
}

type tailObj struct {
	path     string
	module   string
	topic    string
	instance *tail.Tail
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewTailObj(path, module, topic string) (tObj *tailObj, err error) {
	tObj = &tailObj{
		path:   path,
		module: module,
		topic:  topic,
	}
	ctx, cancel := context.WithCancel(context.Background())
	tObj.ctx = ctx
	tObj.cancel = cancel
	err = tObj.Init()
	return
}

// Init 是初始化tail包的函数
func (t *tailObj) Init() (err error) {
	t.instance, err = tail.TailFile(t.path, tail.Config{
		ReOpen:    true,
		Follow:    true,
		Location:  &tail.SeekInfo{Offset: 0, Whence: 2},
		MustExist: false,
		Poll:      true,
	})
	if err != nil {
		fmt.Println("init tail failed, err:", err)
		return
	}
	return
}

// 每个tailObj都要单独读取日志信息发送到kafka中
func (t *tailObj) run() {
	for {
		select {
		case <-t.ctx.Done():
			log.Warnf("the task for path:%s is stop...", t.path)
			return // 函数返回对应的goroutine就结束了
		case line, ok := <-t.instance.Lines:
			if !ok {
				log.Errorf("read line failed")
				continue
			}
			msg := &kafka.Message{
				Line:  line.Text,
				Topic: t.topic, // 先写死
			}
			err := kafka.SendLog(msg)
			if err != nil {
				log.Errorf("send to kafka failed, err:%v\n", err)
			}
		}
		log.Debug("send msg to kafka success")
	}
}

// ReadLine read line from tailObj
func (t *tailObj) ReadLine() (line *tail.Line, err error) {
	var ok bool
	line, ok = <-t.instance.Lines
	if !ok {
		err = fmt.Errorf("read line failed, err:%v", err)
		return
	}
	return
}
