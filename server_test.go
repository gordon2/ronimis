package main

import (
	"math"
	"testing"
	"time"
)

func loadTallinn(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Tallinn")
	if err != nil {
		t.Skipf("Europe/Tallinn tzdata unavailable: %v", err)
	}
	return loc
}

func TestBusynessLocalTime(t *testing.T) {
	tallinn := loadTallinn(t)

	t.Run("UTC summer converts to UTC+3", func(t *testing.T) {
		got, ok := busynessLocalTime("2025-07-01 09:00:00", "UTC", tallinn)
		if !ok {
			t.Fatal("expected ok == true")
		}
		if got.Hour() != 12 {
			t.Errorf("hour = %d, want 12", got.Hour())
		}
		wantInstant := time.Date(2025, 7, 1, 9, 0, 0, 0, time.UTC)
		if !got.Equal(wantInstant) {
			t.Errorf("instant = %v, want %v", got.UTC(), wantInstant)
		}
	})

	t.Run("UTC winter converts to UTC+2", func(t *testing.T) {
		got, ok := busynessLocalTime("2025-12-01 09:00:00", "UTC", tallinn)
		if !ok {
			t.Fatal("expected ok == true")
		}
		if got.Hour() != 11 {
			t.Errorf("hour = %d, want 11", got.Hour())
		}
	})

	t.Run("EEST wall-clock preserved as local", func(t *testing.T) {
		got, ok := busynessLocalTime("2026-07-18 16:35:39", "EEST", tallinn)
		if !ok {
			t.Fatal("expected ok == true")
		}
		if got.Hour() != 16 || got.Minute() != 35 {
			t.Errorf("wall-clock = %02d:%02d, want 16:35", got.Hour(), got.Minute())
		}
		if got.Location() != tallinn {
			t.Errorf("location = %v, want Europe/Tallinn", got.Location())
		}
	})

	t.Run("EET wall-clock preserved as local", func(t *testing.T) {
		got, ok := busynessLocalTime("2025-12-01 09:00:00", "EET", tallinn)
		if !ok {
			t.Fatal("expected ok == true")
		}
		if got.Hour() != 9 {
			t.Errorf("hour = %d, want 9 (wall-clock preserved)", got.Hour())
		}
	})

	t.Run("empty tz treated as UTC", func(t *testing.T) {
		got, ok := busynessLocalTime("2025-07-01 09:00:00", "", tallinn)
		if !ok {
			t.Fatal("expected ok == true")
		}
		if got.Hour() != 12 {
			t.Errorf("hour = %d, want 12", got.Hour())
		}
	})

	t.Run("GMT and Z aliases and case-insensitivity", func(t *testing.T) {
		for _, tz := range []string{"GMT", "Z", "  utc  ", "z"} {
			got, ok := busynessLocalTime("2025-07-01 09:00:00", tz, tallinn)
			if !ok {
				t.Fatalf("tz %q: expected ok == true", tz)
			}
			if got.Hour() != 12 {
				t.Errorf("tz %q: hour = %d, want 12", tz, got.Hour())
			}
		}
	})

	t.Run("invalid timestamp returns false", func(t *testing.T) {
		got, ok := busynessLocalTime("not-a-timestamp", "UTC", tallinn)
		if ok {
			t.Errorf("expected ok == false, got %v", got)
		}
		if !got.IsZero() {
			t.Errorf("expected zero time, got %v", got)
		}
	})
}

func TestPickBucketMinutes(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("one day span stays raw", func(t *testing.T) {
		got := pickBucketMinutes(base, base.AddDate(0, 0, 1))
		if got != 2 {
			t.Errorf("got %d, want 2", got)
		}
	})

	t.Run("31 day span picks a mid ladder value", func(t *testing.T) {
		got := pickBucketMinutes(base, base.AddDate(0, 0, 31))
		if got <= 2 || got > 120 {
			t.Errorf("got %d, want in (2, 120]", got)
		}
		if got != 60 {
			t.Errorf("got %d, want 60", got)
		}
	})

	t.Run("95 day span picks 120", func(t *testing.T) {
		got := pickBucketMinutes(base, base.AddDate(0, 0, 95))
		if got != 120 {
			t.Errorf("got %d, want 120", got)
		}
	})

	t.Run("five year span caps at 1440", func(t *testing.T) {
		got := pickBucketMinutes(base, base.AddDate(5, 0, 0))
		if got != 1440 {
			t.Errorf("got %d, want 1440", got)
		}
	})

	t.Run("negative span returns 2", func(t *testing.T) {
		got := pickBucketMinutes(base.AddDate(0, 0, 1), base)
		if got != 2 {
			t.Errorf("got %d, want 2", got)
		}
	})

	t.Run("zero span returns 2", func(t *testing.T) {
		got := pickBucketMinutes(base, base)
		if got != 2 {
			t.Errorf("got %d, want 2", got)
		}
	})
}

func TestDownsampleDatasets(t *testing.T) {
	t.Run("bucketMinutes<=2 returns input unchanged", func(t *testing.T) {
		in := []Dataset{{
			Label: "gym",
			Data: []DataPoint{
				{X: "2025-10-01T10:00:00+03:00", Y: 6},
				{X: "2025-10-01T10:20:00+03:00", Y: 9},
			},
		}}
		for _, bm := range []int{0, 1, 2} {
			out := downsampleDatasets(in, bm)
			if len(out) != len(in) {
				t.Fatalf("bm=%d: len = %d, want %d", bm, len(out), len(in))
			}
			if len(out[0].Data) != len(in[0].Data) {
				t.Errorf("bm=%d: points = %d, want %d", bm, len(out[0].Data), len(in[0].Data))
			}
		}
	})

	t.Run("60 minute buckets average and preserve order", func(t *testing.T) {
		in := []Dataset{{
			Label: "gym",
			Data: []DataPoint{
				{X: "2025-10-01T10:00:00+03:00", Y: 6},
				{X: "2025-10-01T10:20:00+03:00", Y: 9},
				{X: "2025-10-01T10:40:00+03:00", Y: 12},
				{X: "2025-10-01T11:10:00+03:00", Y: 4},
			},
		}}
		out := downsampleDatasets(in, 60)
		if len(out) != 1 {
			t.Fatalf("datasets = %d, want 1", len(out))
		}
		pts := out[0].Data
		if len(pts) != 2 {
			t.Fatalf("points = %d, want 2", len(pts))
		}

		if got, want := pts[0].X, "2025-10-01T10:00:00+03:00"; got != want {
			t.Errorf("pts[0].X = %q, want %q", got, want)
		}
		if math.Abs(pts[0].Y-9) > 1e-9 {
			t.Errorf("pts[0].Y = %v, want 9", pts[0].Y)
		}

		if got, want := pts[1].X, "2025-10-01T11:00:00+03:00"; got != want {
			t.Errorf("pts[1].X = %q, want %q", got, want)
		}
		if math.Abs(pts[1].Y-4) > 1e-9 {
			t.Errorf("pts[1].Y = %v, want 4", pts[1].Y)
		}

		if pts[0].X >= pts[1].X {
			t.Errorf("chronological order not preserved: %q then %q", pts[0].X, pts[1].X)
		}
	})

	t.Run("averages rounded to one decimal", func(t *testing.T) {
		in := []Dataset{{
			Label: "gym",
			Data: []DataPoint{
				{X: "2025-10-01T10:00:00+03:00", Y: 1},
				{X: "2025-10-01T10:30:00+03:00", Y: 2},
				{X: "2025-10-01T10:50:00+03:00", Y: 2},
			},
		}}
		out := downsampleDatasets(in, 60)
		if len(out[0].Data) != 1 {
			t.Fatalf("points = %d, want 1", len(out[0].Data))
		}
		// (1+2+2)/3 = 1.6666... rounds to 1.7
		if math.Abs(out[0].Data[0].Y-1.7) > 1e-9 {
			t.Errorf("Y = %v, want 1.7", out[0].Data[0].Y)
		}
	})

	t.Run("invalid X points are skipped", func(t *testing.T) {
		in := []Dataset{{
			Label: "gym",
			Data: []DataPoint{
				{X: "garbage", Y: 100},
				{X: "2025-10-01T10:00:00+03:00", Y: 6},
			},
		}}
		out := downsampleDatasets(in, 60)
		if len(out[0].Data) != 1 {
			t.Fatalf("points = %d, want 1", len(out[0].Data))
		}
		if math.Abs(out[0].Data[0].Y-6) > 1e-9 {
			t.Errorf("Y = %v, want 6", out[0].Data[0].Y)
		}
	})
}
