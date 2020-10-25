package main

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"log"
	"os/exec"
	"regexp"
)

//var SASHDDMapData map[int]([]map[string]string)

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

var ingoreKeys = map[string]bool{
	"Device is":        true,
	"Local Time is":    true,
	"SMART support is": true,
	"Warning":          true,
}

// ReadDataFromSmartCtl Read Data From SmartCtl
func (sdd *SyncDataDetect) ReadDataFromSmartCtl(wg *sync.WaitGroup) {
	defer wg.Done()
	//sDevName string, hdinfo *DataDetect
	sDevName := sdd.detectHDD.LinuxName
	hdinfo := sdd.detectHDD
	if !strings.HasPrefix(sDevName, `/dev/`) {
		return
	}
	bOKDisk := false

	if sdd.IsRunning {
		return
	}
	sdd.SetRunning()
	defer sdd.CleanRunning()

	sdd.lock.Lock()
	defer sdd.lock.Unlock()

	cmd := exec.Command("smartctl", sDevName, "-i", "-H")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	timer := time.AfterFunc(300*time.Second, func() {
		cmd.Process.Kill()
	})

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		ss := scanner.Text()
		fmt.Println(ss)
		keyvalue := strings.Split(ss, ":")
		if len(keyvalue) == 2 {
			keyvalue[0] = strings.Trim(keyvalue[0], " ")
			keyvalue[1] = strings.Trim(keyvalue[1], " ")
			if _, ok := ingoreKeys[keyvalue[0]]; ok {
				continue
			}
			if strings.EqualFold("Device Model", keyvalue[0]) {
				hdinfo.Model = keyvalue[1]
			} else if strings.EqualFold("Serial Number", keyvalue[0]) {
				hdinfo.Serialno = keyvalue[1]
			} else if strings.EqualFold("LU WWN Device Id", keyvalue[0]) {
				hdinfo.LuwwndevId = strings.Replace(keyvalue[1], " ", "", -1)
			} else if strings.EqualFold("Firmware Version", keyvalue[0]) {
				hdinfo.Version = keyvalue[1]
			} else if strings.EqualFold("Logical Unit id", keyvalue[0]) {
				v := strings.Replace(keyvalue[1], "-", "", -1)
				v = strings.Replace(v, "0x", "", -1)
				hdinfo.Otherinfo["LogicalUnitID"] = v
			} else if strings.EqualFold(hdinfo.Size, "-") && strings.EqualFold("User Capacity", keyvalue[0]) {
				match := regexp.MustCompile(`^.*?\[(.*?)\]$`)
				size := match.FindStringSubmatch(keyvalue[1])
				if len(size) > 1 {
					hdinfo.Size = strings.Replace(size[1], " ", "", -1)
				}
			} else {
				if strings.EqualFold(keyvalue[0], "SMART Status command failed") {
					hdinfo.Otherinfo["HD-health"] = keyvalue[1]
				} else if strings.EqualFold(keyvalue[0], "SMART overall-health self-assessment test result") ||
					strings.EqualFold(keyvalue[0], "SMART Health Status") ||
					strings.EqualFold(keyvalue[0], "Read SMART Data failed") {
					k := "HD-health"
					if _, ok := hdinfo.Otherinfo[k]; !ok {
						hdinfo.Otherinfo[k] = keyvalue[1]
					}
				} else {
					hdinfo.Otherinfo[keyvalue[0]] = keyvalue[1]
				}
			}
		}
	}
	timer.Stop()
	if v, ok := hdinfo.Otherinfo["HD-health"]; ok {
		if strings.EqualFold(v, "OK") || strings.EqualFold(v, "PASSED") {
			bOKDisk = true
		}
	}

	if bOKDisk {
		cmd = exec.Command("smartctl", sDevName, "-A")
		stdout, err = cmd.StdoutPipe()
		if err != nil {
			log.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}

		timer := time.AfterFunc(300*time.Second, func() {
			cmd.Process.Kill()
		})

		scanner := bufio.NewScanner(stdout)
		var badsectors int
		for scanner.Scan() {
			ss := scanner.Text()
			keyvalue := strings.Split(ss, ":")
			if len(keyvalue) == 2 {
				keyvalue[0] = strings.Trim(keyvalue[0], " ")
				keyvalue[1] = strings.Trim(keyvalue[1], " ")
				if _, ok := ingoreKeys[keyvalue[0]]; ok {
					continue
				}
				if strings.EqualFold("Elements in grown defect list", keyvalue[0]) {
					hdinfo.Otherinfo["badsectors"] = keyvalue[1]
				} else if strings.EqualFold("Non-medium error count", keyvalue[0]) {
					hdinfo.Otherinfo["Nonmediumerrorcnt"] = keyvalue[1]
				}
			} else {
				r := regexp.MustCompile(`^\s*(\d+)\s+(\S+)\s+(0x[0-9a-fA-F]{4})\s+(\d+)\s+(\d+)\s+(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s*`)
				fields := r.FindStringSubmatch(ss)
				if len(fields) == 11 {
					ATTRIBUTE_NAME := fields[2]
					RAW_VALUE := fields[10]
					hdinfo.Otherinfo[ATTRIBUTE_NAME] = RAW_VALUE

					if strings.EqualFold("Reallocated_Sector_Ct", ATTRIBUTE_NAME) ||
						strings.EqualFold("Current_Pending_Sector", ATTRIBUTE_NAME) {
						aa, err := strconv.Atoi(strings.Trim(RAW_VALUE, " "))
						if err == nil {
							badsectors += aa
						}
						hdinfo.Otherinfo["badsectors"] = strconv.Itoa(badsectors)
					}
				}
			}
		}

		timer.Stop()
	}
}

// waitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

// RunListDisk run list disks
func RunListDisk() {

	// dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	// if err != nil {
	// 	log.Fatal(err)
	// 	return
	// }

	// lsscsipath := path.Join(dir, "lsscsi")
	// if _, err := os.Stat(lsscsipath); os.IsNotExist(err) {
	// 	lsscsipath = "lsscsi"
	// }
	lsscsipath := "lsscsi"
	cmd := exec.Command(lsscsipath, "-s", "-g")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	timer := time.AfterFunc(10*time.Second, func() {
		cmd.Process.Kill()
	})

	scanner := bufio.NewScanner(stdout)
	var hddinfo []string
	var hddchanged bool
	var wg sync.WaitGroup
	for scanner.Scan() {
		ss := scanner.Text()
		fmt.Println(ss)
		hddinfo = append(hddinfo, ss)
		if !DetectData.MatchKey(ss) {
			hddchanged = true
		}
		if !DetectData.ContainsKey(ss) {
			//\s Matches any white-space character.
			r := regexp.MustCompile(`^([\s\S]{13})(disk[\s\S]{4})([\s\S]{9})([\s\S]{17})([\s\S]{6})([\s\S]{11})([\s\S]{11})([\s\S]+)$`)
			diskinfos := r.FindStringSubmatch(ss)
			if len(diskinfos) == 9 {
				var dddect = NewSyncDataDetect()
				dddect.detectHDD.Locpath = strings.Trim(diskinfos[1], " ")
				dddect.detectHDD.Type = strings.Trim(diskinfos[2], " ")
				dddect.detectHDD.Manufacture = strings.Trim(diskinfos[3], " ")
				dddect.detectHDD.Model = strings.Trim(diskinfos[4], " ")
				dddect.detectHDD.Version = strings.Trim(diskinfos[5], " ")
				dddect.detectHDD.LinuxName = strings.Trim(diskinfos[6], " ")
				dddect.detectHDD.SGLibName = strings.Trim(diskinfos[7], " ")
				dddect.detectHDD.Size = strings.Trim(diskinfos[8], " ")

				if strings.Index(dddect.detectHDD.LinuxName, `/dev/`) == -1 {
					continue
				}
				//hddchanged = true
				DetectData.AddValue(ss, dddect)
				wg.Add(1)
				go dddect.ReadDataFromSmartCtl(&wg)
			}
		} else {
			if vv, ok := DetectData.Get(ss); ok {
				if len(vv.detectHDD.UILabel) == 0 && len(vv.detectHDD.Otherinfo) == 0 {
					wg.Add(1)
					go vv.ReadDataFromSmartCtl(&wg)
				}
			}
		}
	}
	timer.Stop()
	DetectData.RemoveOld(hddinfo)

	time.Sleep(4 * time.Second)

	if hddchanged {
		fmt.Print("changed!")
		cclist, err := configxmldata.Conf.GetCardListIndex()
		if err == nil {
			for _, i := range cclist {
				wg.Add(1)
				go SASHDDinfo.RunCardInfo(i, &wg)
			}
		}
		for i := 0; i < 30; i++ {
			if waitTimeout(&wg, 10*time.Second) {
				fmt.Println("Timed out waiting for wait group")
				MergeCalibration()
			} else {
				fmt.Println("Wait group finished")
				MergeCalibration()
				break
			}
		}
	} else {
		waitTimeout(&wg, 300*time.Second)
	}

}

// SyncSASHDDMap sas hd map
type SyncSASHDDMap struct {
	lock          *sync.RWMutex
	SASHDDMapData map[int]([]map[string]string)
	ReadStatus    map[int]bool
}

// NewSyncSASHDDMap create structure
func NewSyncSASHDDMap() *SyncSASHDDMap {
	return &SyncSASHDDMap{lock: new(sync.RWMutex),
		SASHDDMapData: make(map[int]([]map[string]string)),
		ReadStatus:    make(map[int]bool)}
}

// ConDefine define disk info
var ConDefine = map[string]string{
	"Enclosure #":               "Enclosure",
	"Slot #":                    "Slot",
	"State":                     "State",
	"SAS Address":               "SASAddress",
	"Size (in MB)/(in sectors)": "Size",
	"Manufacturer":              "Manufacturer",
	"Model Number":              "Model",
	"Firmware Revision":         "Firmware",
	"Serial No":                 "Serial",
	"Unit Serial No(VPD)":       "SerialVPD",
	"GUID":                      "GUID",
	"Protocol":                  "Protocol",
	"Drive Type":                "DriveType",
}

func parseLsiData(lines []string) []map[string]string {
	var ret []map[string]string
	bFindDisk := false
	var item map[string]string
	for _, line := range lines {
		if strings.EqualFold(line, "Device is a Hard disk") {
			bFindDisk = true
			if len(item) > 0 {
				ret = append(ret, item)
			}
			item = make(map[string]string)
		} else if strings.EqualFold(line, "Enclosure information") {
			bFindDisk = false
			if len(item) > 0 {
				ret = append(ret, item)
			}
			break
		} else {
			if bFindDisk {
				pos := strings.Index(line, ":")
				if pos > 0 {
					k := strings.Trim(line[:pos], " ")
					v := strings.Trim(line[pos+1:], " ")

					if val, ok := ConDefine[k]; ok {
						if strings.EqualFold("Size", val) {
							pos = strings.Index(val, "/")
							if pos > 0 {
								item[val] = v[pos+1:]
							}
						} else {
							item[val] = v
						}

					}
				}
			}
		}
	}
	return ret
}

// ClearReadFlag read flag
func (sshm *SyncSASHDDMap) ClearReadFlag() {
	sshm.lock.Lock()
	defer sshm.lock.Unlock()
	for k := range sshm.ReadStatus {
		sshm.ReadStatus[k] = false
	}
}

// RunCardInfo   get card info
func (sshm *SyncSASHDDMap) RunCardInfo(index int, wg *sync.WaitGroup) {
	defer wg.Done()
	// dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	// if err != nil {
	// 	log.Fatal(err)
	// 	return
	// }

	// lsscsipath := path.Join(dir, "sas3ircu")
	// if _, err := os.Stat(lsscsipath); os.IsNotExist(err) {
	// 	lsscsipath = "sas2ircu"
	// }

	cmd := exec.Command("./sas2ircu", strconv.Itoa(index), "DISPLAY")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	timer := time.AfterFunc(600*time.Second, func() {
		cmd.Process.Kill()
	})

	scanner := bufio.NewScanner(stdout)
	var dynaArr []string
	for scanner.Scan() {
		ss := scanner.Text()
		fmt.Println(ss)
		dynaArr = append(dynaArr, ss)
	}
	timer.Stop()

	sshm.lock.Lock()
	defer sshm.lock.Unlock()
	if len(dynaArr) > 0 {
		sshm.SASHDDMapData[index] = parseLsiData(dynaArr)
		sshm.ReadStatus[index] = true
	}
}
