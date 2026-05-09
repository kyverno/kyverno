#!/usr/bin/env bash
# Convert k6-summary.json into a markdown report.

set -e

usage() {
  echo "Usage: $0 [OPTIONS] [path-to-k6-summary.json]"
  echo ""
  echo "Options (all sections included by default):"
  echo "  --no-config    Omit Config section (duration, execution)"
  echo "  --no-script    Omit Script code block"
  echo "  --no-checks    Omit Results > Checks (.results.checks)"
  echo "  --no-metrics   Omit Results > Metrics"
  echo "  -h, --help     Show this help"
}

INCLUDE_CONFIG=1
INCLUDE_SCRIPT=1
INCLUDE_CHECKS=1
INCLUDE_METRICS=1

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-config)  INCLUDE_CONFIG=0; shift ;;
    --no-script)  INCLUDE_SCRIPT=0; shift ;;
    --no-checks)  INCLUDE_CHECKS=0; shift ;;
    --no-metrics) INCLUDE_METRICS=0; shift ;;
    -h|--help)    usage; exit 0 ;;
    -*)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
    *)
      break
      ;;
  esac
done

SUMMARY_JSON="${1:-k6-summary.json}"

if [[ ! -f "$SUMMARY_JSON" ]]; then
  echo "Error: $SUMMARY_JSON not found" >&2
  exit 1
fi

jq -r \
  --arg config   "$INCLUDE_CONFIG" \
  --arg script   "$INCLUDE_SCRIPT" \
  --arg checks   "$INCLUDE_CHECKS" \
  --arg metrics  "$INCLUDE_METRICS" \
  '
  "# K6 Summary Report\n",

  (if $config == "1" then
    "\n## Config\n",
    "| Field | Value |",
    "|-------|-------|",
    "| duration | \(.config.duration) |",
    "| execution | \(.config.execution) |"
  else "" end),

  (if $script == "1" then
    "\n<details>\n<summary><strong>Script</strong></summary>\n\n```javascript\n",
    (.config.script | gsub("\\n"; "\n")),
    "\n```\n\n</details>\n"
  else "" end),

  (if $checks == "1" then "\n## Results\n" else "" end),

  (if $checks == "1" then
    "\n### Checks\n",
    (if (.results.checks.results | length) > 0 then
      "| Pass | Name | |\n|------|------|---|\n",
      (.results.checks.results[] | "| \(.pass) | \(.name) | |\n")
    else
      "*No checks.*\n"
    end),
    (if (.results.checks.metrics | length) > 0 then
      "\n### Checks metrics\n",
      (.results.checks.metrics[] |
        "##### \(.name)\n",
        "**Type:** \(.type)\n",
        (if .values | type == "object" then
          (.values | to_entries) as $entries |
          if ($entries | length) > 0 then
            [
              ($entries | map(.key) | "| " + join(" | ") + " |"),
              ($entries | map("-----") | "|" + join("|") + "|"),
              ($entries | map(.value | tostring) | "| " + join(" | ") + " |")
            ] | join("\n") + "\n"
          else
            "*No values.*\n"
          end
        else
          "\(.values)\n"
        end),
        "\n"
      )
    else
      ""
    end)
  else "" end),

  (if $metrics == "1" then
    "\n## Metrics\n",
    (
      .results.metrics as $metrics |
      (["vus", "iterations", "iteration_duration"] | .[]) as $name |
      ($metrics[] | select(.name == $name)) |
      "### \(.name)\n",
      (if .values | type == "object" then
        .name as $metric_name |
        (.values | to_entries) as $entries |
        if ($entries | length) > 0 then
          [
            ($entries | map(.key) | "| " + join(" | ") + " |"),
            ($entries | map("-----") | "|" + join("|") + "|"),
            (if $metric_name == "iteration_duration" then
              ($entries | map(.value | ((. * 100 | round) / 100 | tostring) + " ms") | "| " + join(" | ") + " |")
            else
              ($entries | map(.value | tostring) | "| " + join(" | ") + " |")
            end)
          ] | join("\n") + "\n"
        else
          "*No values.*\n"
        end
      else
        "\(.values)\n"
      end),
      "\n"
    )
  else "" end)
  ' "$SUMMARY_JSON"
