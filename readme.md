# Local Storage Sync for microk8s-hostpath 

The goal is to be able to sync stateful sets between the different nodes of a cluster to allow the data to be replicated and persist between all the nodes so in case they get rescheduled to a different node their data will be accessible and updated.

You will need to add the label _syncronize-nodes: true_ to the volume claim template in order to enable syncronization.

Documentation still work in progress
