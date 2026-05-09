#!/bin/bash
# report_sensors.sh
# Example reporting script - run via cron or manual trigger

export PATH=$PATH:/home/tomek/go/src/github.com/tomekjarosik/qivitals/tmp

# Helper to log execution
log_report() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"; }

# ==========================================
# INFRASTRUCTURE & SECURITY
# ==========================================
log_report "Reporting infra/security"
qivitals-cli report --id cloud-jarosik-online --data="days_remaining=42" --data="issuer=Let's Encrypt" --data="domain=cloud.jarosik.online"
qivitals-cli report --id pass-jarosik-online --data="days_remaining=38" --data="issuer=Let's Encrypt" --data="domain=pass.jarosik.online"
qivitals-cli report --id s3-jarosik-online --data="days_remaining=41" --data="issuer=Let's Encrypt" --data="domain=s3.jarosik.online"
qivitals-cli report --id mail-jarosik-online --data="days_remaining=35" --data="issuer=Let's Encrypt" --data="domain=mail.jarosik.online"

qivitals-cli report --id proxmox-cluster --data="nodes_online=3" --data="total_cpu=128cores" --data="total_memory=128GB" --data="cpu_usage=23%" --data="memory_usage=61%"
qivitals-cli report --id nextcloud-sync --data="last_scan=1h_ago" --data="storage_used=1.2TB" --data="active_users=3" --data="queue_size=0"

qivitals-cli report --id firewall-status --data="active_rules=42" --data="blocked_ips=15" --data="uptime=45d" --data="status=active"
qivitals-cli report --id intrusion-detection --data="jail_count=3" --data="banned_ips=8" --data="recent_attempts=12" --data="status=healthy"
qivitals-cli report --id unpatched-cves --data="critical=0" --data="high=2" --data="medium=5" --data="pending_updates=7"

# ==========================================
# STORAGE & BACKUP
# ==========================================
log_report "Reporting storage/backup"
qivitals-cli report --id zfs-pool-health --data="status=ONLINE" --data="scrub_progress=82%" --data="errors=0" --data="capacity_used=64%"
qivitals-cli report --id lxc-backup --data="status=success" --data="containers_backed_up=12" --data="size=48.2GB" --data="duration=14m"
qivitals-cli report --id offsite-backup --data="status=success" --data="sync_age=0h_ago" --data="verified=true" --data="size=48.2GB"
qivitals-cli report --id restore-test --data="last_test=2026-05-01" --data="method=full_restore" --data="result=success" --data="duration=22m"
qivitals-cli report --id bitwarden-backup --data="status=success" --data="vault_size=14MB" --data="last_export=2026-05-08"

# ==========================================
# NETWORK
# ==========================================
log_report "Reporting network"
qivitals-cli report --id isp-uptime --data="ping_ms=14" --data="packet_loss=0%" --data="uptime=99.98%" --data="jitter=2ms"
qivitals-cli report --id pihole --data="queries=14520" --data="blocked=3421" --data="dns_sec=active" --data="uptime=45d"
qivitals-cli report --id wan-ip-ddns --data="current_ip=203.0.113.42" --data="changed=0d_ago" --data="ddns_sync=ok"

# ==========================================
# FINANCES
# ==========================================
log_report "Reporting finances"
qivitals-cli report --id net-worth --data="total=184500" --data="currency=PLN" --data="trend=+2.3%" --data="month=May_2026"
qivitals-cli report --id monthly-budget --data="spent=1250" --data="limit=1500" --data="currency=PLN" --data="remaining_savings=250"
qivitals-cli report --id subscriptions --data="count=5" --data="monthly_cost=45" --data="next_renewal=2026-06-15" --data="overdue=0"
qivitals-cli report --id tax-deadlines --data="due_in=42d" --data="estimated=12000" --data="filing_type=JPK" --data="status=on_track"

# ==========================================
# LIFESTYLE
# ==========================================
log_report "Reporting lifestyle"
qivitals-cli report --id kwiatek-ananas --data="watered=2d_ago" --data="soil_moisture=35%" --data="next_water=3d"
qivitals-cli report --id medication --data="medication_taken=1" --data="next_dose=08:00" --data="sleep_hours=7.2" --data="status=ok"
qivitals-cli report --id cleaning --data="last_cleaned=3d_ago" --data="next_task=filters" --data="status=due"
qivitals-cli report --id exercise --data="streak=12d" --data="last_session=2026-05-08" --data="type=strength" --data="duration=45m"

log_report "All reports sent."