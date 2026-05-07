#!/usr/bin/env python3
import argparse
import csv
import json
import os
from collections import defaultdict
from datetime import datetime, timezone


def parse_args():
    parser = argparse.ArgumentParser(
        description="Aggregate research outputs across all presets/scenarios"
    )
    parser.add_argument("--raw-root", required=True, help="Path to raw outputs root")
    parser.add_argument("--out-root", required=True, help="Path to research_data root")
    return parser.parse_args()


def ensure_dir(path):
    os.makedirs(path, exist_ok=True)


def read_csv(path):
    if not os.path.exists(path):
        return []
    with open(path, "r", encoding="utf-8") as f:
        return list(csv.DictReader(f))


def write_csv(path, fieldnames, rows):
    with open(path, "w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        for row in rows:
            writer.writerow(row)


def num(v):
    try:
        return float(v)
    except Exception:
        return None


def aggregate_mean(rows, metric, model):
    vals = [
        num(r.get("mean", ""))
        for r in rows
        if r.get("metric") == metric and r.get("model") == model
    ]
    vals = [v for v in vals if v is not None]
    if not vals:
        return None
    return sum(vals) / len(vals)


def aggregate_max(rows, metric, model):
    vals = [
        num(r.get("max", ""))
        for r in rows
        if r.get("metric") == metric and r.get("model") == model
    ]
    vals = [v for v in vals if v is not None]
    if not vals:
        return None
    return max(vals)


def fmt(v, scale=1.0, suffix=""):
    if v is None:
        return "n/a"
    return f"{(v / scale):.4f}{suffix}"


def main():
    args = parse_args()
    raw_root = args.raw_root
    out_root = args.out_root

    processed_dir = os.path.join(out_root, "processed")
    reports_dir = os.path.join(out_root, "reports")
    meta_dir = os.path.join(out_root, "_meta")
    ensure_dir(processed_dir)
    ensure_dir(reports_dir)
    ensure_dir(meta_dir)

    all_run_logs = []
    all_summary_rows = []
    scenarios_seen = []
    presets_seen = []

    if not os.path.isdir(raw_root):
        raise SystemExit(f"raw root does not exist: {raw_root}")

    for preset in sorted(os.listdir(raw_root)):
        preset_path = os.path.join(raw_root, preset)
        if not os.path.isdir(preset_path):
            continue
        presets_seen.append(preset)
        for scenario in sorted(os.listdir(preset_path)):
            scenario_path = os.path.join(preset_path, scenario)
            if not os.path.isdir(scenario_path):
                continue
            scenarios_seen.append(scenario)

            run_log_path = os.path.join(scenario_path, "run_log.csv")
            run_log_rows = read_csv(run_log_path)
            for row in run_log_rows:
                row["preset_dir"] = preset
                row["scenario_dir"] = scenario
                row["source_path"] = scenario_path
                all_run_logs.append(row)

            summary_path = os.path.join(scenario_path, "summary", "summary_by_run.csv")
            summary_rows = read_csv(summary_path)
            for row in summary_rows:
                row["preset_dir"] = preset
                row["scenario_dir"] = scenario
                row["source_path"] = scenario_path
                all_summary_rows.append(row)

    if all_run_logs:
        run_log_fields = list(all_run_logs[0].keys())
        write_csv(
            os.path.join(processed_dir, "run_log_all.csv"), run_log_fields, all_run_logs
        )
    else:
        write_csv(os.path.join(processed_dir, "run_log_all.csv"), ["run_id"], [])

    if all_summary_rows:
        summary_fields = list(all_summary_rows[0].keys())
        write_csv(
            os.path.join(processed_dir, "summary_by_run_all.csv"),
            summary_fields,
            all_summary_rows,
        )
    else:
        write_csv(os.path.join(processed_dir, "summary_by_run_all.csv"), ["run_id"], [])

    grouped = defaultdict(list)
    for row in all_summary_rows:
        grouped[
            (row.get("preset_dir", "unknown"), row.get("scenario_dir", "unknown"))
        ].append(row)

    table_rows = []
    for (preset, scenario), rows in sorted(grouped.items()):
        entry = {
            "preset": preset,
            "scenario": scenario,
            "model_a_visited_rate_mean": aggregate_mean(
                rows, "visited_rate", "model_a_dynamic"
            ),
            "model_b_visited_rate_mean": aggregate_mean(
                rows, "visited_rate", "model_b_worker_pool"
            ),
            "model_a_discovered_rate_mean": aggregate_mean(
                rows, "discovered_rate", "model_a_dynamic"
            ),
            "model_b_discovered_rate_mean": aggregate_mean(
                rows, "discovered_rate", "model_b_worker_pool"
            ),
            "model_a_p95_fetch_mean": aggregate_mean(
                rows, "p95_fetch", "model_a_dynamic"
            ),
            "model_b_p95_fetch_mean": aggregate_mean(
                rows, "p95_fetch", "model_b_worker_pool"
            ),
            "model_a_fetch_error_rate_mean": aggregate_mean(
                rows, "fetch_error_rate", "model_a_dynamic"
            ),
            "model_b_fetch_error_rate_mean": aggregate_mean(
                rows, "fetch_error_rate", "model_b_worker_pool"
            ),
            "model_a_rss_peak": aggregate_max(rows, "rss", "model_a_dynamic"),
            "model_b_rss_peak": aggregate_max(rows, "rss", "model_b_worker_pool"),
            "model_a_gc_p99_mean": aggregate_mean(
                rows, "gc_pause_p99", "model_a_dynamic"
            ),
            "model_b_gc_p99_mean": aggregate_mean(
                rows, "gc_pause_p99", "model_b_worker_pool"
            ),
        }
        table_rows.append(entry)

    table_fields = list(table_rows[0].keys()) if table_rows else ["preset", "scenario"]
    write_csv(
        os.path.join(processed_dir, "comparison_table.csv"), table_fields, table_rows
    )

    generated_at = datetime.now(timezone.utc).isoformat()

    latest_md = os.path.join(reports_dir, "latest.md")
    report_md = os.path.join(reports_dir, "report.md")

    with open(latest_md, "w", encoding="utf-8") as f:
        f.write("# Latest Research Snapshot\n\n")
        f.write(f"Generated at: `{generated_at}`\n\n")
        f.write(
            "This snapshot summarizes all preset and scenario runs currently available in this research_data set.\n\n"
        )
        f.write(
            f"- Presets covered: `{', '.join(sorted(set(presets_seen))) or 'none'}`\n"
        )
        f.write(
            f"- Scenarios covered: `{', '.join(sorted(set(scenarios_seen))) or 'none'}`\n"
        )
        f.write(f"- Total run records: `{len(all_run_logs)}`\n")
        f.write(f"- Total metric rows: `{len(all_summary_rows)}`\n\n")
        f.write(
            "Interpretation note: higher throughput metrics are generally better, while lower latency/error/memory/GC values indicate better efficiency and stability.\n"
        )

    with open(report_md, "w", encoding="utf-8") as f:
        f.write("# Comprehensive Research Report\n\n")
        f.write(f"Generated at: `{generated_at}`\n\n")
        f.write("## Research Goal Alignment\n\n")
        f.write(
            "This report evaluates the central goal from the research outline: identify where the fixed worker pool model (Model B) becomes more efficient and stable than the dynamic goroutine-per-task model (Model A).\n\n"
        )
        f.write("## Metric Definitions (Explicit)\n\n")
        f.write(
            "- `visited_rate` (URLs/s): speed of unique URL progression through scheduler. Higher means better throughput.\n"
        )
        f.write(
            "- `discovered_rate` (URLs/s): speed of link discovery from parsed pages. Higher means more expansion pressure.\n"
        )
        f.write(
            "- `p95_fetch` (seconds): 95th percentile fetch latency. Lower means better tail performance.\n"
        )
        f.write(
            "- `fetch_error_rate` (errors/s): failure frequency during crawl fetch operations. Lower means better stability.\n"
        )
        f.write(
            "- `rss` (bytes): resident set size in RAM. Lower peak means better memory efficiency under same workload.\n"
        )
        f.write(
            "- `gc_pause_p99` (seconds): p99 GC pause duration. Lower means less runtime overhead due to GC.\n\n"
        )
        f.write("## Consolidated Comparison Table\n\n")
        f.write(
            "| Preset | Scenario | A Visited/s | B Visited/s | A p95 Fetch (s) | B p95 Fetch (s) | A Error/s | B Error/s | A Peak RSS (MiB) | B Peak RSS (MiB) | A GC p99 (s) | B GC p99 (s) |\n"
        )
        f.write("|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
        for row in table_rows:
            f.write(
                "| {preset} | {scenario} | {a_vis} | {b_vis} | {a_p95} | {b_p95} | {a_err} | {b_err} | {a_rss} | {b_rss} | {a_gc} | {b_gc} |\n".format(
                    preset=row["preset"],
                    scenario=row["scenario"],
                    a_vis=fmt(row["model_a_visited_rate_mean"]),
                    b_vis=fmt(row["model_b_visited_rate_mean"]),
                    a_p95=fmt(row["model_a_p95_fetch_mean"]),
                    b_p95=fmt(row["model_b_p95_fetch_mean"]),
                    a_err=fmt(row["model_a_fetch_error_rate_mean"]),
                    b_err=fmt(row["model_b_fetch_error_rate_mean"]),
                    a_rss=fmt(row["model_a_rss_peak"], scale=1024 * 1024),
                    b_rss=fmt(row["model_b_rss_peak"], scale=1024 * 1024),
                    a_gc=fmt(row["model_a_gc_p99_mean"]),
                    b_gc=fmt(row["model_b_gc_p99_mean"]),
                )
            )

        f.write("\n## Explicit Interpretation Guidance\n\n")
        f.write(
            "1. Throughput wins are indicated by larger `visited_rate` under the same preset+scenario.\n"
        )
        f.write(
            "2. Efficiency wins are indicated by lower `rss` and lower `gc_pause_p99` under comparable throughput.\n"
        )
        f.write(
            "3. Stability wins are indicated by lower `fetch_error_rate`, especially in `429` and `500` scenarios.\n"
        )
        f.write(
            "4. The practical crossover point appears where Model B maintains equal/better throughput while also reducing memory and runtime overhead.\n\n"
        )
        f.write("## Artifacts Referenced\n\n")
        f.write("- Processed table: `processed/comparison_table.csv`\n")
        f.write("- Full metrics rows: `processed/summary_by_run_all.csv`\n")
        f.write("- Run metadata: `processed/run_log_all.csv`\n")
        f.write("- Diagrams: `diagrams/`\n")

    manifest = {
        "generated_at": generated_at,
        "raw_root": raw_root,
        "processed_dir": processed_dir,
        "reports_dir": reports_dir,
        "records": {
            "run_logs": len(all_run_logs),
            "summary_rows": len(all_summary_rows),
            "comparison_rows": len(table_rows),
        },
    }
    with open(os.path.join(meta_dir, "manifest.json"), "w", encoding="utf-8") as f:
        json.dump(manifest, f, indent=2)


if __name__ == "__main__":
    main()
