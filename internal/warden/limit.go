package warden

import (
	"context"
	"fmt"
	"io"
)

type ExistingWorklogCounter interface {
	ExistingWorklogSeconds(ctx context.Context, date string, author string) (int, error)
}

func ValidateDailyWorklogLimit(
	ctx context.Context,
	worklogs []Worklog,
	counter ExistingWorklogCounter,
	author string,
	maxHoursPerDay float64,
	out io.Writer,
) error {
	plannedByDate := map[string]int{}
	for _, worklog := range worklogs {
		plannedByDate[worklog.StartedDate] += worklog.TimeSpentSeconds
	}

	maxSeconds := int(maxHoursPerDay * 3600)
	for date, plannedSeconds := range plannedByDate {
		existingSeconds, err := counter.ExistingWorklogSeconds(ctx, date, author)
		if err != nil {
			return err
		}

		existingHours := float64(existingSeconds) / 3600
		plannedHours := float64(plannedSeconds) / 3600
		totalHours := float64(existingSeconds+plannedSeconds) / 3600
		_, _ = fmt.Fprintf(
			out,
			"%s уже списано %.2fч + планируется %.2fч = %.2fч / %.2fч\n",
			date,
			existingHours,
			plannedHours,
			totalHours,
			maxHoursPerDay,
		)

		if existingSeconds+plannedSeconds > maxSeconds {
			return fmt.Errorf(
				"%s превысит дневной лимит worklog: уже списано %.2fч + планируется %.2fч > %.2fч",
				date,
				existingHours,
				plannedHours,
				maxHoursPerDay,
			)
		}
	}

	return nil
}
