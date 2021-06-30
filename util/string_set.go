// Copyright (C) 2021 Toitware ApS. All rights reserved.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package util

type StringSet map[string]struct{}

func NewStringSet(strs ...string) StringSet {
	res := StringSet{}
	for _, s := range strs {
		res[s] = struct{}{}
	}
	return res
}

func (s *StringSet) UnmarshalYAML(unmarshal func(interface{}) error) error {
	l := []string{}

	if err := unmarshal(&l); err != nil {
		return err
	}

	s.Add(l...)
	return nil
}

func (s StringSet) MarshalYAML() (interface{}, error) {
	return s.Values(), nil
}

func (s *StringSet) Add(strs ...string) {
	if *s == nil {
		*s = StringSet{}
	}

	for _, str := range strs {
		(*s)[str] = struct{}{}
	}
}

func (s StringSet) Remove(strs ...string) {
	if s == nil {
		return
	}

	for _, str := range strs {
		delete(s, str)
	}
}

func (s StringSet) Contains(str string) bool {
	if s == nil {
		return false
	}

	_, exists := s[str]
	return exists
}

func (s StringSet) Values() []string {
	var res []string
	if s == nil {
		return res
	}

	for k := range s {
		res = append(res, k)
	}
	return res
}
