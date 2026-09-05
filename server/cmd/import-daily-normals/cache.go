package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"wby/internal/weather"
)

var dailyHeader = []string{"date", "temp_avg", "temp_high", "temp_low", "precip_mm", "snow_cm"}
var hourlyHeader = []string{"time", "temp", "humidity", "wind_speed", "wind_gust", "precip_mm"}

func dailyCachePath(dir string, fmisid int, period string) string {
	return filepath.Join(dir, strconv.Itoa(fmisid), "daily-"+period+".csv")
}

func hourlyCachePath(dir string, fmisid int, period string) string {
	return filepath.Join(dir, strconv.Itoa(fmisid), "hourly-"+period+".csv")
}

func instantHourlyCachePath(dir string, fmisid int, period string) string {
	return filepath.Join(dir, strconv.Itoa(fmisid), "hourly-instant-"+period+".csv")
}

func readDailyCSV(path string) ([]weather.DailyRecord, bool, error) {
	rows, err := readCSV(path, dailyHeader)
	if err != nil || rows == nil {
		return nil, false, err
	}
	records := make([]weather.DailyRecord, 0, len(rows))
	for _, row := range rows {
		date, err := time.Parse(time.RFC3339, row[0])
		if err != nil {
			return nil, false, fmt.Errorf("%s: parse date %q: %w", path, row[0], err)
		}
		records = append(records, weather.DailyRecord{
			Date:     date,
			TempAvg:  parseFloat(row[1]),
			TempHigh: parseFloat(row[2]),
			TempLow:  parseFloat(row[3]),
			PrecipMm: parseFloat(row[4]),
			SnowCm:   parseFloat(row[5]),
		})
	}
	return records, true, nil
}

func writeDailyCSV(path string, records []weather.DailyRecord) error {
	rows := make([][]string, 0, len(records))
	for _, r := range records {
		rows = append(rows, []string{
			r.Date.UTC().Format(time.RFC3339),
			formatFloat(r.TempAvg),
			formatFloat(r.TempHigh),
			formatFloat(r.TempLow),
			formatFloat(r.PrecipMm),
			formatFloat(r.SnowCm),
		})
	}
	return writeCSV(path, dailyHeader, rows)
}

func readHourlyCSV(path string) ([]weather.HourlyRecord, bool, error) {
	rows, err := readCSV(path, hourlyHeader)
	if err != nil || rows == nil {
		return nil, false, err
	}
	records := make([]weather.HourlyRecord, 0, len(rows))
	for _, row := range rows {
		t, err := time.Parse(time.RFC3339, row[0])
		if err != nil {
			return nil, false, fmt.Errorf("%s: parse time %q: %w", path, row[0], err)
		}
		records = append(records, weather.HourlyRecord{
			Time:      t,
			Temp:      parseFloat(row[1]),
			Humidity:  parseFloat(row[2]),
			WindSpeed: parseFloat(row[3]),
			WindGust:  parseFloat(row[4]),
			PrecipMm:  parseFloat(row[5]),
		})
	}
	return records, true, nil
}

func writeHourlyCSV(path string, records []weather.HourlyRecord) error {
	rows := make([][]string, 0, len(records))
	for _, r := range records {
		rows = append(rows, []string{
			r.Time.UTC().Format(time.RFC3339),
			formatFloat(r.Temp),
			formatFloat(r.Humidity),
			formatFloat(r.WindSpeed),
			formatFloat(r.WindGust),
			formatFloat(r.PrecipMm),
		})
	}
	return writeCSV(path, hourlyHeader, rows)
}

func readCSV(path string, header []string) ([][]string, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = len(header)
	r.ReuseRecord = false
	got, err := r.Read()
	if err == io.EOF {
		return nil, fmt.Errorf("%s: empty file", path)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: read header: %w", path, err)
	}
	for i := range header {
		if got[i] != header[i] {
			return nil, fmt.Errorf("%s: unexpected header %v", path, got)
		}
	}
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: read rows: %w", path, err)
	}
	if rows == nil {
		rows = [][]string{}
	}
	return rows, nil
}

func writeCSV(path string, header []string, rows [][]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		f.Close()
		return err
	}
	if err := w.WriteAll(rows); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func parseFloat(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func formatFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}
