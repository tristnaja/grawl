#!/usr/bin/env Rscript

suppressPackageStartupMessages({
  library(ggplot2)
})

args <- commandArgs(trailingOnly = TRUE)
if (length(args) < 2) {
  stop("usage: generate_diagrams.R <comparison_table.csv> <out_dir>")
}

in_csv <- args[[1]]
out_dir <- args[[2]]

if (!file.exists(in_csv)) {
  stop(paste("missing input csv:", in_csv))
}

dir.create(out_dir, recursive = TRUE, showWarnings = FALSE)

df <- read.csv(in_csv, stringsAsFactors = FALSE)

to_long <- function(df, a_col, b_col, value_name) {
  data.frame(
    preset = c(df$preset, df$preset),
    scenario = c(df$scenario, df$scenario),
    model = c(rep("Model A", nrow(df)), rep("Model B", nrow(df))),
    value = c(df[[a_col]], df[[b_col]]),
    metric = value_name,
    stringsAsFactors = FALSE
  )
}

plot_metric <- function(long_df, title, ylab, filename) {
  p <- ggplot(long_df, aes(x = scenario, y = value, fill = model)) +
    geom_col(position = position_dodge(width = 0.8), width = 0.7) +
    facet_wrap(~preset, scales = "free_y") +
    labs(title = title, x = "Scenario", y = ylab) +
    theme_minimal(base_size = 12)
  ggsave(filename = file.path(out_dir, filename), plot = p, width = 10, height = 5, dpi = 150)
}

if (nrow(df) > 0) {
  throughput <- to_long(df, "model_a_visited_rate_mean", "model_b_visited_rate_mean", "Visited URLs/s")
  latency <- to_long(df, "model_a_p95_fetch_mean", "model_b_p95_fetch_mean", "p95 Fetch (s)")
  errors <- to_long(df, "model_a_fetch_error_rate_mean", "model_b_fetch_error_rate_mean", "Fetch Errors/s")
  rss <- to_long(df, "model_a_rss_peak", "model_b_rss_peak", "Peak RSS (bytes)")

  plot_metric(throughput, "Throughput Comparison", "Visited URLs/s", "throughput_comparison.png")
  plot_metric(latency, "Latency Comparison", "p95 Fetch (s)", "latency_comparison.png")
  plot_metric(errors, "Error Rate Comparison", "Fetch Errors/s", "error_rate_comparison.png")
  plot_metric(rss, "Memory Comparison", "Peak RSS (bytes)", "memory_comparison.png")
}
