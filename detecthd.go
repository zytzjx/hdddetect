package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

type DataDetect struct {
	Locpath     string `json:"locpath"`
	Type        string `json:"sType"`
	Manufacture string `json:"sManufacture"`
	Model       string `json:"sModel"`
	Version     string `json:"sVersion"`
	LinuxName   string `json:"sLinuxName"`
	Size        string `json:"sSize"`
	SGLibName   string `json:"sSGLibName"`
	//use smartctl get info
	//public String model replace using this
	Serialno    string `json:"sSerialno"`
	LuwwndevId  string `json:"luwwndevId"`
	Calibration string `json:"sCalibration"`
	UILabel     string `json:"sUILabel"`
	//public String firware version replace by sVersion;
	Otherinfo map[string]string `json:"otherinfo"`
}

func NewDataDetect() *DataDetect {
	return &DataDetect{Otherinfo: make(map[string]string)}
}

func (dd *DataDetect) AddMap2Other(ddother map[string]string) {
	if len(ddother) == 0 {
		return
	}

	for kk, vv := range ddother {
		dd.Otherinfo[kk] = vv
	}
}

type SyncDataDetect struct {
	lock      *sync.RWMutex
	IsRunning bool
	detectHDD *DataDetect
}

func NewSyncDataDetect() *SyncDataDetect {
	return &SyncDataDetect{lock: new(sync.RWMutex), IsRunning: false, detectHDD: NewDataDetect()}
}

func (sdd *SyncDataDetect) SetRunning() {
	sdd.IsRunning = true
}

func (sdd *SyncDataDetect) CleanRunning() {
	sdd.IsRunning = false
}

func (sdd *SyncDataDetect) String() string {
	sdd.lock.Lock()
	defer sdd.lock.Unlock()

	jsonString, err := json.Marshal(sdd.detectHDD)
	if err != nil {
		return ""
	}
	return string(jsonString)
}

type SyncMap struct {
	lock     *sync.RWMutex
	dddetect map[string]*SyncDataDetect
}

func NewSyncMap() *SyncMap {
	return &SyncMap{lock: new(sync.RWMutex), dddetect: make(map[string]*SyncDataDetect)}
}

func (sm *SyncMap) MatchKey(key string) bool {
	var okret bool
	if len(key) == 0 {
		return okret
	}
	re := regexp.MustCompile(`^([\s\S]{13})(disk[\s\S]{4})([\s\S]{9})([\s\S]{17})([\s\S]{6})([\s\S]{11})([\s\S]{11})([\s\S]+)$`)
	if !re.MatchString(key) {
		return true
	}
	var sik = re.ReplaceAllString(key, `$1$2$3$4$5$7$8`)
	sm.lock.Lock()
	defer sm.lock.Unlock()

	for kk, _ := range sm.dddetect {
		sikk := re.ReplaceAllString(kk, `$1$2$3$4$5$7$8`)
		if strings.Compare(sik, sikk) == 0 {
			okret = true
			//fmt.Print(key)
			break
		}
	}

	return okret
}

func (sm *SyncMap) ContainsKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	sm.lock.Lock()
	defer sm.lock.Unlock()
	_, ok := sm.dddetect[key]
	return ok
}

func (sm *SyncMap) AddValue(key string, dd *SyncDataDetect) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	if dd == nil {
		delete(sm.dddetect, key)
	} else {
		sm.dddetect[key] = dd
	}

}

func (sm *SyncMap) RemoveOld(newkeylist []string) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	for kk, _ := range sm.dddetect {
		if !stringInSlice(kk, newkeylist) {
			delete(sm.dddetect, kk)
		}
	}

}

func (sm *SyncMap) Get(key string) (*SyncDataDetect, bool) {
	if len(key) == 0 {
		return nil, false
	}
	sm.lock.Lock()
	defer sm.lock.Unlock()
	vv, ok := sm.dddetect[key]
	return vv, ok
}

func (sm *SyncMap) Add(key string) *SyncDataDetect {
	if len(key) == 0 {
		return nil
	}
	sm.lock.Lock()
	defer sm.lock.Unlock()
	if vv, ok := sm.dddetect[key]; ok {
		return vv
	}
	sbc := NewSyncDataDetect()
	sm.dddetect[key] = sbc
	return sbc
}

func (sm *SyncMap) Remove(key string) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	delete(sm.dddetect, key)
}

func (sm *SyncMap) String() string {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	smddinfo := make(map[string]*DataDetect)
	for k, v := range sm.dddetect {
		smddinfo[k] = v.detectHDD
	}

	jsonString, err := json.Marshal(smddinfo)
	if err != nil {
		return ""
	}
	return string(jsonString)
}

var DetectData = NewSyncMap()
var SASHDDinfo = NewSyncSASHDDMap()

func MergeCalibration() {
	SASHDDinfo.lock.Lock()
	defer SASHDDinfo.lock.Unlock()
	DetectData.lock.Lock()
	defer DetectData.lock.Unlock()

	for index, sasmap := range SASHDDinfo.SASHDDMapData {
		if mm, ok := SASHDDinfo.ReadStatus[index]; ok {
			if !mm {
				continue
			}
		}
		SASHDDinfo.ReadStatus[index] = false
		for _, card := range sasmap {
			Serial, oks := card["Serial"]
			GUID, okg := card["GUID"]
			for _, v := range DetectData.dddetect {
				if len(v.detectHDD.Serialno) == 0 {
					continue
				}
				sserno := v.detectHDD.Serialno
				sserno = strings.Replace(sserno, "-", "", -1)
				var sLuID string
				var ok bool
				if sLuID, ok = v.detectHDD.Otherinfo["LogicalUnitID"]; !ok {
					sLuID = ""
				}
				if (oks && (strings.HasPrefix(sserno, Serial) || strings.HasPrefix(Serial, sserno))) ||
					(okg && (strings.EqualFold(GUID, v.detectHDD.LuwwndevId) || strings.EqualFold(GUID, sLuID))) {
					if slot, ok := card["Slot"]; ok {
						v.detectHDD.Calibration = fmt.Sprintf("%d_%s", index, slot)
					}
					if len(v.detectHDD.Calibration) > 0 {
						v.detectHDD.UILabel, _ = configxmldata.Conf.GetPortMap()[v.detectHDD.Calibration]
					}

					for kkk, vvv := range card {
						v.detectHDD.Otherinfo[kkk] = vvv
					}
				}
			}

		}
	}

}

func main() {
	fmt.Println("version: 20.10.25.0, auther:Jeffery Zhang")
	nDelay := flag.Int("interval", 10, "interval run check disk.")
	flag.Parse()

	LoadConfigXml()

	go func() {
		for {
			RunListDisk()

			MergeCalibration()
			//fmt.Println("interval:", *nDelay)
			time.Sleep(time.Duration(*nDelay) * time.Second)
		}
	}()

	time.Sleep(20 * time.Second)

	StartTCPServer()

	fmt.Println(DetectData.dddetect)
	//SASHDDinfo.RunCardInfo(1)
	fmt.Println(SASHDDinfo.SASHDDMapData)
}
