#!/usr/bin/env python3
"""Download historical FMI open-data observations for a station into CSV.

Usage:
  fmi_obs_history.py [--resolution daily|hourly|10min] [--station NAME|FMISID ...]
                     [--start YYYY-MM-DD] [--end YYYY-MM-DD] [--out DIR]

Output: <out>/<station>_<resolution>.csv with one row per timestamp (UTC) and
one column per FMI parameter. Missing values (FMI NaN) are left empty.
"""

import argparse
import csv
import datetime as dt
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
from concurrent.futures import ThreadPoolExecutor

WFS = "https://opendata.fmi.fi/wfs"
NS = {"BsWfs": "http://xml.fmi.fi/schema/wfs/2.0"}

STATIONS = {
    "kaisaniemi": 100971,
    "kumpula": 101004,
}

STATION_START = {
    "kaisaniemi": dt.date(1844, 1, 1),
    "kumpula": dt.date(2005, 1, 1),
}

RESOLUTIONS = {
    "daily": ("fmi::observations::weather::daily::simple", dt.timedelta(days=365)),
    "hourly": ("fmi::observations::weather::hourly::simple", dt.timedelta(days=31)),
    "10min": ("fmi::observations::weather::simple", dt.timedelta(days=7)),
}

EARLIEST = {
    "daily": dt.date(1829, 1, 1),
    "hourly": dt.date(1986, 1, 1),
    "10min": dt.date(2010, 1, 1),
}


def fetch(query, fmisid, start, end, retries=6):
    params = {
        "service": "WFS",
        "version": "2.0.0",
        "request": "getFeature",
        "storedquery_id": query,
        "fmisid": fmisid,
        "starttime": start.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "endtime": end.strftime("%Y-%m-%dT%H:%M:%SZ"),
    }
    url = WFS + "?" + urllib.parse.urlencode(params)
    for attempt in range(retries):
        try:
            with urllib.request.urlopen(url, timeout=120) as resp:
                return resp.read()
        except urllib.error.HTTPError as e:
            body = e.read().decode(errors="replace")
            if "Too long time interval" in body or "out of the allowed range" in body:
                raise RuntimeError(f"{start:%Y-%m-%d}..{end:%Y-%m-%d}: {body[:300]}")
            if e.code < 500 and e.code != 429:
                raise RuntimeError(f"HTTP {e.code} for {url}: {body[:300]}")
        except (urllib.error.URLError, TimeoutError, ConnectionError):
            pass
        time.sleep(2**attempt)
    raise RuntimeError(f"giving up on {start:%Y-%m-%d}..{end:%Y-%m-%d}")


def parse(data, resolution):
    rows = {}
    root = ET.fromstring(data)
    for el in root.iter("{%s}BsWfsElement" % NS["BsWfs"]):
        t = el.find("BsWfs:Time", NS).text
        if resolution == "daily":
            t = t[:10]
        name = el.find("BsWfs:ParameterName", NS).text
        val = el.find("BsWfs:ParameterValue", NS).text
        if val is None or val == "NaN":
            val = ""
        row = rows.setdefault(t, {})
        if row.get(name, "") == "":
            row[name] = val
    return rows


def chunks(start, end, step):
    cur = start
    while cur < end:
        nxt = min(cur + step, end)
        yield cur, nxt
        cur = nxt


def download(station, fmisid, resolution, start, end, out_dir, workers):
    query, step = RESOLUTIONS[resolution]
    spans = list(chunks(start, end, step))
    rows = {}

    def one(span):
        return parse(fetch(query, fmisid, *span), resolution)

    done = 0
    with ThreadPoolExecutor(workers) as ex:
        for part in ex.map(one, spans):
            for t, vals in part.items():
                rows.setdefault(t, {}).update({k: v for k, v in vals.items() if v != ""})
            done += 1
            if done % 20 == 0 or done == len(spans):
                print(f"{station} {resolution}: {done}/{len(spans)} requests, {len(rows)} rows", file=sys.stderr)

    columns = sorted({k for r in rows.values() for k in r})
    path = f"{out_dir}/{station}_{resolution}.csv"
    with open(path, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["time"] + columns)
        for t in sorted(rows):
            w.writerow([t] + [rows[t].get(c, "") for c in columns])
    print(f"wrote {path}: {len(rows)} rows, columns {columns}", file=sys.stderr)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--resolution", choices=RESOLUTIONS, default="daily")
    ap.add_argument("--station", action="append", help="station name or FMISID (default: kaisaniemi, kumpula)")
    ap.add_argument("--start", type=dt.date.fromisoformat)
    ap.add_argument("--end", type=dt.date.fromisoformat, default=dt.date.today())
    ap.add_argument("--out", default="data/fmi-observations")
    ap.add_argument("--workers", type=int, default=3)
    args = ap.parse_args()

    stations = args.station or list(STATIONS)
    os.makedirs(args.out, exist_ok=True)
    for s in stations:
        fmisid = STATIONS.get(s) or int(s)
        name = s if s in STATIONS else str(fmisid)
        start = args.start or max(EARLIEST[args.resolution], STATION_START.get(name, dt.date.min))
        t0 = dt.datetime.combine(start, dt.time.min)
        t1 = dt.datetime.combine(args.end, dt.time.min)
        download(name, fmisid, args.resolution, t0, t1, args.out, args.workers)


if __name__ == "__main__":
    main()
