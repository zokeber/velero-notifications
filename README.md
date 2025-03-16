# Helm Chart Velero Notifications Repository

## Add Velero Notifications repository

In order to install velero-notificacionts, first add this repository to your Helm repos:

```bash
helm repo add zokeber-velero-notifications https://zokeber.github.io/velero-notifications/
```

After that, you can run `helm search repo zokeber-velero-notifications` to see the charts.


## Install Velero Notifications

```bash
helm install velero-notifications zokeber-velero-notifications/velero-notifications
```

## Overriding Chart Values

Please looking at the [Helm Chart Readme file](https://github.com/zokeber/velero-notifications/blob/main/charts/velero-notifications/README.md) to setting up or overriding some values.