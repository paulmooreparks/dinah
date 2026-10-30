package judge

import (
	"sort"
	"time"
)

// A budget is how much a provider's judgement calls may cost in a period, in the
// provider's own unit (docs/plans/judgement-agents.md, step 4). It is spent
// evenly: at any moment the allowance is the limit times the share of the period
// that has passed, and a call is sent only while what has been spent is no more than the
// allowance. A call's cost is known only when it returns, so one call may take the
// total past the allowance, and the next calls wait for the allowance to catch up.

// WhyBudget is the reason a call was answered by its rule because the budget did
// not allow it.
const WhyBudget = "budget"

// Budget is a provider's budget. A nil Limit is none, which is the default; 0 is
// a budget that refuses every call.
type Budget struct {
	Limit *int
	Unit  string
}

// Period is when the weekly period begins: the weekday, 0 for Sunday, and the
// hour, in the computer's local time.
type Period struct {
	Weekday int
	Hour    int
}

// DefaultPeriod is Monday at midnight.
var DefaultPeriod = Period{Weekday: int(time.Monday), Hour: 0}

// PeriodAt is the week that contains now: it begins at the latest weekday and hour
// that is not after now, in now's own time zone, and ends a week later.
func PeriodAt(now time.Time, p Period) (start, end time.Time) {
	y, m, d := now.Date()
	start = time.Date(y, m, d, p.Hour, 0, 0, 0, now.Location())
	for start.Weekday() != time.Weekday(p.Weekday) {
		start = start.AddDate(0, 0, -1)
	}
	if start.After(now) {
		start = start.AddDate(0, 0, -7)
	}
	return start, start.AddDate(0, 0, 7)
}

// Allowance is what may have been spent by now: the limit times the share of the
// period that has passed.
func Allowance(limit int, start, end, now time.Time) float64 {
	if !now.After(start) {
		return 0
	}
	if !now.Before(end) {
		return float64(limit)
	}
	return float64(limit) * float64(now.Sub(start)) / float64(end.Sub(start))
}

// BudgetUse is a budget as it stands, for the widget and the page.
type BudgetUse struct {
	Provider  string  `json:"provider"`
	Unit      string  `json:"unit"`
	Limit     int     `json:"limit"`
	Spent     int     `json:"spent"`
	Allowance float64 `json:"allowance"`
	Left      int     `json:"left"`
	// Unmeasured is how many calls in the period the provider did not report the
	// cost of, which Spent does not include. While it is not 0 the budget cannot be
	// held to, and the page says so in words instead of showing Spent as a figure.
	Unmeasured  int       `json:"unmeasured"`
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`
}

// SetBudgets sets the budgets, by provider, and the period they are spent over.
func (j *Judges) SetBudgets(budgets map[string]Budget, period Period) {
	j.mu.Lock()
	j.budgets, j.period = budgets, period
	j.mu.Unlock()
}

// BudgetUses is every budget that has a limit, as it stands now, in the order of
// the providers' names.
func (j *Judges) BudgetUses() ([]BudgetUse, error) {
	j.mu.Lock()
	budgets, period := j.budgets, j.period
	j.mu.Unlock()
	now := j.now()
	start, end := PeriodAt(now, period)
	names := make([]string, 0, len(budgets))
	for provider := range budgets {
		names = append(names, provider)
	}
	sort.Strings(names)
	var out []BudgetUse
	for _, provider := range names {
		b := budgets[provider]
		if b.Limit == nil {
			continue
		}
		spent, err := j.store.Spent(provider, b.Unit, start, now.Add(time.Millisecond))
		if err != nil {
			return nil, err
		}
		unmeasured, err := j.store.Unmeasured(provider, start, now.Add(time.Millisecond))
		if err != nil {
			return nil, err
		}
		out = append(out, BudgetUse{Provider: provider, Unit: b.Unit, Unmeasured: unmeasured, Limit: *b.Limit, Spent: spent, Allowance: Allowance(*b.Limit, start, end, now),
			Left: max(*b.Limit-spent, 0), PeriodStart: start, PeriodEnd: end})
	}
	return out, nil
}

// overBudget says the provider's budget does not allow another call now: what has
// been spent in the period is more than the allowance, or the budget is 0. At the
// start of a period both are nothing, so the first call goes, and from then on
// spending is held to the even pace with the overshoot of one call.
func (j *Judges) overBudget(provider string) bool {
	j.mu.Lock()
	b, ok := j.budgets[provider]
	period := j.period
	j.mu.Unlock()
	if !ok || b.Limit == nil || j.store == nil {
		return false
	}
	now := j.now()
	start, end := PeriodAt(now, period)
	spent, err := j.store.Spent(provider, b.Unit, start, now.Add(time.Millisecond))
	if err != nil {
		j.logf("judge: the spending of %s could not be read, so its budget is not applied: %v", provider, err)
		return false
	}
	return *b.Limit == 0 || float64(spent) > Allowance(*b.Limit, start, end, now)
}
