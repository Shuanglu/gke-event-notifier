package gkeeventnotifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/slack-go/slack"
)

type PubSubMessage struct {
	Data       []byte            `json:"data"`
	Attributes map[string]string `json:"attributes"`
}

type UpgradePayload struct {
	ResourceType       string `json:"resourceType,omitempty"`
	Operation          string `json:"operation,omitempty"`
	OperationStartTime string `json:"operationStartTime,omitempty"`
	CurrentVersion     string `json:"currentVersion,omitempty"`
	TargetVersion      string `json:"targetVersion,omitempty"`
	Resource           string `json:"resource,omitempty"`
}

var (
	upgradeEventRe       = regexp.MustCompile(`.*UpgradeEvent.*`)
	nodepoolRe           = regexp.MustCompile(`/nodePools/.*`)
	webhookURL           = os.Getenv("SLACK_WEBHOOK")
	projectID            = os.Getenv("PROJECT_ID")
	slackMessage         = slack.WebhookMessage{}
	markdownElementType  = "mrkdwn"
	plaintextElementType = "plain_text"
)

func GkeEventNotifier(ctx context.Context, psm PubSubMessage) error {
	log.Println("Receiving PubSubMessage")
	if webhookURL == "" {
		log.Panicf("Missing slack webhook URL")
	}
	if upgradeEventRe.MatchString(psm.Attributes["type_url"]) {
		slackMessage = UpgradeEvent(ctx, psm, slackMessage)
	} else {
		slackMessage = SecurityEvent(ctx, psm, slackMessage)
	}
	err := slack.PostWebhookContext(ctx, webhookURL, &slackMessage)
	if err != nil {
		log.Panicf("Failed to send message to slack channel: %v", err)
	}
	log.Printf("Sent message to slack channel")
	return nil
}

func UpgradeEvent(ctx context.Context, psm PubSubMessage, slackMessage slack.WebhookMessage) slack.WebhookMessage {
	payloadByte := []byte(psm.Attributes["payload"])
	var upgradePayload UpgradePayload
	err := json.Unmarshal(payloadByte, &upgradePayload)
	if err != nil {
		log.Panicf("Failed to unmarshal 'payload' of the upgrade event: %v", err)
	}
	var headerText string
	if upgradePayload.ResourceType == "MASTER" {
		headerText = fmt.Sprintf("%v of cluster %v is upgrading from version %v to version %v.", upgradePayload.ResourceType, psm.Attributes["cluster_name"], upgradePayload.CurrentVersion, upgradePayload.TargetVersion)
	} else {
		nodepoolTmp := nodepoolRe.FindAllString(upgradePayload.Resource, -1)
		if len(nodepoolTmp) <= 0 {
			log.Panicf("The type of the upgraded resource is %q but failed to find nodepool name from the pubsub message %q", upgradePayload.ResourceType, upgradePayload.Resource)
		}
		// always use the last match as nodepool name
		nodepoolName := strings.Split(nodepoolTmp[len(nodepoolTmp)-1], "/")[2]
		headerText = fmt.Sprintf("%v %v of cluster %v is upgrading from version %v to version %v.", upgradePayload.ResourceType, nodepoolName, psm.Attributes["cluster_name"], upgradePayload.CurrentVersion, upgradePayload.TargetVersion)
	}
	// build message blocks --> https://github.com/slack-go/slack/blob/master/examples/blocks/blocks.go

	// header
	var headerTextBlockObject *slack.TextBlockObject
	var headerBlock slack.Block
	headerTextBlockObject = slack.NewTextBlockObject(plaintextElementType, headerText, false, false)
	if len(headerText) > 150 {
		// header text has 150 characters limit. need to fall back to sectionBlock if it exceeds. ref: https://api.slack.com/reference/block-kit/blocks#header
		log.Printf("Header text exceeds the 150 characters length limit")
		headerBlock = slack.NewSectionBlock(headerTextBlockObject, nil, nil)
	} else {
		headerBlock = slack.NewHeaderBlock(headerTextBlockObject)
	}

	// Fields

	timestampText := fmt.Sprintf("The operation started at %q", upgradePayload.OperationStartTime)
	timestampBlock := slack.NewSectionBlock(slack.NewTextBlockObject(markdownElementType, timestampText, false, false), nil, nil)

	cliDescribeText := fmt.Sprintf("To check the operation detail, please run `gcloud container operations --project '%v' describe  '%v' --region '%v'`", projectID, upgradePayload.Operation, psm.Attributes["cluster_location"])
	cliDescribeBlock := slack.NewSectionBlock(slack.NewTextBlockObject(markdownElementType, cliDescribeText, false, false), nil, nil)

	cliCanceltext := fmt.Sprintf("To cancel the operation, please run `gcloud container operations --project '%v' cancel '%v' --region '%v'`", projectID, upgradePayload.Operation, psm.Attributes["cluster_location"])
	cliCancelBlock := slack.NewSectionBlock(slack.NewTextBlockObject(markdownElementType, cliCanceltext, false, false), nil, nil)

	var blockset []slack.Block
	blockset = append(blockset, headerBlock, timestampBlock, *cliDescribeBlock, *cliCancelBlock)
	slackMessage.Blocks = &slack.Blocks{
		BlockSet: blockset,
	}
	return slackMessage
}

// TODO: we are not aware of the message format of the security yet. Just post plain text of the message we receive
func SecurityEvent(ctx context.Context, psm PubSubMessage, slackMessage slack.WebhookMessage) slack.WebhookMessage {
	payload := psm.Attributes["payload"]
	headerText := string(psm.Data)
	headerTextBlockObject := slack.NewTextBlockObject(plaintextElementType, headerText, false, false)
	fieldTextBlockObject := []*slack.TextBlockObject{slack.NewTextBlockObject(markdownElementType, payload, false, false)}
	blockSection := slack.NewSectionBlock(headerTextBlockObject, fieldTextBlockObject, nil)

	var blockset []slack.Block
	blockset = append(blockset, blockSection)
	slackMessage.Blocks = &slack.Blocks{
		BlockSet: blockset,
	}
	return slackMessage
}
