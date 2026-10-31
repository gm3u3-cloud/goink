import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

// useTimeAgo wraps "relative time" rendering: it maintains a now timestamp
// refreshed every minute, and returns timeAgo(iso) which turns an ISO time
// string into "just now / x minutes ago / x hours ago / ...".
//
// enabled controls whether the timer runs: pass true when the panel/list is
// visible to auto-refresh relative times, false when hidden to stop the timer
// and avoid pointless re-renders.
// Reused in: SessionHistory(open), RecentSessions, GitHistoryList.
//
// i18n keys uniformly use the time.* namespace (the relative-time keys from
// the old chat.*/git.* namespaces have been moved out).
export function useTimeAgo(enabled = true) {
  const { t } = useTranslation();
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!enabled) return;
    const timer = setInterval(() => setNow(Date.now()), 60_000);
    return () => clearInterval(timer);
  }, [enabled]);

  return useCallback(
    (iso: string) => {
      const diff = now - new Date(iso).getTime();
      const min = Math.floor(diff / 60000);
      if (min < 1) return t("time.justNow");
      if (min < 60) return t("time.minutesAgo", { count: min });
      const hour = Math.floor(min / 60);
      if (hour < 24) return t("time.hoursAgo", { count: hour });
      const day = Math.floor(hour / 24);
      if (day < 30) return t("time.daysAgo", { count: day });
      return t("time.monthsAgo", { count: Math.floor(day / 30) });
    },
    [now, t],
  );
}
