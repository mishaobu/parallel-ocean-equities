#!/bin/sh

set -eu

base_url="http://127.0.0.1:${PORT:-8080}"
poll_seconds="${STARTUP_REFRESH_VERIFY_POLL_SECONDS:-5}"
timeout_seconds="${STARTUP_REFRESH_VERIFY_TIMEOUT_SECONDS:-1200}"

case "$poll_seconds:$timeout_seconds" in
	*[!0-9:]* | 0:* | *:0)
		echo "startup refresh verification intervals must be positive integers" >&2
		exit 2
		;;
esac

metric_value() {
	metric_name="$1"
	metrics="$2"
	printf '%s\n' "$metrics" | awk -v name="$metric_name" '$1 == name { print $2; exit }'
}

health="$(wget -qO- "$base_url/healthz")"
ticker_count="$(printf '%s\n' "$health" | sed -n 's/.*"tickers":[[:space:]]*\([0-9][0-9]*\).*/\1/p')"
if [ -z "$ticker_count" ] || [ "$ticker_count" -lt 1 ]; then
	echo "startup refresh verification could not resolve a non-empty watchlist" >&2
	exit 1
fi

started_at="$(date +%s)"
deadline=$((started_at + timeout_seconds))

while :; do
	metrics="$(wget -qO- "$base_url/metrics")"
	refresh_total="$(metric_value equities_refresh_total "$metrics")"
	refresh_failures="$(metric_value equities_refresh_failures_total "$metrics")"
	refresh_inflight="$(metric_value equities_refresh_inflight "$metrics")"
	snapshot_scheduler_running="$(metric_value equities_scheduled_snapshot_scheduler_running "$metrics")"

	if [ -z "$refresh_total" ] || [ -z "$refresh_failures" ] || [ -z "$refresh_inflight" ] || [ -z "$snapshot_scheduler_running" ]; then
		echo "startup refresh verification could not read refresh and snapshot scheduler metrics" >&2
		exit 1
	fi
	if [ "$refresh_failures" -gt 0 ]; then
		echo "startup refresh failed: $refresh_failures of $refresh_total completed attempts failed" >&2
		exit 1
	fi
	if [ "$refresh_total" -ge "$ticker_count" ] && [ "$refresh_inflight" -eq 0 ]; then
		if [ "$snapshot_scheduler_running" -ne 1 ]; then
			echo "startup refresh completed but quote snapshot scheduler is not running" >&2
			exit 1
		fi
		echo "startup refresh verified: $refresh_total completed attempts for $ticker_count persisted tickers"
		exit 0
	fi
	if [ "$(date +%s)" -ge "$deadline" ]; then
		echo "startup refresh timed out: completed=$refresh_total expected=$ticker_count inflight=$refresh_inflight snapshot_scheduler_running=$snapshot_scheduler_running" >&2
		exit 1
	fi
	sleep "$poll_seconds"
done
