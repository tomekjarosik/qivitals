# TODOs

Small:
[ ] - add version like in Geranos
[ ] - make sure that "not reporting data" has highest priority (over conditions)
[ ] - kubernetes-like patching based on a file (rename 'update' to 'patch')
[ ] - To achieve the ultimate Kubernetes-like experience (similar to kubectl get sensors -o yaml > patch.yaml),
you should replace your boolean --machine flag with an --output (or -o) string flag that supports json and yaml.

Long term:
[ ] - implement system labels (e.g. owner.sys.qivitals, silenced.sys.qivitals)
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



