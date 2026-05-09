#!/bin/bash
# register_sensors.sh
# Run once to register all monitoring targets

export PATH=$PATH:/home/tomek/go/src/github.com/tomekjarosik/qivitals/tmp

# ==========================================
# INFRASTRUCTURE & SECURITY
# ==========================================
qivitals-cli register --id cloud-jarosik-online --name "cloud.jarosik.online" \
    --namespace "infra" --description "TLS cert for Nextcloud" \
    --label "type=cert" --label "critical=true" --label "env=prod" \
    --graceful "30d" --failure "7d"

qivitals-cli register --id pass-jarosik-online --name "pass.jarosik.online" \
    --namespace "infra" --description "TLS cert for Bitwarden" \
    --label "type=cert" --label "critical=true" --label "env=prod" \
    --graceful "30d" --failure "7d"

qivitals-cli register --id s3-jarosik-online --name "s3.jarosik.online" \
    --namespace "infra" --description "TLS cert for S3" \
    --label "type=cert" --label "critical=true" \
    --graceful "30d" --failure "7d"

qivitals-cli register --id mail-jarosik-online --name "mail.jarosik.online" \
    --namespace "infra" --description "TLS cert for mail server" \
    --label "type=cert" --label "critical=true" --label "env=prod" \
    --graceful "30d" --failure "7d"

qivitals-cli register --id proxmox-cluster --name "Proxmox Cluster Health" \
    --namespace "infra" --description "Node status, CPU, RAM, cluster quorum" \
    --label "type=cluster" --label "critical=true" --label "env=prod" \
    --graceful "1h" --failure "4h"

qivitals-cli register --id nextcloud-sync --name "Nextcloud Scan/Sync" \
    --namespace "infra" --description "Last scan time, queue size, storage usage" \
    --label "type=app" --label "critical=true" --label "env=prod" \
    --graceful "2h" --failure "8h"

qivitals-cli register --id firewall-status --name "Firewall Status" \
    --namespace "security" --description "Active rules, blocked IPs, fail2ban state" \
    --label "type=network" --label "critical=true" \
    --graceful "15m" --failure "1h"

qivitals-cli register --id intrusion-detection --name "Intrusion Detection" \
    --namespace "security" --description "Failed logins, jail status, alerts" \
    --label "type=security" --label "critical=true" \
    --graceful "1h" --failure "4h"

qivitals-cli register --id unpatched-cves --name "Unpatched CVEs" \
    --namespace "security" --description "High/Critical unpatched vulnerabilities" \
    --label "type=security" --label "critical=true" \
    --graceful "24h" --failure "7d"

# ==========================================
# STORAGE & BACKUP
# ==========================================
qivitals-cli register --id zfs-pool-health --name "ZFS Pool Health" \
    --namespace "storage" --description "Scrub progress, errors, capacity, health" \
    --label "type=storage" --label "critical=true" \
    --graceful "1h" --failure "6h"

qivitals-cli register --id lxc-backup --name "LXC Backup" \
    --namespace "storage" --description "LXC container snapshots & replication to r730-a" \
    --label "type=backup" --label "critical=true" --label "env=prod" \
    --graceful "1d" --failure "3d"

qivitals-cli register --id offsite-backup --name "Offsite Backup Sync" \
    --namespace "storage" --description "Last sync success, age, size, integrity" \
    --label "type=backup" --label "critical=true" \
    --graceful "1d" --failure "3d"

qivitals-cli register --id restore-test --name "Backup Restore Test" \
    --namespace "storage" --description "Last successful restore date & method" \
    --label "type=backup" --label "critical=true" \
    --graceful "7d" --failure "14d"

qivitals-cli register --id bitwarden-backup --name "Bitwarden Backup" \
    --namespace "storage" --description "Vault export/import verification" \
    --label "type=backup" --label "env=prod" \
    --graceful "2d" --failure "5d"

# ==========================================
# NETWORK
# ==========================================
qivitals-cli register --id isp-uptime --name "ISP Uptime/Latency" \
    --namespace "network" --description "Ping success, jitter, packet loss" \
    --label "type=network" --label "critical=true" \
    --graceful "15m" --failure "1h"

qivitals-cli register --id pihole --name "Pi-hole/AdGuard" \
    --namespace "network" --description "Queries, blocked, DNSSEC, uptime" \
    --label "type=network" --label "critical=true" \
    --graceful "15m" --failure "1h"

qivitals-cli register --id wan-ip-ddns --name "WAN IP / DDNS" \
    --namespace "network" --description "IP change detection, DDNS sync status" \
    --label "type=network" \
    --graceful "2h" --failure "12h"

# ==========================================
# FINANCES
# ==========================================
qivitals-cli register --id net-worth --name "Net Worth Snapshot" \
    --namespace "finances" --description "Current total, trend vs last month" \
    --label "type=finance" --label "update=frequency" \
    --graceful "30d" --failure "60d"

qivitals-cli register --id monthly-budget --name "Monthly Budget vs Actual" \
    --namespace "finances" --description "Savings rate, overspend alerts" \
    --label "type=finance" \
    --graceful "30d" --failure "60d"

qivitals-cli register --id subscriptions --name "Subscription Renewals" \
    --namespace "finances" --description "Next 30 days, monthly cost, upcoming renewals" \
    --label "type=finance" \
    --graceful "30d" --failure "60d"

qivitals-cli register --id tax-deadlines --name "Tax Deadline Countdown" \
    --namespace "finances" --description "Next filing date, estimated liability" \
    --label "type=finance" --label "critical=true" \
    --graceful "90d" --failure "180d"

# ==========================================
# LIFESTYLE
# ==========================================
qivitals-cli register --id kwiatek-ananas --name "Kwiatek Ananas" \
    --namespace "lifestyle" --description "Czy kwiatek ananas został podlany" \
    --label "type=manual" --label "category=plant" \
    --graceful "3d" --failure "7d"

qivitals-cli register --id medication --name "Medication/Health Tracker" \
    --namespace "lifestyle" --description "Missed doses, next schedule" \
    --label "type=manual" --label "category=health" \
    --graceful "1d" --failure "3d"

qivitals-cli register --id cleaning --name "Cleaning/Maintenance" \
    --namespace "lifestyle" --description "Fridges, HVAC filters, gutters, appliances" \
    --label "type=manual" --label "category=home" \
    --graceful "7d" --failure "14d"

qivitals-cli register --id exercise --name "Exercise/Training Streak" \
    --namespace "lifestyle" --description "Days consistent, last workout" \
    --label "type=manual" --label "category=health" \
    --graceful "3d" --failure "7d"