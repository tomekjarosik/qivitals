#!/bin/bash

export PATH=$PATH:/home/tomek/go/src/github.com/tomekjarosik/one-status

sensorcli register --name "cloud.jarosik.online" --namespace "infra" \
    --description "TLS cert for Nextcloud" \
    --label "tls=1" \
    --graceful 5184000 --failure 7776000

sensorcli register --name "pass.jarosik.online" --namespace "infra" \
    --description "TLS cert for Bitwarden" \
    --label "tls=1" \
    --graceful 5184000 --failure 7776000

sensorcli register --name "s3.jarosik.online" --namespace "infra" \
    --description "TLS cert for S3" \
    --label "tls=1" \
    --graceful 5184000 --failure 7776000

sensorcli register --namespace "homelab" --name "Bitwarden Backup" --label "env=prod"

sensorcli register --name "Thames Water" --namespace "home" \
    --description "Monthly water utility bill" \
    --graceful 2592000 --failure 3024000 \
    --label "category=bills" --label "type=manual"

sensorcli register --name "Kwiatek Ananas" --namespace "home" \
    --description "Czy kwiatek ananas zostal podlany" \
    --graceful 2592000 --failure 3024000 \
    --label "category=bills" --label "type=manual"

sensorcli register --name "Proxmox VM Replication" --namespace "infra" \
    --description "Have VMs replicated to r730-a" \
    --graceful 2592000 --failure 3024000 \
    --label "env=prod" --label "proxmox=1"
