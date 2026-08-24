package api

import (
	"encoding/json"
	"net/http"
	"time"

	"wby/internal/weather"
)

// Frame cadences offered to the map scrubber. Frames are nominal (derived
// from the layer's publish cadence, not from upstream availability); clients
// snap their time params to these instants so overlay URLs stay stable and
// cacheable across scrubs. Replacing this with real availability (e.g. the
// WMS GetCapabilities time dimension) only changes this endpoint.
const (
	temperatureFrameStep  = time.Hour
	temperatureFrameHours = weather.MapForecastHorizon // future hours offered, backed by the GRIB field

	precipitationFrameStep = 5 * time.Minute
	precipitationFrameSpan = time.Hour // ± around now, matching the official FMI app's radar window

	precipitation12hFrameStep  = time.Hour
	precipitation12hFrameHours = 12 // future hours offered, backed by the Harmonie GRIB field
)

type frameSetJSON struct {
	// Times is the sorted list of frame instants. The entry at NowIndex stands
	// for the live "now" frame: clients render it by omitting the time param
	// (server-side latest + fallback logic) rather than sending the instant.
	Times       []string `json:"times"`
	NowIndex    int      `json:"now_index"`
	StepSeconds int      `json:"step_seconds"`
}

type mapFramesJSON struct {
	GeneratedAt      string       `json:"generated_at"`
	Temperature      frameSetJSON `json:"temperature"`
	Precipitation    frameSetJSON `json:"precipitation"`
	Precipitation12h frameSetJSON `json:"precipitation12h"`
}

func (h *Handler) getMapFrames(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	json.NewEncoder(w).Encode(buildMapFrames(time.Now()))
}

func buildMapFrames(now time.Time) mapFramesJSON {
	nowUTC := now.UTC()

	tempBase := nowUTC.Truncate(temperatureFrameStep)
	tempTimes := make([]string, 0, temperatureFrameHours+1)
	for i := 0; i <= temperatureFrameHours; i++ {
		tempTimes = append(tempTimes, tempBase.Add(time.Duration(i)*temperatureFrameStep).Format(time.RFC3339))
	}

	precipBase := nowUTC.Truncate(precipitationFrameStep)
	precipSteps := int(precipitationFrameSpan / precipitationFrameStep)
	precipTimes := make([]string, 0, 2*precipSteps+1)
	for i := -precipSteps; i <= precipSteps; i++ {
		precipTimes = append(precipTimes, precipBase.Add(time.Duration(i)*precipitationFrameStep).Format(time.RFC3339))
	}

	precip12hBase := nowUTC.Truncate(precipitation12hFrameStep)
	precip12hTimes := make([]string, 0, precipitation12hFrameHours+1)
	for i := 0; i <= precipitation12hFrameHours; i++ {
		precip12hTimes = append(precip12hTimes, precip12hBase.Add(time.Duration(i)*precipitation12hFrameStep).Format(time.RFC3339))
	}

	return mapFramesJSON{
		GeneratedAt: nowUTC.Format(time.RFC3339),
		Temperature: frameSetJSON{
			Times:       tempTimes,
			NowIndex:    0,
			StepSeconds: int(temperatureFrameStep.Seconds()),
		},
		Precipitation: frameSetJSON{
			Times:       precipTimes,
			NowIndex:    precipSteps,
			StepSeconds: int(precipitationFrameStep.Seconds()),
		},
		Precipitation12h: frameSetJSON{
			Times:       precip12hTimes,
			NowIndex:    0,
			StepSeconds: int(precipitation12hFrameStep.Seconds()),
		},
	}
}
