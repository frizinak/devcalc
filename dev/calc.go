package dev

import "fmt"

type Result struct {
	ChemVolume  float64
	ChemWeight  float64
	WaterVolume float64
}

func (r Result) String() string {
	if r.ChemWeight == 0 && r.ChemVolume != 0 {
		return fmt.Sprintf(
			"%.2fml + %2.fml = %.2fml",
			r.ChemVolume,
			r.WaterVolume,
			r.ChemVolume+r.WaterVolume,
		)
	}

	return fmt.Sprintf(
		"%.2fml (%.2fg) + %2.fml = %.2fml (%.2fg)",
		r.ChemVolume,
		r.ChemWeight,
		r.WaterVolume,
		r.ChemVolume+r.WaterVolume,
		r.ChemWeight+r.WaterVolume,
	)
}

func Calc(c Chem, volume float64) Result {
	var r Result
	r.ChemVolume = c.Volume(volume)
	r.ChemWeight = r.ChemVolume * c.Density()
	r.WaterVolume = volume - r.ChemVolume

	return r
}
