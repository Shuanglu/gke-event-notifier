# GKE-Event-Notifier 

## Introduction 
A golang Cloud Function which subscribes the GKE events from a PubSub topic and sends the event like below to Slack channel via webhook

```
MASTER of cluster <cluster name> is upgrading from version <cluster version> to version <cluster version>.

The operation started at <operation start timestamp>

To check the operation detail, please run gcloud container operations --project <project id> describe  <operation id> --region <region name>

To cancel the operation, please run gcloud container operations --project <project id> cancel <operation id> --region <region name>
```

## Reference 
- [Configure cluster notifications for third-party services](https://cloud.google.com/kubernetes-engine/docs/tutorials/cluster-notifications-slack)
- [Slack API in Go](https://github.com/slack-go/slack)