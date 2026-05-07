#!/usr/bin/env python3
import argparse
import csv
import json
import math
import os
import re
from collections import defaultdict
from datetime import datetime, timezone


METRIC_FILES = {
    "visited_rate": "prom_visited_rate.json",
    "discovered_rate": "prom_discovered_rate.json",
    "fetch_error_rate": "prom_fetch_error_rate.json",
    "robots_denied_rate": "prom_robots_denied_rate.json",
    "p95_fetch": "prom_p95_fetch.json",
    "inflight": "prom_inflight.json",
    "workers": "prom_workers.json",
    "goroutines": "prom_goroutines.json",
    "rss": "prom_rss.json",
    "heap_alloc": "prom_heap_alloc.json",
    "runtime_goroutines": "prom_runtime_goroutines.json",
    "gc_pause_p99": "prom_gc_pause_p99.json",
}


def parse_args():
    parser = argparse.ArgumentParser(
        description="Summarize grawl research outputs and generate report"
    )
    parser.add_argument("--input-dir", required=True, help="Research output directory")
    return parser.parse_args()


def safe_float(value):
    try:
        return float(value)
    except Exception:
        return math.nan


def parse_model_name(metric_dict):
    model = metric_dict.get("model")
    if model:
        return model

    job = metric_dict.get("job", "")
    if "model_a" in job:
        return "model_a_dynamic"
    if "model_b" in job:
        return "model_b_worker_pool"
    return "unknown"


def load_query_values(path):
    if not os.path.exists(path):
        return {}

    with open(path, "r", encoding="utf-8") as f:
        payload = json.load(f)

    result = payload.get("data", {}).get("result", [])
    out = defaultdict(list)

    for series in result:
        model = parse_model_name(series.get("metric", {}))
        values = series.get("values", [])
        nums = [safe_float(v[1]) for v in values]
        nums = [x for x in nums if not math.isnan(x)]
        if nums:
            out[model].extend(nums)

    return out


def percentile(values, p):
    if not values:
        return math.nan
    ordered = sorted(values)
    k = (len(ordered) - 1) * p
    f = math.floor(k)
    c = math.ceil(k)
    if f == c:
        return ordered[int(k)]
    return ordered[f] * (c - k) + ordered[c] * (k - f)


def metric_stats(values):
    if not values:
        return {"mean": math.nan, "p50": math.nan, "p95": math.nan, "max": math.nan}
    return {
        "mean": sum(values) / len(values),
        "p50": percentile(values, 0.50),
        "p95": percentile(values, 0.95),
        "max": max(values),
    }


def parse_run_id(run_id):
    # expected: <preset>_<scenario>_rXX
    parts = run_id.rsplit("_", 1)
    repeat = parts[1] if len(parts) == 2 else "r00"
    pre = parts[0] if len(parts) == 2 else run_id
    pparts = pre.split("_", 1)
    preset = pparts[0] if pparts else "unknown"
    scenario = pparts[1] if len(pparts) > 1 else "unknown"
    return preset, scenario, repeat


def run_summary(run_path):
    by_model = defaultdict(dict)

    for metric_key, filename in METRIC_FILES.items():
        values_by_model = load_query_values(os.path.join(run_path, filename))
        for model, vals in values_by_model.items():
            by_model[model][metric_key] = metric_stats(vals)

    return by_model


def read_run_log(path):
    if not os.path.exists(path):
        return {}
    out = {}
    with open(path, "r", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        for row in reader:
            out[row["run_id"]] = row
    return out


def model_label(model):
    if model == "model_a_dynamic":
        return "Model A (dynamic)"
    if model == "model_b_worker_pool":
        return "Model B (worker pool)"
    return model


def fmt_num(v, scale=1.0, suffix=""):
    if v is None or (isinstance(v, float) and math.isnan(v)):
        return "n/a"
    return f"{(v / scale):.4f}{suffix}"


def aggregate_rows(rows, metric, stat_key):
    vals = []
    for row in rows:
        v = row.get(metric, {}).get(stat_key)
        if v is None:
            continue
        if isinstance(v, float) and math.isnan(v):
            continue
        vals.append(v)
    if not vals:
        return math.nan
    return sum(vals) / len(vals)


def build_report(summary_rows, run_log_rows, generated_at):
    grouped = defaultdict(lambda: defaultdict(lambda: defaultdict(list)))
    # grouped[preset][scenario][model] = [row_metrics]
    for run_id, models in summary_rows.items():
        preset, scenario, _repeat = parse_run_id(run_id)
        for model, metrics in models.items():
            grouped[preset][scenario][model].append(metrics)

    lines = []
    lines.append("# Research Summary Report")
    lines.append("")
    lines.append(f"Generated at: `{generated_at}`")
    lines.append("")
    lines.append("This report maps directly to the research outline goals:")
    lines.append("")
    lines.append("- Throughput comparison between Model A and Model B")
    lines.append(
        "- Memory/runtime overhead indicators (RSS, heap, goroutines, GC pause)"
    )
    lines.append("- Stability indicators under baseline and injected-failure scenarios")
    lines.append("")

    lines.append("## Run Inventory")
    lines.append("")
    lines.append("| Run ID | Preset | Scenario | Status | Duration |")
    lines.append("|---|---|---|---|---|")
    for run_id in sorted(run_log_rows.keys()):
        row = run_log_rows[run_id]
        preset, scenario, _ = parse_run_id(run_id)
        lines.append(
            f"| {run_id} | {preset} | {scenario} | {row.get('status', 'n/a')} | {row.get('duration', 'n/a')} |"
        )
    lines.append("")

    lines.append("## Results by Preset and Scenario")
    lines.append("")

    for preset in sorted(grouped.keys()):
        lines.append(f"### Preset: `{preset}`")
        lines.append("")
        for scenario in sorted(grouped[preset].keys()):
            lines.append(f"#### Scenario: `{scenario}`")
            lines.append("")

            model_a = grouped[preset][scenario].get("model_a_dynamic", [])
            model_b = grouped[preset][scenario].get("model_b_worker_pool", [])

            a_vis = aggregate_rows(model_a, "visited_rate", "mean")
            b_vis = aggregate_rows(model_b, "visited_rate", "mean")
            a_dis = aggregate_rows(model_a, "discovered_rate", "mean")
            b_dis = aggregate_rows(model_b, "discovered_rate", "mean")
            a_err = aggregate_rows(model_a, "fetch_error_rate", "mean")
            b_err = aggregate_rows(model_b, "fetch_error_rate", "mean")
            a_p95 = aggregate_rows(model_a, "p95_fetch", "mean")
            b_p95 = aggregate_rows(model_b, "p95_fetch", "mean")
            a_rss = aggregate_rows(model_a, "rss", "max")
            b_rss = aggregate_rows(model_b, "rss", "max")
            a_gc = aggregate_rows(model_a, "gc_pause_p99", "mean")
            b_gc = aggregate_rows(model_b, "gc_pause_p99", "mean")

            lines.append("| Metric | Model A | Model B |")
            lines.append("|---|---:|---:|")
            lines.append(
                f"| Throughput: visited URLs/s (mean of run means) | {fmt_num(a_vis)} | {fmt_num(b_vis)} |"
            )
            lines.append(
                f"| Throughput: discovered URLs/s (mean of run means) | {fmt_num(a_dis)} | {fmt_num(b_dis)} |"
            )
            lines.append(
                f"| Error rate: fetch errors/s (mean of run means) | {fmt_num(a_err)} | {fmt_num(b_err)} |"
            )
            lines.append(
                f"| Fetch latency p95 (seconds, mean) | {fmt_num(a_p95)} | {fmt_num(b_p95)} |"
            )
            lines.append(
                f"| Peak RSS (MiB, mean of run max) | {fmt_num(a_rss, scale=1024 * 1024)} | {fmt_num(b_rss, scale=1024 * 1024)} |"
            )
            lines.append(
                f"| GC pause p99 (seconds, mean) | {fmt_num(a_gc)} | {fmt_num(b_gc)} |"
            )
            lines.append("")

            # concise scenario conclusion
            conclusion = []
            if not math.isnan(a_vis) and not math.isnan(b_vis):
                if b_vis > a_vis:
                    conclusion.append(
                        "Model B shows higher visited-throughput in this scenario."
                    )
                elif a_vis > b_vis:
                    conclusion.append(
                        "Model A shows higher visited-throughput in this scenario."
                    )
                else:
                    conclusion.append(
                        "Both models show similar visited-throughput in this scenario."
                    )

            if not math.isnan(a_rss) and not math.isnan(b_rss):
                if b_rss < a_rss:
                    conclusion.append(
                        "Model B has lower peak RSS, indicating tighter memory behavior."
                    )
                elif a_rss < b_rss:
                    conclusion.append("Model A has lower peak RSS in this scenario.")

            if not math.isnan(a_err) and not math.isnan(b_err):
                if b_err < a_err:
                    conclusion.append(
                        "Model B has lower fetch-error rate (more stable under this load pattern)."
                    )
                elif a_err < b_err:
                    conclusion.append(
                        "Model A has lower fetch-error rate (more stable under this load pattern)."
                    )

            if conclusion:
                lines.append("Interpretation:")
                for item in conclusion:
                    lines.append(f"- {item}")
                lines.append("")

    lines.append("## Research Goal Mapping")
    lines.append("")
    lines.append(
        "The research outline asks for the efficiency crossover point where fixed worker pool outperforms dynamic scheduling in resource efficiency and stability."
    )
    lines.append("")
    lines.append("Use this report as follows:")
    lines.append("")
    lines.append(
        "- Throughput axis: compare `visited URLs/s` and `discovered URLs/s` between models."
    )
    lines.append("- Memory efficiency axis: compare `Peak RSS` and `heap` behavior.")
    lines.append(
        "- Runtime overhead axis: compare `GC pause p99` and goroutine/worker pressure metrics."
    )
    lines.append(
        "- Stability axis: compare fetch-error rate under baseline vs `429` and `500` scenarios."
    )
    lines.append("")
    lines.append(
        "When Model B maintains lower memory/GC overhead while preserving equal or better throughput, that region is a practical crossover indicator."
    )

    return "\n".join(lines) + "\n"


def write_summary_csv(summary_dir, generated_at, summary_rows):
    out = os.path.join(summary_dir, "summary_by_run.csv")
    fields = [
        "generated_at",
        "run_id",
        "model",
        "metric",
        "mean",
        "p50",
        "p95",
        "max",
    ]

    with open(out, "w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fields)
        writer.writeheader()
        for run_id in sorted(summary_rows.keys()):
            models = summary_rows[run_id]
            for model, metrics in models.items():
                for metric, stats in metrics.items():
                    writer.writerow(
                        {
                            "generated_at": generated_at,
                            "run_id": run_id,
                            "model": model,
                            "metric": metric,
                            "mean": stats.get("mean", ""),
                            "p50": stats.get("p50", ""),
                            "p95": stats.get("p95", ""),
                            "max": stats.get("max", ""),
                        }
                    )


def main():
    args = parse_args()
    root = args.input_dir
    if not os.path.isdir(root):
        raise SystemExit(f"input directory does not exist: {root}")

    run_log_rows = read_run_log(os.path.join(root, "run_log.csv"))

    run_dir_pattern = re.compile(r".+_r\d{2}$")
    summary_rows = {}
    for name in sorted(os.listdir(root)):
        path = os.path.join(root, name)
        if not os.path.isdir(path):
            continue
        if name == "summary":
            continue
        if not run_dir_pattern.match(name):
            continue
        summary_rows[name] = run_summary(path)

    generated_at = datetime.now(timezone.utc).isoformat()
    summary_dir = os.path.join(root, "summary")
    os.makedirs(summary_dir, exist_ok=True)

    write_summary_csv(summary_dir, generated_at, summary_rows)

    report_md = build_report(summary_rows, run_log_rows, generated_at)
    with open(os.path.join(summary_dir, "report.md"), "w", encoding="utf-8") as f:
        f.write(report_md)

    with open(os.path.join(summary_dir, "latest.md"), "w", encoding="utf-8") as f:
        f.write("# Research Summary\n\n")
        f.write(f"Generated at: `{generated_at}`\n\n")
        f.write(
            "- `summary_by_run.csv`: normalized per-run/per-model/per-metric aggregates\n"
        )
        f.write("- `report.md`: comprehensive interpretation against research goals\n")


if __name__ == "__main__":
    main()
