package common

// common

type CollectEntry struct {
	Path   string `json:"path"`
	Module string `json:"module"`
	Topic  string `json:"topic"`
}


type CollectSysInfoConfig struct {
	Interval int64 `json:"interval"`
	Topic string `json:"topic"`
}