package fmi

import (
	"context"
	"fmt"
	"io"
	"net/url"
)

// FetchGRIB downloads a GRIB2 grid for the given producer/params/bbox from the
// FMI Binary Download Service and streams the response body to w. The download
// endpoint returns the GRIB2 bytes directly (no WFS fileReference step), so the
// body can be large — callers should stream to a file rather than buffer it.
//
// bbox is "left,bottom,right,top" in lon/lat degrees (EPSG:4326). params is a
// comma-separated list of FMI download parameter names (e.g. "temperature").
func (c *Client) FetchGRIB(ctx context.Context, producer, params, bbox string, w io.Writer) error {
	q := url.Values{
		"producer":   {producer},
		"param":      {params},
		"format":     {"grib2"},
		"bbox":       {bbox},
		"projection": {"EPSG:4326"},
	}
	resp, err := c.get(ctx, "FMI download", c.downloadURL+"?"+q.Encode())
	if err != nil {
		return fmt.Errorf("fetch GRIB: %w", err)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("stream GRIB body: %w", err)
	}
	return nil
}
