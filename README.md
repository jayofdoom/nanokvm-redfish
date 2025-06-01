# nanokvm-redfish

This is a hastily coded redfish server meant to run on a NanoKVM and
provide power status and control. It's been tested working against
https://opendev.org/openstack/sushy.

This was mostly vibe coded with claude-code, if you run it in production
without lots of testing please don't blame me :).

My final goal is to make NanoKVM-managed servers be able to be controllable
with https://opendev.org/openstack/ironic.
