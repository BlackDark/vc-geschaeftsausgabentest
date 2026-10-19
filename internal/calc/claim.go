package calc

// ClaimTotals are the combined claim sums (C21).
type ClaimTotals struct {
	Travel        Cents `json:"travel"`
	Outlays       Cents `json:"outlays"`
	Reimbursement Cents `json:"reimbursement"`
	Advances      Cents `json:"advances"`
	Payout        Cents `json:"payout"`
	Repayment     bool  `json:"repayment"`
}

// Totals sums the selected trips and outlays and subtracts advances.
func (r *Result) Totals(tripIDs []int64, outlayIDs []int64, advances []Cents) ClaimTotals {
	var t ClaimTotals
	for _, id := range tripIDs {
		if tr, ok := r.Trips[id]; ok {
			t.Travel += tr.Totals.Total
		}
	}
	selected := map[int64]bool{}
	for _, id := range outlayIDs {
		selected[id] = true
	}
	for _, o := range r.Outlays {
		if selected[o.ExpenseID] {
			t.Outlays += o.Reimbursable
		}
	}
	for _, a := range advances {
		t.Advances += a
	}
	t.Reimbursement = t.Travel + t.Outlays
	t.Payout = t.Reimbursement - t.Advances
	t.Repayment = t.Payout < 0
	return t
}
