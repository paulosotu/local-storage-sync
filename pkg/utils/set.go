package utils

type StringSet struct {
	values map[string]struct{}
}

func NewStringSet() *StringSet {
	return &StringSet{values: make(map[string]struct{})}
}

func (ss *StringSet) Add(val string) {
	ss.values[val] = struct{}{}
}

func (ss *StringSet) GetValues() []string {
	values := make([]string, len(ss.values))
	i := 0
	for k := range ss.values {
		values[i] = k
		i++
	}
	return values
}
