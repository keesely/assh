package kiris

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
)

type K_VMaps struct {
	data map[string]*K_VElement
}

type K_VElement struct {
	Key   string
	Value interface{}
	Attr  map[string]interface{}
}

func NewK_VMaps() *K_VMaps {
	return &K_VMaps{make(map[string]*K_VElement)}
}

func (this *K_VMaps) Load(dbFile string) (*K_VMaps, error) {
	if dbFile == "" {
		return nil, fmt.Errorf("%s", "Invalid db file path")
	}
	dbFile = RealPath(dbFile)
	file, err := os.Open(dbFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dec := gob.NewDecoder(file)
	err = dec.Decode(&this.data)
	if err != nil {
		return nil, err
	}
	return this, nil
}

func (this *K_VMaps) Save(dbFile string) error {
	if dbFile == "" {
		return fmt.Errorf("%s", "Invalid db file path")
	}
	var GolngGob bytes.Buffer
	gob.Register(map[string]K_VMaps{})
	gob.Register(K_VElement{})

	enc := gob.NewEncoder(&GolngGob)

	err := enc.Encode(this.data)
	if err != nil {
		return err
	}
	dbFile = RealPath(dbFile)
	ioutil.WriteFile(dbFile, GolngGob.Bytes(), 0644)
	return nil
}

func (this *K_VMaps) Set(key string, value interface{}) *K_VElement {
	if "" == key {
		return nil
	}
	element := &K_VElement{}
	if ok := this.Lookup(key); ok != nil {
		element = this.data[key]
	}
	element.Key = key
	element.Value = value
	this.data[key] = element
	return this.data[key]
}

func (this *K_VMaps) Del(key string) bool {
	if "" == key || nil == this.Lookup(key) {
		return false
	}
	delete(this.data, key)
	return true
}

func (this *K_VMaps) Lookup(key string) *K_VElement {
	if val, ok := this.data[key]; ok {
		return val
	}
	return nil
}

func (this *K_VMaps) Get(key string) *K_VElement {
	return this.Lookup(key)
}

func (this *K_VMaps) GetValue(key string, def ...interface{}) interface{} {
	if element := this.Lookup(key); element != nil {
		return element.Value
	}
	if def != nil && len(def) > 0 {
		return def[0]
	}
	return nil
}

func (this *K_VMaps) Print() {
	fmt.Printf("%100s\n", StrPad("", "=", 100, KIRIS_STR_PAD_RIGHT))
	for key, val := range this.GetData() {
		fmt.Printf(" Key: %-30s Value: %v \n", key, val.Value)
	}
	fmt.Printf("%100s\n", StrPad("", "=", 100, KIRIS_STR_PAD_RIGHT))
}

func (this *K_VMaps) GetData() map[string]*K_VElement {
	return this.data
}

func (this *K_VElement) SetAttr(key string, value interface{}) *K_VElement {
	if nil == this.Attr {
		this.Attr = make(map[string]interface{})
	}
	this.Attr[key] = value
	return this
}

func (this *K_VElement) GetAttr(key string) interface{} {
	if val, ok := this.Attr[key]; ok {
		return val
	}
	return nil
}
