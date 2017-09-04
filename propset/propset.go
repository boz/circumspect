package propset

// todo: yadda yadda yadda

import (
	"fmt"
	"strconv"
)

type PropSet map[string]Property

type Property interface {
	String() string
}

func New() PropSet {
	return PropSet(make(map[string]Property))
}

func (ps PropSet) Add(name string, p Property) PropSet {
	ps[name] = p
	return ps
}

func (ps PropSet) AddMap(name string, p map[string]string) PropSet {
	return ps.Add(name, Map(p))
}

func (ps PropSet) AddString(name string, p string) PropSet {
	return ps.Add(name, String(p))
}

func (ps PropSet) AddInt(name string, p int) PropSet {
	return ps.Add(name, Int(p))
}

func (ps PropSet) Merge(other PropSet) PropSet {
	for k, v := range other {
		ps.Add(k, v)
	}
	return ps
}

type Map map[string]string

func (p Map) String() string {
	return fmt.Sprintf("%#v", (map[string]string)(p))
}

type String string

func (p String) String() string {
	return string(p)
}

type Int int

func (p Int) String() string {
	return strconv.Itoa(int(p))
}
