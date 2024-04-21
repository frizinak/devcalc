package dev

import (
	"fmt"
	"strconv"
	"strings"
)

type Chem interface {
	Density() float64
	Volume(float64) float64
}

type Simple struct {
	density float64
	ratio   float64
}

func (s Simple) Density() float64         { return s.density }
func (s Simple) Volume(v float64) float64 { return s.ratio * v }

func ScaleParts(scale string) (values [2]int) {
	p := strings.FieldsFunc(scale, func(r rune) bool {
		return r == ':' || r == '/' || r == '+'
	})
	if len(p) > 2 {
		return
	}
	for i, n := range p {
		l, err := strconv.Atoi(n)
		if err != nil {
			panic(err)
		}
		if i > 1 {
			fmt.Println(scale)
		}
		values[i] = l
	}

	return
}

func ScaleRatio(scale string) float64 {
	v := ScaleParts(scale)

	return float64(v[0]) / float64(v[1]+v[0])
}

func ScaleString(values [2]int) string {
	return fmt.Sprintf("%d+%d", values[0], values[1])
}

func NewChem(density, ratio float64) Chem {
	return Simple{density, ratio}
}

type Stock struct {
	Name string
}
