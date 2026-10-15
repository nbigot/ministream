package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	json "github.com/goccy/go-json"
)

// report prints markdown tables with the median of the runs of each server build.
func report(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	order := fs.String("impls", "", "comma separated names of the builds: order of the columns (default: alphabetical)")
	_ = fs.Parse(args)
	files := fs.Args()

	type key struct{ storage, impl string }
	results := map[key][]*Result{}
	var impls []string
	storages := []string{"inmemory", "jsonfile", "mysql"}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		r := &Result{}
		if err := json.Unmarshal(data, r); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		k := key{r.Storage, r.Impl}
		if !contains(impls, r.Impl) {
			impls = append(impls, r.Impl)
		}
		results[k] = append(results[k], r)
	}
	sort.Strings(impls)
	if *order != "" {
		position := map[string]int{}
		for i, impl := range strings.Split(*order, ",") {
			position[strings.TrimSpace(impl)] = i + 1
		}
		// builds missing from the list come last
		rank := func(impl string) int {
			if p, ok := position[impl]; ok {
				return p
			}
			return len(position) + 1
		}
		sort.SliceStable(impls, func(i, j int) bool { return rank(impls[i]) < rank(impls[j]) })
	}

	for _, storage := range storages {
		var cols []string
		for _, impl := range impls {
			if len(results[key{storage, impl}]) > 0 {
				cols = append(cols, impl)
			}
		}
		if len(cols) == 0 {
			continue
		}
		fmt.Printf("\n## %s\n\n", storage)
		runs := []string{}
		for _, c := range cols {
			runs = append(runs, fmt.Sprintf("%s: %d", c, len(results[key{storage, c}])))
		}
		fmt.Printf("Runs: %s. Median of the runs, spread (max-min)/median in brackets when above 5%%.\n\n", strings.Join(runs, ", "))
		fmt.Printf("| metric | %s |\n|---|%s\n", strings.Join(cols, " | "), strings.Repeat("---:|", len(cols)))
		row := func(label, format string, get func(r *Result) (float64, bool)) {
			cells := []string{}
			any := false
			for _, c := range cols {
				var vals []float64
				for _, r := range results[key{storage, c}] {
					if v, ok := get(r); ok {
						vals = append(vals, v)
					}
				}
				if len(vals) == 0 {
					cells = append(cells, "")
					continue
				}
				any = true
				cells = append(cells, fmtMedian(format, vals))
			}
			if any {
				fmt.Printf("| %s | %s |\n", label, strings.Join(cells, " | "))
			}
		}
		phase := func(r *Result, name string) *Phase {
			for _, p := range r.Phases {
				if p.Name == name {
					return p
				}
			}
			return nil
		}
		row("startup (ms)", "%.0f", func(r *Result) (float64, bool) { return r.StartupMs, true })
		row("idle memory (MB)", "%.1f", func(r *Result) (float64, bool) { return r.IdleRSSMB, true })
		for _, name := range []string{"ping", "put-record", "put-records", "read", "read-jq"} {
			unit := "records/s"
			if name == "ping" {
				unit = "requests/s"
			}
			row(name+": "+unit, "%.0f", func(r *Result) (float64, bool) {
				p := phase(r, name)
				if p == nil {
					return 0, false
				}
				if name == "ping" {
					return p.ReqPerSec, true
				}
				return p.RecPerSec, true
			})
			if name == "put-record" || name == "put-records" {
				row(name+": records/s stored", "%.0f", func(r *Result) (float64, bool) {
					p := phase(r, name)
					if p == nil || p.PersistSec == 0 {
						return 0, false
					}
					return float64(p.Records) / p.PersistSec, true
				})
			}
			row(name+": latency p50 (ms)", "%.2f", func(r *Result) (float64, bool) {
				p := phase(r, name)
				return p.LatencyMs["p50"], p != nil
			})
			row(name+": latency p99 (ms)", "%.2f", func(r *Result) (float64, bool) {
				p := phase(r, name)
				return p.LatencyMs["p99"], p != nil
			})
			per := "1k records"
			if name == "ping" {
				per = "1k requests"
			}
			row(name+": server CPU ms per "+per, "%.1f", func(r *Result) (float64, bool) {
				p := phase(r, name)
				if p == nil {
					return 0, false
				}
				n := p.Records
				if name == "ping" {
					n = p.Requests
				}
				return p.ServerCPUSec * 1000 / (float64(n) / 1000), true
			})
			if storage == "mysql" && name != "ping" {
				row(name+": MySQL statements per 1k records", "%.0f", func(r *Result) (float64, bool) {
					p := phase(r, name)
					if p == nil || p.MySQL == nil {
						return 0, false
					}
					return float64(p.MySQL["Questions"]) * 1000 / float64(p.Records), true
				})
			}
			if name == "ping" || name == "put-record" {
				row(name+": TCP connections opened", "%.0f", func(r *Result) (float64, bool) {
					p := phase(r, name)
					return float64(p.Connections), p != nil
				})
			}
			row(name+": errors", "%.0f", func(r *Result) (float64, bool) {
				p := phase(r, name)
				return float64(p.Errors), p != nil
			})
		}
		row("peak memory (MB)", "%.1f", func(r *Result) (float64, bool) { return r.PeakRSSMB, true })
		row("final memory (MB)", "%.1f", func(r *Result) (float64, bool) { return r.FinalRSSMB, true })
		row("server CPU total (s)", "%.1f", func(r *Result) (float64, bool) { return r.TotalCPUSec, true })
		row("disk usage (MB)", "%.1f", func(r *Result) (float64, bool) { return r.DiskMB, r.DiskMB > 0 })
		row("binary size (MB)", "%.1f", func(r *Result) (float64, bool) { return r.BinaryMB, true })
	}
	return nil
}

func fmtMedian(format string, vals []float64) string {
	sort.Float64s(vals)
	med := vals[len(vals)/2]
	if len(vals)%2 == 0 {
		med = (vals[len(vals)/2-1] + vals[len(vals)/2]) / 2
	}
	s := fmt.Sprintf(format, med)
	if len(vals) > 1 && med > 0 {
		spread := (vals[len(vals)-1] - vals[0]) / med * 100
		if spread > 5 {
			s += fmt.Sprintf(" (%.0f%%)", spread)
		}
	}
	return s
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
