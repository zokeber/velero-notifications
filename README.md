# Velero Notifications

Velero Notifications is a Golang-based controller designed to monitor Velero backups in your Kubernetes cluster and send notifications via Slack or Email when backups complete successfully or fail. The controller uses the Kubernetes dynamic client to query backup resources, track state changes, and notify only once when a new backup is finishes.

When running locally, the application uses your local kubeconfig. When deployed in-cluster, it automatically uses the in-cluster configuration.

## Features

- **Backup Monitoring:** Detects when new backups begin (`InProgress`) and notifies on completion or failure.
- **Notification Channels:** Sends notifications through Slack and Email (with support for additional channels in the future).
- **Customizable Configuration:** Fully configurable via a YAML file for logging, backup intervals, and notification settings.
- **Helm Chart Packaging:** Easily deployable in any Kubernetes cluster using our Helm chart.

## Usage

Velero Notifications continuously monitors Velero backups and sends notifications only when a backup that was in the InProgress state finishes (either successfully or with failure). Notifications include details such as start and completion times, progress (items backed up), warnings, and—for failed backups—the failure reason.

## Installation

### Using Helm Chart

Our Helm chart is hosted at [https://zokeber.github.io/velero-notifications/](https://zokeber.github.io/velero-notifications/). To install the chart:

1. **Add the Helm repository:**

```bash
helm repo add zokeber-velero-notifications https://zokeber.github.io/velero-notifications/
helm repo update
```

2. **Create the namespace (if not already created):**

```bash
kubectl create namespace velero
```

3. **Install the chart:**

```bash
helm install zokeber-velero-notifications velero-notifications/velero-notifications --namespace velero
```
4. **(Optional) Override default values:**

The application is configured via a values yaml file. Create a custom values file (e.g., custom-values.yaml) with your desired settings, then run:

```bash
helm upgrade zokeber-velero-notifications velero-notifications/velero-notifications --namespace velero -f custom-values.yaml
```

An example configuration (in the Helm chart's ) is:

```yaml
namespace: "velero"
check_interval: 60
notification_prefix: "[kubernetes-context] "
verbose: true

slack:
  enabled: false
  webhook_url: "https://hooks.slack.com/services/XXXXXXX"
  channel: "velero-notifications"
  username: "Velero"

email:
  enabled: true
  failures_only: false
  smtp_server: "smtp.gmail.com"
  smtp_port: 587
  username: "username@gmail.com"
  password: "password"
  from: "username@gmail.com"
  to: "notifications@gmail.com"
```

Please looking at the [Helm Chart Readme file](https://github.com/zokeber/velero-notifications/blob/main/charts/velero-notifications/README.md) to setting up or overriding some values.

## Contributing

We welcome contributions from the community! Please see the [CONTRIBUTING.md](CONTRIBUTING.md) file for details on our code style, branching model, and how to submit pull requests.

## License

This project is licensed under the MIT License.

## Contact

For any questions or issues, please open an issue on GitHub.