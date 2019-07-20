package tailfile

import (
	"logagent/common"
)

// tail file manager

var (
	ttMgr *tailTaskMgr
)

type tailTaskMgr struct {
	tailTaskMap      map[string]*tailObj
	collectEntryList []*common.CollectEntry
	newConfChan      <-chan []*common.CollectEntry
}

// Init 初始化管理类
// 根据日志收集的配置项创建对应的tailObj实例
func Init(collectEntryList []*common.CollectEntry, confChan <-chan []*common.CollectEntry) (err error) {
	log.Debug("???")
	ttMgr = &tailTaskMgr{
		collectEntryList: collectEntryList,
		tailTaskMap:      make(map[string]*tailObj, 32),
		newConfChan:      confChan,
	}
	log.Debug("tailfile_mgr:Init")
	// 遍历配置项启动tailObj
	for _, conf := range collectEntryList {
		log.Debugf("current conf:%#v", conf)
		// 去重
		if ttMgr.exist(conf.Path) {
			log.Warnf("the log of path:%s is collecting already")
			continue
		}
		log.Debugf("start to create a tailObj for collect path:%s", conf.Path)
		tObj, err := NewTailObj(conf.Path, conf.Module, conf.Topic)
		if err != nil {
			log.Errorf("create tailObj for %s failed, err:%v", conf.Path, err)
			continue
		}
		go tObj.run() // 干活去吧
		ttMgr.tailTaskMap[conf.Path] = tObj
		log.Debugf("create tailObj for %s success", conf.Path)
	}
	go ttMgr.run()
	return
}

// exist 判断是否已经有一个tailObj在收集对应path的日志
func (t *tailTaskMgr) exist(path string) (isExist bool) {
	for k := range t.tailTaskMap {
		if k == path {
			isExist = true
			break
		}
	}
	return
}

// 监控新的日志配置项
func (t *tailTaskMgr) run() {
	log.Debug("wait conf change from newConf")
	for {
		newConfList := <-t.newConfChan
		log.Debugf("new conf is comming, newConf:%v", newConfList)
		// 有了新配置过来，分为几种情况
		for _, newConf := range newConfList {
			// 1. 之前已经存在的就不做处理
			if t.exist(newConf.Path) {
				log.Debugf("the task of path:%s is already run", newConf.Path)
				continue
			}
			// 2. 新的配置项之前没有只需要启动一个tailObj实例即可。
			tObj, err := NewTailObj(newConf.Path, newConf.Module, newConf.Topic)
			if err != nil {
				log.Errorf("create tail task for path:%s failed, err:%v", newConf.Path, err)
				continue
			}
			log.Debugf("create new tail task for path:%s", newConf.Path)
			go tObj.run()
			t.tailTaskMap[newConf.Path] = tObj
		}
		// 3. 新的配置项中没有的，我们就应该取消掉对应的taskObj
		for k := range t.tailTaskMap {
			isFound := false
			for _, nc := range newConfList {
				if k == nc.Path {
					isFound = true
					break
				}
			}
			if !isFound {
				// 该配置项在最新的配置中找不到，就应该取消
				t.tailTaskMap[k].cancel() // 通知goroutine退出
				log.Debugf("the task of path:%s is remove from tailTaskMap", k)
				delete(t.tailTaskMap, k) // 删除该配置项
			}
		}
	}
}
