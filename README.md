# QiVitals

This is QiVitals - system health at a glance.

The idea behind it is to have a single status page that will connect sensor from various sources.
It could be a person reporting they did something, or zfs storage reporting healthy scrub.
It could be a TLS certificate check. The main difference to other is that sensors need to report regularily;
otherwise they will go red. It's for cases like you did a cron job which ran successfully but then it stopped after upgrade.
Or a backup job to other location that worked few times but broke at some point. Or maybe you needed to review 
your subscriptions every 3 months, but it's already been a year and you overpaid for something you wanted to just try.


This is something that I'm working on for my own use. A lot of important things are too challenging to expose to a public page.
Sometimes simply not possible, like checking if once-a-week backup on a PC that is mostly working ran successfully. 
Such check would be red most of the time in normal status dashboards.

### Configuration
```yaml
server:
  address: localhost:50051
  tls_key_file: /etc/default/qivitals/server.key
  tls_cert_file: /home/default/qivitals/server.crt
  database_url: naive-file # put proper postgres url here
  auth:
    users:
      bot1:
        publicKeys:
          - ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILQ959Oxgkd7i4RMvhQCstZUPlLD3L89cxIVM/dWxvk+ qivitals-cli
        namespaces:
          - default
          - storage
          - tls
```

```yaml
cli:
  url: localhost:50051
  identity:
    username: bot1
    keyPath: /home/bot1/.qivitals/key
```