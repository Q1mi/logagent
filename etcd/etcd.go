package etcd

import (
	"context"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"logagent/common"
	"os"
	"time"

	"go.etcd.io/etcd/clientv3"
)

// etcd

var (
	log      *logrus.Logger
	client   *clientv3.Client
	confChan chan []*common.CollectEntry
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

func Init(address []string, key string) (err error) {
	client, err = clientv3.New(clientv3.Config{
		Endpoints:   address,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		// handle error!
		log.Errorf("connect to etcd failed, err:%v\n", err)
		return
	}
	confChan = make(chan []*common.CollectEntry)
	go watchConf(key) // 开始watch etcd中的日志配置项变化
	return
}

func GetConf(key string) (conf []*common.CollectEntry, err error) {
	// get
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	resp, err := client.Get(ctx, key)
	defer cancel()
	if err != nil {
		log.Errorf("get from etcd failed, err:%v\n", err)
		return
	}
	if len(resp.Kvs) == 0 {
		log.Warnf("can't get any value by key:%s from etcd", key)
		return
	}
	keyValues := resp.Kvs[0]
	err = json.Unmarshal(keyValues.Value, &conf)
	if err != nil {
		log.Errorf("unmarshal value from etcd failed, err:%v", err)
		return
	}
	log.Debugf("load conf from etcd success, conf:%#v", conf)
	return
}

func watchConf(key string) {
	for {
		rch := client.Watch(context.Background(), key) // <-chan WatchResponse
		log.Debugf("watch return, rch:%#v", rch)
		for wresp := range rch {
			if err := wresp.Err(); err != nil {
				log.Warnf("watch key:%s err:%v", key, err)
				continue
			}
			for _, ev := range wresp.Events {
				log.Debugf("Type: %s Key:%s Value:%s", ev.Type, ev.Kv.Key, ev.Kv.Value)
				// 获取了最新的日志配置项怎么传给tailTaskMgr呢？
				var newConf []*common.CollectEntry
				// 需要判断如果是删除操作
				if ev.Type == clientv3.EventTypeDelete {
					confChan <- newConf
					continue
				}
				err := json.Unmarshal(ev.Kv.Value, &newConf)
				if err != nil {
					log.Warnf("unmarshal the conf from etcd failed, err:%v", err)
					continue
				}
				confChan <- newConf
				log.Debug("send newConf to confChan success")
			}
		}
	}
}

// 向外暴露一个chan
func WatchChan() <-chan []*common.CollectEntry {
	return confChan
}
