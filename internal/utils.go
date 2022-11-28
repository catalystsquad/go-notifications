package internal

import (
	"encoding/json"
	"fmt"
	"github.com/catalystsquad/go-scheduler/pkg"
	notificationsv1alpha1 "github.com/catalystsquad/protos-go-notifications/gen/proto/go/notifications/v1alpha1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"time"
)

func GetTaskDefinitionFromScheduledNotification(notification *notificationsv1alpha1.ScheduledNotification) (*pkg.TaskDefinition, error) {
	if notification.Id == "" {
		notification.Id = uuid.Nil.String()
	}
	id, err := uuid.Parse(notification.Id)
	if err != nil {
		return nil, err
	}
	scheduledNotificationDefinition := &pkg.TaskDefinition{
		Id:          &id,
		Metadata:    notification,
		ExpireAfter: time.Duration(notification.ExpireAfter),
	}
	notificationExecuteOnceTrigger := notification.GetExecuteOnceTrigger()
	notificationCronTrigger := notification.GetCronTrigger()
	if notificationExecuteOnceTrigger != nil {
		fireAt, err := time.Parse(time.RFC3339, notificationExecuteOnceTrigger.FireAt)
		if err != nil {
			return nil, err
		}
		scheduledNotificationDefinition.ExecuteOnceTrigger = pkg.NewExecuteOnceTrigger(fireAt)
	} else {
		cronTrigger, err := pkg.NewCronTrigger(notificationCronTrigger.Expression)
		if err != nil {
			return nil, err
		}
		scheduledNotificationDefinition.CronTrigger = cronTrigger
	}
	return scheduledNotificationDefinition, nil
}

func GetUuidsFromStrings(ids []string) ([]*uuid.UUID, error) {
	uuids := []*uuid.UUID{}
	for _, id := range ids {
		parsedUuid, err := uuid.Parse(id)
		if err != nil {
			return nil, err
		}
		uuids = append(uuids, &parsedUuid)
	}
	return uuids, nil
}

func GetScheduledNotificationsFromTaskDefinitions(definitions []pkg.TaskDefinition) ([]*notificationsv1alpha1.ScheduledNotification, error) {
	scheduledNotifications := []*notificationsv1alpha1.ScheduledNotification{}
	for _, definition := range definitions {
		scheduledNotification, err := GetScheduledNotificationFromTaskDefinition(definition)
		if err != nil {
			return nil, err
		}
		scheduledNotifications = append(scheduledNotifications, scheduledNotification)
	}
	return scheduledNotifications, nil
}

func GetScheduledNotificationFromTaskDefinition(definition pkg.TaskDefinition) (*notificationsv1alpha1.ScheduledNotification, error) {
	metadataJson, err := json.Marshal(definition.Metadata)
	if err != nil {
		return nil, err
	}
	scheduledNotification := &notificationsv1alpha1.ScheduledNotification{}
	marshaller := protojson.UnmarshalOptions{DiscardUnknown: true}
	err = marshaller.Unmarshal(metadataJson, scheduledNotification)
	if err != nil {
		return nil, err
	}
	setId(definition, scheduledNotification)
	err = setTrigger(definition, scheduledNotification)
	return scheduledNotification, err
}

func setId(definition pkg.TaskDefinition, scheduledNotification *notificationsv1alpha1.ScheduledNotification) {
	scheduledNotification.Id = definition.Id.String()
}

func setTrigger(definition pkg.TaskDefinition, scheduledNotification *notificationsv1alpha1.ScheduledNotification) error {
	if definition.ExecuteOnceTrigger != nil {
		triggerBytes, err := json.Marshal(definition.ExecuteOnceTrigger)
		if err != nil {
			return err
		}
		executeOnceTrigger := &notificationsv1alpha1.ExecuteOnceTrigger{}
		err = protojson.Unmarshal(triggerBytes, executeOnceTrigger)
		if err != nil {
			return err
		}
		scheduledNotification.Trigger = &notificationsv1alpha1.ScheduledNotification_ExecuteOnceTrigger{ExecuteOnceTrigger: executeOnceTrigger}
	} else {
		triggerBytes, err := json.Marshal(definition.CronTrigger)
		if err != nil {
			return err
		}
		cronTrigger := &notificationsv1alpha1.CronTrigger{}
		err = protojson.Unmarshal(triggerBytes, cronTrigger)
		if err != nil {
			return err
		}
		scheduledNotification.Trigger = &notificationsv1alpha1.ScheduledNotification_CronTrigger{CronTrigger: cronTrigger}
	}
	return nil
}

func GetUserTopic(userId string) string {
	return fmt.Sprintf("users/%s", userId)
}
