package setting

// ReferralCommissionPercent is the share of a referred customer's paid top-up
// credited to their inviter. Zero disables the commission; 100 would pay out
// the whole top-up, so it is clamped when the payouts are computed.
var ReferralCommissionPercent = 0.0
