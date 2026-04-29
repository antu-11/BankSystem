package email

import "fmt"

// RenderTemplate returns the subject line and HTML body for a given email job.
func RenderTemplate(job Job) (subject, body string) {
	switch job.Type {
	case JobWelcome:
		return renderWelcome(job)
	case JobTransactionSuccess:
		return renderTransactionSuccess(job)
	case JobTransactionFailed:
		return renderTransactionFailed(job)
	default:
		return "OmniLedger Notification", "<p>You have a new notification from OmniLedger.</p>"
	}
}

func renderWelcome(job Job) (string, string) {
	subject := "Welcome to OmniLedger 🏦"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: 'Segoe UI', Arial, sans-serif; background: #0f0f0f; color: #e0e0e0; padding: 40px;">
  <div style="max-width: 560px; margin: auto; background: #1a1a2e; border-radius: 12px; padding: 32px; border: 1px solid #2a2a4a;">
    <h1 style="color: #a78bfa; margin-top: 0;">Welcome to OmniLedger</h1>
    <p>Hello <strong>%s</strong>,</p>
    <p>Your account has been created successfully. You are now part of the most secure banking system ever built.</p>
    <hr style="border-color: #2a2a4a;">
    <p style="font-size: 13px; color: #888;">If you did not create this account, please contact support immediately.</p>
  </div>
</body>
</html>`, job.Username)
	return subject, body
}

func renderTransactionSuccess(job Job) (string, string) {
	amount := job.ExtraData["amount"]
	txnID := job.ExtraData["transaction_id"]
	direction := job.ExtraData["direction"]

	subject := fmt.Sprintf("Transaction Successful — ₹%s %s", amount, direction)
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: 'Segoe UI', Arial, sans-serif; background: #0f0f0f; color: #e0e0e0; padding: 40px;">
  <div style="max-width: 560px; margin: auto; background: #1a1a2e; border-radius: 12px; padding: 32px; border: 1px solid #2a2a4a;">
    <h1 style="color: #34d399; margin-top: 0;">Transaction Successful ✓</h1>
    <p>Hello <strong>%s</strong>,</p>
    <p>Your transaction has been completed successfully.</p>
    <table style="width: 100%%; border-collapse: collapse; margin: 16px 0;">
      <tr><td style="padding: 8px; color: #888;">Amount</td><td style="padding: 8px; font-weight: bold;">₹%s</td></tr>
      <tr><td style="padding: 8px; color: #888;">Direction</td><td style="padding: 8px;">%s</td></tr>
      <tr><td style="padding: 8px; color: #888;">Transaction ID</td><td style="padding: 8px; font-family: monospace; font-size: 12px;">%s</td></tr>
    </table>
    <hr style="border-color: #2a2a4a;">
    <p style="font-size: 13px; color: #888;">This is an automated notification from OmniLedger.</p>
  </div>
</body>
</html>`, job.Username, amount, direction, txnID)
	return subject, body
}

func renderTransactionFailed(job Job) (string, string) {
	amount := job.ExtraData["amount"]
	reason := job.ExtraData["reason"]

	subject := fmt.Sprintf("Transaction Failed — ₹%s", amount)
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: 'Segoe UI', Arial, sans-serif; background: #0f0f0f; color: #e0e0e0; padding: 40px;">
  <div style="max-width: 560px; margin: auto; background: #1a1a2e; border-radius: 12px; padding: 32px; border: 1px solid #2a2a4a;">
    <h1 style="color: #f87171; margin-top: 0;">Transaction Failed ✗</h1>
    <p>Hello <strong>%s</strong>,</p>
    <p>Unfortunately, your transaction of <strong>₹%s</strong> could not be processed.</p>
    <table style="width: 100%%; border-collapse: collapse; margin: 16px 0;">
      <tr><td style="padding: 8px; color: #888;">Amount</td><td style="padding: 8px; font-weight: bold;">₹%s</td></tr>
      <tr><td style="padding: 8px; color: #888;">Reason</td><td style="padding: 8px; color: #f87171;">%s</td></tr>
    </table>
    <hr style="border-color: #2a2a4a;">
    <p style="font-size: 13px; color: #888;">Please retry or contact support if the issue persists.</p>
  </div>
</body>
</html>`, job.Username, amount, amount, reason)
	return subject, body
}
