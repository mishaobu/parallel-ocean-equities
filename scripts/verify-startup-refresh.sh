#!/bin/sh

set -eu

base_url="http://127.0.0.1:${PORT:-8080}"
app_path="${BASE_PATH:-/equities}"
poll_seconds="${STARTUP_REFRESH_VERIFY_POLL_SECONDS:-5}"
timeout_seconds="${STARTUP_REFRESH_VERIFY_TIMEOUT_SECONDS:-1200}"
not_before="${STARTUP_REFRESH_NOT_BEFORE:-}"
not_before_epoch="${STARTUP_REFRESH_NOT_BEFORE_EPOCH:-}"
allow_macro_degraded="${STARTUP_REFRESH_ALLOW_MACRO_DEGRADED:-false}"

case "$poll_seconds:$timeout_seconds" in
	*[!0-9:]* | 0:* | *:0)
		echo "startup refresh verification intervals must be positive integers" >&2
		exit 2
		;;
esac

case "$allow_macro_degraded" in
	true | false) ;;
	*)
		echo "STARTUP_REFRESH_ALLOW_MACRO_DEGRADED must be true or false" >&2
		exit 2
		;;
esac

case "$not_before" in
	"" | ????-??-??T??:??:??Z) ;;
	*)
		echo "STARTUP_REFRESH_NOT_BEFORE must be an RFC3339 UTC timestamp without fractional seconds" >&2
		exit 2
		;;
esac

case "$not_before_epoch" in
	"" | *[!0-9]*)
		if [ -n "$not_before_epoch" ]; then
			echo "STARTUP_REFRESH_NOT_BEFORE_EPOCH must be Unix seconds" >&2
			exit 2
		fi
		;;
esac

metric_value() {
	metric_name="$1"
	metrics="$2"
	printf '%s\n' "$metrics" | awk -v name="$metric_name" '$1 == name { print $2; exit }'
}

readiness="$(wget -qO- "$base_url/readyz")"
ticker_count="$(printf '%s\n' "$readiness" | sed -n 's/.*"tickers":[[:space:]]*\([0-9][0-9]*\).*/\1/p')"
if [ -z "$ticker_count" ] || [ "$ticker_count" -lt 1 ]; then
	echo "startup refresh verification could not resolve a non-empty watchlist" >&2
	exit 1
fi

started_at="$(date +%s)"
deadline=$((started_at + timeout_seconds))
last_pending="startup refresh has not reported ticker observations"

while :; do
	metrics="$(wget -qO- "$base_url/metrics")"
	refresh_total="$(metric_value equities_refresh_total "$metrics")"
	refresh_failures="$(metric_value equities_refresh_failures_total "$metrics")"
	refresh_inflight="$(metric_value equities_refresh_inflight "$metrics")"
	snapshot_scheduler_running="$(metric_value equities_scheduled_snapshot_scheduler_running "$metrics")"
	macro_refreshing="$(metric_value equities_macro_refreshing "$metrics")"
	macro_failures="$(metric_value equities_macro_refresh_failures_total "$metrics")"
	macro_last_attempt="$(metric_value equities_macro_last_attempt_timestamp_seconds "$metrics")"
	macro_last_success="$(metric_value equities_macro_last_success_timestamp_seconds "$metrics")"
	macro_degraded="$(metric_value equities_macro_degraded "$metrics")"
	macro_stale="$(metric_value equities_macro_stale "$metrics")"

	if [ -z "$refresh_total" ] || [ -z "$refresh_failures" ] || [ -z "$refresh_inflight" ] || [ -z "$snapshot_scheduler_running" ] || [ -z "$macro_refreshing" ] || [ -z "$macro_failures" ] || [ -z "$macro_last_attempt" ] || [ -z "$macro_last_success" ] || [ -z "$macro_degraded" ] || [ -z "$macro_stale" ]; then
		echo "startup refresh verification could not read refresh, macro, and snapshot scheduler metrics" >&2
		exit 1
	fi
	if [ "$refresh_failures" -gt 0 ]; then
		echo "startup refresh failed: $refresh_failures of $refresh_total completed attempts failed" >&2
		exit 1
	fi
	if [ "$snapshot_scheduler_running" -ne 1 ]; then
		echo "startup refresh cannot proceed because quote snapshot scheduler is not running" >&2
		exit 1
	fi

	tickers="$(printf '%s\n' "$metrics" | sed -n 's/^equities_scheduled_snapshot_quote_fields_expected{ticker="\([^"]*\)"} .*/\1/p' | sort -u)"
	observed_ticker_count="$(printf '%s\n' "$tickers" | awk 'NF { count++ } END { print count + 0 }')"
	all_tickers_ready=true
	last_pending=""
	if [ "$observed_ticker_count" -ne "$ticker_count" ]; then
		all_tickers_ready=false
		last_pending="metrics expose $observed_ticker_count of $ticker_count persisted tickers"
	else
		for ticker in $tickers; do
			equity="$(wget -qO- "$base_url$app_path/api/tickers/$ticker")"
			status="$(printf '%s\n' "$equity" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')"
			updated_at="$(printf '%s\n' "$equity" | sed -n 's/.*"updatedAt":"\([^"]*\)".*/\1/p')"
			updated_at_seconds="$(printf '%s\n' "$updated_at" | sed 's/\.[0-9][0-9]*Z$/Z/')"
			if [ "$status" != "ready" ]; then
				all_tickers_ready=false
				last_pending="$ticker status is ${status:-missing}"
				break
			fi
			if [ -n "$not_before" ] && ! awk -v actual="$updated_at_seconds" -v cutoff="$not_before" 'BEGIN { exit !(actual >= cutoff) }'; then
				all_tickers_ready=false
				last_pending="$ticker was last refreshed at ${updated_at:-missing}, before $not_before"
				break
			fi
		done
	fi

	if [ "$refresh_total" -ge "$ticker_count" ] && [ "$refresh_inflight" -eq 0 ] && [ "$all_tickers_ready" = true ]; then
		if [ "$macro_refreshing" -ne 0 ]; then
			last_pending="macro refresh is still running"
		elif [ -n "$not_before_epoch" ] && [ "$macro_last_attempt" -lt "$not_before_epoch" ]; then
			last_pending="macro refresh has not attempted since process startup"
		elif [ "$macro_last_success" = "0" ]; then
			echo "macro refresh completed without a persisted successful baseline" >&2
			exit 1
		elif [ "$macro_stale" -ne 0 ]; then
			echo "persisted macro baseline is beyond the backend freshness limit" >&2
			exit 1
		elif [ "$macro_degraded" -ne 0 ] && [ "$allow_macro_degraded" != true ]; then
			last_pending="macro refresh is degraded and policy requires a healthy refresh"
		else
			if [ "$macro_degraded" -ne 0 ]; then
				echo "warning: macro refresh is degraded after $macro_failures failed attempt(s); verified non-stale persisted baseline remains available" >&2
			fi
			echo "startup refresh verified: $refresh_total completed attempts; all $ticker_count persisted tickers are ready and current; scheduler is running"
			exit 0
		fi
	fi
	if [ "$(date +%s)" -ge "$deadline" ]; then
		echo "startup refresh timed out: completed=$refresh_total expected=$ticker_count inflight=$refresh_inflight macro_refreshing=$macro_refreshing macro_failures=$macro_failures macro_last_attempt=$macro_last_attempt macro_last_success=$macro_last_success macro_degraded=$macro_degraded macro_stale=$macro_stale snapshot_scheduler_running=$snapshot_scheduler_running pending=$last_pending" >&2
		exit 1
	fi
	sleep "$poll_seconds"
done
