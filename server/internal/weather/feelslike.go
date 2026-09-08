package weather

import "math"

// FeelsLike returns FMI's apparent temperature (the FeelsLike parameter of
// their newbase library) from air temperature (°C), wind speed (m/s),
// relative humidity (%) and global radiation (W/m²). Wind cools at every
// temperature relative to 37 °C, humidity adds the summer simmer index above
// 14.5 °C, and radiation warms when known. Missing humidity counts as 50 %,
// which leaves the humidity term neutral; missing wind returns the air
// temperature.
func FeelsLike(temp, wind, humidity, radiation *float64) *float64 {
	if temp == nil || wind == nil {
		return temp
	}
	t := *temp
	w := math.Max(*wind, 0)

	const a, t0 = 15.0, 37.0
	chill := a + (1-a/t0)*t + a/t0*math.Pow(w+1, 0.16)*(t-t0)

	rh := 50.0
	if humidity != nil {
		rh = *humidity
	}
	heat := summerSimmerIndex(rh, t)

	feels := t + (chill - t) + (heat - t)
	if radiation != nil {
		const absorption = 0.07
		feels += 0.7*absorption*math.Max(*radiation, 0)/(w+10) - 0.25
	}
	return &feels
}

// FeelsLike is the observation's apparent temperature, using the station's
// global radiation when it reports one.
func (o Observation) FeelsLike() *float64 {
	var rad *float64
	for _, key := range []string{"glob_1min", "radiationglobal", "globalradiation"} {
		if v, ok := o.ExtraNumericParams[key]; ok {
			rad = &v
			break
		}
	}
	return FeelsLike(o.Temperature, o.WindSpeed, o.Humidity, rad)
}

func summerSimmerIndex(rh, t float64) float64 {
	const simmerLimit = 14.5
	if t <= simmerLimit {
		return t
	}
	const rhRef = 0.5
	r := rh / 100
	return (1.8*t - 0.55*(1-r)*(1.8*t-26) - 0.55*(1-rhRef)*26) / (1.8 * (1 - 0.55*(1-rhRef)))
}
