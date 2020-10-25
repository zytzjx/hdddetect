package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type item struct {
	Key   string `xml:"key,attr"`
	Value string `xml:"value,attr"`
}

type config struct {
	CardList string `xml:"value,attr"`
	PortMap  []item `xml:"add"`
}

type configuration struct {
	Conf config `xml:"mapconf"`
}

// Split rune
func Split(r rune) bool {
	return r == ',' || r == ';' || r == ':'
}

func (cc config) GetCardListIndex() ([]int, error) {
	var myslice []int
	if len(cc.CardList) > 0 {
		patts := strings.FieldsFunc(cc.CardList, Split)
		myslice := make([]int, len(patts))
		var i int = 0

		for _, patt := range patts {
			index, err := strconv.Atoi(patt)
			if err == nil {
				myslice[i] = index
				i++
			}
		}
		return myslice[:i], nil
	}
	return myslice, errors.New("card list is empty")
}

// GetPortMap get map
func (cc config) GetPortMap() map[string]string {
	labmap := make(map[string]string)
	for _, vv := range cc.PortMap {
		labmap[vv.Key] = vv.Value
	}
	return labmap
}

var configxmldata *configuration

// LoadConfigXML load xml
func LoadConfigXML() {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
		return
	}

	configxmlpath := path.Join(dir, "appconf.xml")

	xmlFile, err := os.Open(configxmlpath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		//xmlFile = []byte(xmlstr)
		return
	}
	defer xmlFile.Close()

	b, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		log.Fatal(err)
		return
	}
	configxmldata = &configuration{}
	err = xml.Unmarshal(b, configxmldata)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(configxmldata)

}
