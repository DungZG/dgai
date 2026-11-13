package setting

// ChannelHealthAlertSuccessRate is the success rate (percent) below which a
// channel triggers an administrator notice.
var ChannelHealthAlertSuccessRate = 80.0

// ChannelHealthAlertMinRequests keeps low-traffic channels from alerting on a
// single failure; a channel must serve at least this many requests in the
// window before its rate is considered meaningful.
var ChannelHealthAlertMinRequests int64 = 20

// SpendVelocityMultiplier is how many times a user's own daily average they may
// spend today before an administrator is told.
var SpendVelocityMultiplier = 3.0

// SpendVelocityMinQuota keeps the alert quiet for accounts too small to matter.
var SpendVelocityMinQuota int64 = 500000 // 1 USD
