package tgbot

import (
	"strings"
	"testing"
	"time"
)

func at(t *testing.T, layout string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04", layout, time.Local)
	if err != nil {
		t.Fatalf("parse %q: %v", layout, err)
	}
	return parsed
}

// Rungs are exact calendar-day matches, not a rolling window: each one must fire
// on its own day and stay silent on every other, or a customer is nagged daily.
func TestExpiryRung(t *testing.T) {
	now := at(t, "2026-08-21 09:00")

	tests := []struct {
		name   string
		expiry time.Time
		want   renewalRung
	}{
		{"three days out", at(t, "2026-08-24 09:00"), rungThreeDays},
		{"three days out, late in the day", at(t, "2026-08-24 23:59"), rungThreeDays},
		{"two days out", at(t, "2026-08-23 09:00"), rungTwoDays},
		{"tomorrow", at(t, "2026-08-22 00:01"), rungTomorrow},
		{"later today", at(t, "2026-08-21 23:00"), rungToday},
		{"earlier today is still today", at(t, "2026-08-21 01:00"), rungToday},
		{"yesterday", at(t, "2026-08-20 23:00"), rungOverdue},
		{"long overdue", at(t, "2026-07-01 09:00"), rungOverdue},
		{"four days out", at(t, "2026-08-25 09:00"), rungNone},
		{"far future", at(t, "2027-01-01 09:00"), rungNone},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := expiryRung(tc.expiry.UnixMilli(), now); got != tc.want {
				t.Fatalf("expiryRung(%s) = %v, want %v", tc.expiry.Format(time.RFC3339), got, tc.want)
			}
		})
	}
}

// A zero expiry never lapses and a negative one has not started counting down,
// so neither belongs on a ladder about renewal dates.
func TestExpiryRungIgnoresSentinelExpiries(t *testing.T) {
	now := at(t, "2026-08-21 09:00")
	for _, expiry := range []int64{0, -2592000000} {
		if got := expiryRung(expiry, now); got != rungNone {
			t.Fatalf("expiryRung(%d) = %v, want rungNone", expiry, got)
		}
	}
}

// "Tomorrow" must mean the next date, not the next 24 hours: an expiry late
// tonight is due today, and one just after midnight is due tomorrow.
func TestExpiryRungUsesCalendarDaysNotElapsedHours(t *testing.T) {
	now := at(t, "2026-08-21 23:30")
	if got := expiryRung(at(t, "2026-08-22 00:30").UnixMilli(), now); got != rungTomorrow {
		t.Fatalf("an expiry one hour away but on the next date = %v, want rungTomorrow", got)
	}
	if got := expiryRung(at(t, "2026-08-21 23:59").UnixMilli(), now); got != rungToday {
		t.Fatalf("an expiry later tonight = %v, want rungToday", got)
	}
}

// The 2-day rung widens what the admin summary covers without adding a fourth
// reminder: it must carry a summary key and no customer message key.
func TestTwoDayRungIsSummaryOnly(t *testing.T) {
	if _, notifies := rungMessageKey[rungTwoDays]; notifies {
		t.Fatal("rungTwoDays has a customer message key, so clients would be notified 2 days out")
	}
	if _, summarised := rungSummaryKey[rungTwoDays]; !summarised {
		t.Fatal("rungTwoDays has no summary key, so admins would never see it")
	}

	// Every other rung on the ladder still reaches the customer.
	for _, rung := range []renewalRung{rungThreeDays, rungTomorrow, rungToday, rungOverdue} {
		if _, notifies := rungMessageKey[rung]; !notifies {
			t.Fatalf("rung %v lost its customer message key", rung)
		}
	}
}

// The summary reads as a countdown, so 2 days must sit between tomorrow and 3
// days rather than being appended wherever the map happened to order it.
func TestRenewalSummaryOrdersTwoDaysAfterTomorrow(t *testing.T) {
	tg := &Tgbot{}
	summary := tg.renewalSummary(map[renewalRung][]string{
		rungOverdue:   {"overdue@x"},
		rungToday:     {"today@x"},
		rungTomorrow:  {"tomorrow@x"},
		rungTwoDays:   {"twodays@x"},
		rungThreeDays: {"threedays@x"},
	})

	want := []string{
		"tgbot.messages.renewSummaryOverdue",
		"tgbot.messages.renewSummaryToday",
		"tgbot.messages.renewSummaryTomorrow",
		"tgbot.messages.renewSummaryTwoDays",
		"tgbot.messages.renewSummaryThreeDays",
	}
	prev := -1
	for _, key := range want {
		next := strings.Index(summary, key)
		if next < 0 {
			t.Fatalf("summary is missing %q: %q", key, summary)
		}
		if next < prev {
			t.Fatalf("%q appears out of countdown order in %q", key, summary)
		}
		prev = next
	}
}
