package sysinfo

import (
	"encoding/json"
	"fmt"
	"logagent/kafka"
	"time"

	"github.com/prometheus/common/log"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

var (
	lastNetIOStatTimeStamp int64    // 上一次获取网络IO数据的时间点
	lastNetInfo            *NetInfo // 上一次的网络IO数据
	collectSysInfoTopic    string
)

// 收集系统信息汇报到kafka

func getCpuInfo() {
	var cpuInfo = new(CpuInfo)
	// CPU使用率
	percent, _ := cpu.Percent(time.Second, false)
	fmt.Printf("cpu percent:%v\n", percent)
	// 写入到influxDB中
	cpuInfo.CpuPercent = percent[0]
	sendKafka(cpuInfo)
}

func getMemInfo() {
	var memInfo = new(MemInfo)
	info, err := mem.VirtualMemory()
	if err != nil {
		fmt.Printf("get mem info failed, err:%v", err)
		return
	}
	memInfo.Total = info.Total
	memInfo.Available = info.Available
	memInfo.Used = info.Used
	memInfo.UsedPercent = info.UsedPercent
	memInfo.Buffers = info.Buffers
	memInfo.Cached = info.Cached
	sendKafka(memInfo)
}

func getDiskInfo() {
	var diskInfo = &DiskInfo{
		PartitionUsageStat: make(map[string]*UsageStat, 16),
	}
	parts, _ := disk.Partitions(true)
	for _, part := range parts {
		// 拿到每一个分区
		usageStatInfo, err := disk.Usage(part.Mountpoint) // 传挂载点
		if err != nil {
			fmt.Printf("get %s usage stat failed, err:%v", err)
			continue
		}
		usageStat := &UsageStat{
			Path:              usageStatInfo.Path,
			Fstype:            usageStatInfo.Fstype,
			Total:             usageStatInfo.Total,
			Free:              usageStatInfo.Free,
			Used:              usageStatInfo.Used,
			UsedPercent:       usageStatInfo.UsedPercent,
			InodesTotal:       usageStatInfo.InodesTotal,
			InodesUsed:        usageStatInfo.InodesUsed,
			InodesFree:        usageStatInfo.InodesFree,
			InodesUsedPercent: usageStatInfo.InodesUsedPercent,
		}
		diskInfo.PartitionUsageStat[part.Mountpoint] = usageStat
	}
	sendKafka(diskInfo)
}

func getNetInfo() {
	var netInfo = &NetInfo{
		NetIOCountersStat: make(map[string]*IOStat, 8),
	}
	currentTimeStamp := time.Now().Unix()
	netIOs, err := net.IOCounters(true)
	if err != nil {
		fmt.Printf("get net io counters failed, err:%v", err)
		return
	}
	for _, netIO := range netIOs {
		var ioStat = new(IOStat)
		ioStat.BytesSent = netIO.BytesSent
		ioStat.BytesRecv = netIO.BytesRecv
		ioStat.PacketsSent = netIO.PacketsSent
		ioStat.PacketsRecv = netIO.PacketsRecv
		// 将具体网卡数据的ioStat变量添加到map中
		netInfo.NetIOCountersStat[netIO.Name] = ioStat // 不要放到continue下面

		// 开始计算网卡相关速率
		if lastNetIOStatTimeStamp == 0 || lastNetInfo == nil {
			continue
		}
		// 计算时间间隔
		interval := currentTimeStamp - lastNetIOStatTimeStamp
		// 计算速率
		ioStat.BytesSentRate = (float64(ioStat.BytesSent) - float64(lastNetInfo.NetIOCountersStat[netIO.Name].BytesSent)) / float64(interval)
		ioStat.BytesRecvRate = (float64(ioStat.BytesRecv) - float64(lastNetInfo.NetIOCountersStat[netIO.Name].BytesRecv)) / float64(interval)
		ioStat.PacketsSentRate = (float64(ioStat.PacketsSent) - float64(lastNetInfo.NetIOCountersStat[netIO.Name].PacketsSent)) / float64(interval)
		ioStat.PacketsRecvRate = (float64(ioStat.PacketsRecv) - float64(lastNetInfo.NetIOCountersStat[netIO.Name].PacketsRecv)) / float64(interval)

	}
	// 更新全局记录的上一次采集网卡的时间点和网卡数据
	lastNetIOStatTimeStamp = currentTimeStamp // 更新时间
	lastNetInfo = netInfo
	sendKafka(netInfo)
}

func sendKafka(data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	kafkaMsg := &kafka.Message{
		Topic: collectSysInfoTopic,
		Data:  string(jsonData),
	}
	err = kafka.SendLog(kafkaMsg)
	if err != nil {
		log.Warnf("send collect sys info msg failed, err:%v\n", err)
		return
	}
	log.Info("send collect sys info msg")

}
func Run(interval time.Duration, topic string) {
	collectSysInfoTopic = topic
	ticker := time.Tick(interval)
	for _ = range ticker {
		getCpuInfo()
		getMemInfo()
		getDiskInfo()
		getNetInfo()
	}
}
