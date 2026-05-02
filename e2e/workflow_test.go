package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- Real-World Workflows ---

func TestWorkflow_MonthlyBills(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	// 1. Setup: A sensor for a monthly water bill (graceful: 30 days, failure: 35 days)
	days30 := int64(30 * 24 * 60 * 60)
	days35 := int64(35 * 24 * 60 * 60)
	waterBillID := Register(t, "home", "water-bill", "Monthly water utility bill", days30, days35, "category=bills", "type=manual")

	// 2. Action: Human clicks "I paid this" (sends a report)
	Report(t, waterBillID, "paid_amount=45.50", "method=bank_transfer")

	// 3. Verify: Bill is marked as ACTIVE (paid)
	RequireState(t, waterBillID, "ACTIVE")

	// 4. Verify details
	res := Query(t, "--namespace", "home", "--name", "water-bill")
	assert.Equal(t, "45.50", res.Sensors[0].Status.ReportedData["paid_amount"])
}

func TestWorkflow_ITInfrastructure(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	// 1. Register TLS Certificate monitor (graceful: 60 days, failure: 90 days)
	tlsID := Register(t, "infra", "tls-jarosik-online", "TLS cert for main domain", 60*24*3600, 90*24*3600)

	// 2. Register Backup Monitor (graceful: 25 hours, failure: 48 hours)
	backupID := Register(t, "infra", "backup-proxmox-nextcloud", "Daily Nextcloud VM backup", 25*3600, 48*3600)

	// 3. Register VM Alive Ping (graceful: 5 mins, failure: 15 mins)
	vmPingID := Register(t, "infra", "ping-nextcloud-vm", "Nextcloud internal health endpoint", 300, 900)

	// --- Simulate automated Cron Jobs running ---

	// Cert bot runs and reports 65 days remaining
	Report(t, tlsID, "days_remaining=65")

	// Backup script finishes successfully
	Report(t, backupID, "size_gb=14.2", "duration_sec=450")

	// Uptime kuma / ping script runs
	Report(t, vmPingID, "latency_ms=4")

	// --- Verifications ---
	RequireState(t, tlsID, "ACTIVE")
	RequireState(t, backupID, "ACTIVE")
	RequireState(t, vmPingID, "ACTIVE")
}

func TestWorkflow_HomeNetwork(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	// Router pings 8.8.8.8 every minute. Graceful=2 mins, Failure=5 mins
	internetID := Register(t, "network", "isp-connection", "Internet connectivity check", 120, 300, "location=home")

	// Router sends OK
	Report(t, internetID, "packet_loss=0%")
	RequireState(t, internetID, "ACTIVE")
}

func TestWorkflow_FamilyChores(t *testing.T) {
	serverCmd := startTestServer(t)
	defer serverCmd.Process.Kill()

	// Dog needs feeding twice a day (grace: 14 hours)
	dogID := Register(t, "chores", "feed-dog", "Feed the dog", 14*3600, 24*3600)

	// Plants need water every 3 days
	plantsID := Register(t, "chores", "water-plants", "Water living room plants", 3*24*3600, 5*24*3600)

	// Kid presses NFC button next to dog bowl
	Report(t, dogID, "feeder=timmy")
	RequireState(t, dogID, "ACTIVE")

	// No one watered plants yet, but we just registered it, so it might technically be DEAD depending on initialization logic.
	// We simulate a report to make it active.
	Report(t, plantsID)
	RequireState(t, plantsID, "ACTIVE")
}
