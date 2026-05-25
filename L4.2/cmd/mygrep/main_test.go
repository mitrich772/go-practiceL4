package main

import (
	"reflect"
	"testing"
)

func TestExpandShortFlags(t *testing.T) {
	cases := []struct {
		in, want []string
	}{
		{
			in:   []string{"-Fn", "-e", "ERROR", "file"},
			want: []string{"-F", "-n", "-e", "ERROR", "file"},
		},
		{
			in:   []string{"-Fnc", "-e", "x"},
			want: []string{"-F", "-n", "-c", "-e", "x"},
		},
		{
			// -e не булев — не трогаем.
			in:   []string{"-e", "pat", "-F"},
			want: []string{"-e", "pat", "-F"},
		},
		{
			// --servers оставляем.
			in:   []string{"--servers", "a:1,b:2", "-Fi"},
			want: []string{"--servers", "a:1,b:2", "-F", "-i"},
		},
		{
			// Группа с неизвестной буквой не разворачивается.
			in:   []string{"-Fz"},
			want: []string{"-Fz"},
		},
		{
			// Одиночные флаги не трогаются.
			in:   []string{"-F", "-n"},
			want: []string{"-F", "-n"},
		},
	}
	for _, tc := range cases {
		got := expandShortFlags(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("expandShortFlags(%v):\n got %v\nwant %v", tc.in, got, tc.want)
		}
	}
}
