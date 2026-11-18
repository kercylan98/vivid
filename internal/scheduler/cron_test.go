package scheduler

import (
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/require"
)

func TestCronNext(t *testing.T) {
	for _, test := range []struct {
		name, expression, after, next string
	}{
		{"daily", "0 0 0 * * ?", "2026-11-18T01:00:00Z", "2026-11-19T00:00:00Z"},
		{"weekday", "0 30 9 ? * MON-FRI", "2026-11-20T10:00:00Z", "2026-11-23T09:30:00Z"},
		{"sunday", "0 0 0 ? * 0", "2026-11-18T00:00:00Z", "2026-11-22T00:00:00Z"},
		{"list", "0,30 */10 8-10 * * ?", "2026-11-18T08:00:00Z", "2026-11-18T08:00:30Z"},
		{"year", "0 0 0 1 1 ? 2700", "2026-11-18T01:00:00Z", "2700-01-01T00:00:00Z"},
		{"last_year", "0 0 0 1 1 ? 9999", "2026-11-18T01:00:00Z", "9999-01-01T00:00:00Z"},
		{"year_list", "0 0 0 1 1 ? 2020,2028,2030", "2026-11-18T01:00:00Z", "2028-01-01T00:00:00Z"},
		{"year_step", "0 0 0 1 1 ? 2020-2030/3", "2026-11-18T01:00:00Z", "2029-01-01T00:00:00Z"},
		{"year_wildcard_step", "0 0 0 1 1 ? */4", "2026-11-18T01:00:00Z", "2030-01-01T00:00:00Z"},
		{"leap_century", "0 0 0 29 2 ?", "2096-03-01T00:00:00Z", "2104-02-29T00:00:00Z"},
		{"leap_year_filter", "0 0 0 29 2 ? 2100,2104", "2096-03-01T00:00:00Z", "2104-02-29T00:00:00Z"},
		{"expired", "0 0 0 1 1 ? 2000", "2026-11-18T01:00:00Z", ""},
		{"impossible", "0 0 0 31 2 ?", "2026-11-18T01:00:00Z", ""},
		{"end", "* * * * * *", "9999-12-31T23:59:59Z", ""},
		{"descriptor", "@daily", "2026-11-18T01:00:00Z", "2026-11-19T00:00:00Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, err := ParseCron(test.expression, time.UTC)
			require.NoError(t, err)
			after, err := time.Parse(time.RFC3339, test.after)
			require.NoError(t, err)
			next := plan.Next(after)
			if test.next == "" {
				require.True(t, next.IsZero(), "没有后续时间时返回零值")
			} else {
				require.Equal(t, test.next, next.Format(time.RFC3339))
				require.True(t, next.After(after))
			}
		})
	}
}

func TestCronRejectsInvalidExpression(t *testing.T) {
	for _, expression := range []string{
		"", "* * * * *", "invalid", "TZ=UTC", "@every 1s", "0 0 0 * * ? 2020/0",
		"0 0 0 * * ? 10000", "0 0 0 * * ? 1969", "0 0 0 * * ? 2030-2020", "0 0 0 * * ? 2020,,2030",
		"0 0 0 * * ? */999999999999999999999", "99,100 * * * * *", "* 80,90 * * * *", "0 0 24 * * ?",
		"0 0 0 1,32 * ?", "0 0 0 * 0,13 ?", "0 0 0 ? * 7,8", "0 0 0 1 * MON", "0 0 0 L * ?", "0 0 0 1W * ?", "0 0 0 ? * MON#2",
	} {
		t.Run(expression, func(t *testing.T) {
			plan, err := ParseCron(expression, time.UTC)
			require.ErrorIs(t, err, vivid.ErrorCronParse)
			require.Nil(t, plan)
		})
	}
}

func TestCronTimeZone(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	for _, test := range []struct{ expression, after, next string }{
		{"0 30 2 * * ?", "2027-03-13T08:00:00Z", "2027-03-15T06:30:00Z"},
		{"0 30 1 * * ?", "2027-11-07T05:30:00Z", "2027-11-07T06:30:00Z"},
	} {
		plan, err := ParseCron(test.expression, location)
		require.NoError(t, err)
		after, err := time.Parse(time.RFC3339, test.after)
		require.NoError(t, err)
		require.Equal(t, test.next, plan.Next(after).UTC().Format(time.RFC3339))
	}
	plan, err := ParseCron("0 0 0 * * ?", nil)
	require.NoError(t, err)
	after := time.Date(2026, 11, 18, 1, 0, 0, 0, time.Local)
	require.Equal(t, time.Date(2026, 11, 19, 0, 0, 0, 0, time.Local), plan.Next(after))
}

func FuzzCronParse(f *testing.F) {
	for _, expression := range []string{"* * * * * *", "0 0 0 1 1 ? 9999", "0,99 * * * * *", "TZ=", "0 0 0 29 2 ? 2100/2"} {
		f.Add(expression)
	}
	f.Fuzz(func(t *testing.T, expression string) {
		if len(expression) > 512 {
			t.Skip()
		}
		plan, err := ParseCron(expression, time.UTC)
		if err == nil {
			after := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
			next := plan.Next(after)
			require.True(t, next.IsZero() || next.After(after))
		}
	})
}
