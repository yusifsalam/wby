package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"wby/internal/weather"
)

var dailyHeader = []string{"date", "temp_avg", "temp_high", "temp_low", "precip_mm", "snow_cm"}
var hourlyHeader = []string{"time", "temp", "humidity", "wind_speed", "wind_gust", "precip_mm"}

// Cache products, each a directory of one CSV per calendar year under the
// station directory. A year file is written after every fetch, empty when FMI
// returned nothing, so its presence alone means the year need not be fetched
// again.
const (
	dailyProduct         = "daily"
	hourlyProduct        = "hourly"
	instantHourlyProduct = "hourly-instant"
)

func yearCachePath(dir string, fmisid int, product string, year int) string {
	return filepath.Join(dir, strconv.Itoa(fmisid), product, strconv.Itoa(year)+".csv")
}

func yearRange(year int) (time.Time, time.Time) {
	return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(year, 12, 31, 23, 0, 0, 0, time.UTC)
}

// yearCache reads, writes and fetches one product's records a year at a time.
type yearCache[T any] struct {
	product string
	year    func(T) int
	read    func(path string) ([]T, bool, error)
	write   func(path string, records []T) error
	fetch   func(ctx context.Context, start, end time.Time) ([]T, error)
}

// missingYears lists the years in [startYear, endYear] without a cache file.
func (c yearCache[T]) missingYears(dir string, fmisid, startYear, endYear int) ([]int, error) {
	var missing []int
	for y := startYear; y <= endYear; y++ {
		_, err := os.Stat(yearCachePath(dir, fmisid, c.product, y))
		if errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, y)
		} else if err != nil {
			return nil, err
		}
	}
	return missing, nil
}

// load returns the records of every year in [startYear, endYear], reading
// cached years and fetching the rest one year at a time so a failure keeps
// what was already fetched.
func (c yearCache[T]) load(ctx context.Context, log *slog.Logger, dir string, fmisid, startYear, endYear int) ([]T, error) {
	var all []T
	cached, fetched := 0, 0
	t0 := time.Now()
	for y := startYear; y <= endYear; y++ {
		path := yearCachePath(dir, fmisid, c.product, y)
		records, ok, err := c.read(path)
		if err != nil {
			return nil, err
		}
		if ok {
			cached++
		} else {
			start, end := yearRange(y)
			if records, err = c.fetch(ctx, start, end); err != nil {
				return nil, fmt.Errorf("%s %d: %w", c.product, y, err)
			}
			if err := c.write(path, records); err != nil {
				return nil, fmt.Errorf("write %s cache: %w", c.product, err)
			}
			fetched++
		}
		all = append(all, records...)
	}
	log.Info("loaded "+c.product+" observations", "records", len(all), "cached_years", cached, "fetched_years", fetched, "took", time.Since(t0).Round(time.Second))
	return all, nil
}

// markEmpty writes an empty cache file for each year so later runs treat
// them as fetched.
func (c yearCache[T]) markEmpty(dir string, fmisid int, years []int) error {
	for _, y := range years {
		if err := c.write(yearCachePath(dir, fmisid, c.product, y), nil); err != nil {
			return fmt.Errorf("write %s cache: %w", c.product, err)
		}
	}
	return nil
}

// splitLegacy converts a per-period cache file (daily-YYYY-YYYY.csv and the
// hourly variants, from before the per-year layout) into year files for every
// year of its period, then removes it. Years already cached are left alone.
func (c yearCache[T]) splitLegacy(log *slog.Logger, dir string, fmisid int, path string, startYear, endYear int) error {
	records, ok, err := c.read(path)
	if err != nil || !ok {
		return err
	}
	byYear := make(map[int][]T)
	for _, r := range records {
		byYear[c.year(r)] = append(byYear[c.year(r)], r)
	}
	written := 0
	for y := startYear; y <= endYear; y++ {
		target := yearCachePath(dir, fmisid, c.product, y)
		if _, err := os.Stat(target); err == nil {
			continue
		}
		if err := c.write(target, byYear[y]); err != nil {
			return fmt.Errorf("write %s cache: %w", c.product, err)
		}
		written++
	}
	log.Info("split legacy cache into years", "path", path, "records", len(records), "years_written", written)
	return os.Remove(path)
}

var legacyCachePattern = regexp.MustCompile(`^(daily|hourly|hourly-instant)-(\d{4})-(\d{4})\.csv$`)

// migrateLegacyCache splits any per-period cache files in the station
// directory into the per-year layout.
func migrateLegacyCache(log *slog.Logger, dir string, fmisid int, daily yearCache[weather.DailyRecord], hourly, instant yearCache[weather.HourlyRecord]) error {
	entries, err := os.ReadDir(filepath.Join(dir, strconv.Itoa(fmisid)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		m := legacyCachePattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		startYear, _ := strconv.Atoi(m[2])
		endYear, _ := strconv.Atoi(m[3])
		path := filepath.Join(dir, strconv.Itoa(fmisid), e.Name())
		switch m[1] {
		case dailyProduct:
			err = daily.splitLegacy(log, dir, fmisid, path, startYear, endYear)
		case hourlyProduct:
			err = hourly.splitLegacy(log, dir, fmisid, path, startYear, endYear)
		case instantHourlyProduct:
			err = instant.splitLegacy(log, dir, fmisid, path, startYear, endYear)
		}
		if err != nil {
			return err
		}
	}
	return nil
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
