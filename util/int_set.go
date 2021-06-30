// Copyright (C) 2021 Toitware ApS. All rights reserved.
// Use of this source code is governed by an MIT-style license that can be
// found in the LICENSE file.

package util

type IntSet map[int]struct{}

func NewIntSet(vals ...int) IntSet {
	res := IntSet{}
	for _, i := range vals {
		res[i] = struct{}{}
	}
	return res
}

func (s *IntSet) UnmarshalYAML(unmarshal func(interface{}) error) error {
	l := []int{}

	if err := unmarshal(&l); err != nil {
		return err
	}

	s.Add(l...)
	return nil
}

func (s IntSet) MarshalYAML() (interface{}, error) {
	return s.Values(), nil
}

func (s *IntSet) Add(vals ...int) {
	if *s == nil {
		*s = IntSet{}
	}

	for _, i := range vals {
		(*s)[i] = struct{}{}
	}
}

func (s IntSet) Remove(vals ...int) {
	if s == nil {
		return
	}

	for _, i := range vals {
		delete(s, i)
	}
}

func (s IntSet) Contains(i int) bool {
	if s == nil {
		return false
	}

	_, exists := s[i]
	return exists
}

func (s IntSet) Values() []int {
	var res []int
	if s == nil {
		return res
	}

	for k := range s {
		res = append(res, k)
	}
	return res
}
