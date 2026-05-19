# Sensors


## Specification

This package aims to deliver most used sensors. Each sensor reports data in a key=value format. 
We can have pre-built sensors for ZFS, proxmox, storage in general, compute, hardware etc.

One sensor is generic-json which accepts data in json format, where each top level key is a string with a value string


We can have sensors like ZPOOL sensor
```bash
qivitals-cli report --id <sensorid> zpool <poolname>
```
This sensor reports if storage is healthy or not. In the future, we will have "conditions" in the sensor specification on the server side.
We will specify conditions that apply to the reported data. (and of course, when data is not reported at all)



## Implementation details

We use Cobra commands, but we do want to have dynamic way of defining a sensor. SensorDevice will be an interface
which will be implemented by speficic sensor devices. Each SensorDevice will need a type (like "zpool").
We will also need some other sub-commands somehow (e.g. zpool requires poolname, otherwise it does not make much sense).
How to do that dynamically so command feels natural is to be solved.