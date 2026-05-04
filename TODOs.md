One Status page.

Problem: Currently many status pages are asking for status of some endpoint, and only then they report success of failure.
This approach is limited because such status tools only provide "public" layer.
We need to design system that will be able to support all possible statuses. From private: does this backup succeeded.
To physical: did I contact a friend in last 3 months.
The solution is to have periodic "check in" from source providers.
We can have various signals:

Bot Push signal:
for example, a script will publish its ok/failure status to "One Status" using its unique key (let's use short ed25519 private/public keys).

Human push signal:
a human needs to click "I did this"

Pull signal
this is similar to other status pages, like is that https website working, is TLS cert valid etc


We will store a last timestamp of such signal (and also make sure we don't allow for too many writes per second,
if last timestamp is withing configurable interval like last minute, do not overwrite)


Each "Status check" will be fed by one or many "signals". Each "Status check" will have (key,value) 
pairs associated with it to allow for custom filtering and preparing status checklists.

It must be able to support checklists in daily life, recurring checklist (e.g. are all my receipts paid),
and automatic. We can then easily prepare a signal from email, which will forward email to one 
of signal producers and produce a signal like "bill for water has been paid", and then "Bills checklist" will be green



The project must be in Go. Pages must be as static as possible (eg. generated periodically and served from RAM)
It must be HTTP gateway, with protobuf and gRPC, and postgres as backend for storage


Sensorcli will communicate over gRPC - it must have basic functionality to read,update,list sensors

# TODOs

Small:
[ ] - use "last update timestamp"

Long term:
[ ] - add public key to SensorInfo or hash of a password/token for WebUI
[ ] - add sensor data evaluation function to SensorInfo - could be few predefined at first
[ ] - add functionality to make status "reviewed" / "casual"
[ ] - figure out easiest sensors to fix first (WebUI)
[ ] - implement streaks - how often a sensor did not fail
[ ] - implement "Nudge" service that will send reminders
[ ] - webUI: namespace health percentage and "streaks" - how often a sensor did not fail
[ ] - update statuses in DB based on current time: find all statuses that need update first, and then calculate their update
[ ] - webUI: add ability to edit statuses grace period and failure period
[ ] - webUI: add ability to add built-in sensors like DNS domains or TLS certs