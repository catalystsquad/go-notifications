package internal

import (
	"encoding/json"

	"github.com/catalystsquad/app-utils-go/logging"
	"github.com/catalystsquad/go-notifications/notification_store"
	"github.com/catalystsquad/go-scheduler/pkg"
	notificationsv1alpha1 "github.com/catalystsquad/protos-go-notifications/gen/proto/go/notifications/v1alpha1"
	"google.golang.org/protobuf/encoding/protojson"
)

func HandleScheduledNotification(task pkg.TaskInstance) error {
	bytes, err := json.Marshal(task.TaskDefinition.Metadata)
	if err != nil {
		logging.Log.WithError(err).Error("error marshalling task defintion metadata to json")
		return err
	}
	event := &notificationsv1alpha1.ScheduledNotification{}
	marshaller := protojson.UnmarshalOptions{DiscardUnknown: true}
	err = marshaller.Unmarshal(bytes, event)
	if err != nil {
		logging.Log.WithError(err).Error("error marshalling json to notification event")
		return err
	}
	event.Notification.Topic = GetUserTopic(event.UserId)
	err = notification_store.NotificationStore.PublishEvents([]*notificationsv1alpha1.NotificationEvent{event.Notification})
	if err != nil {
		logging.Log.WithError(err).Error("error sending scheduled notification")
		return err
	}
	return nil
}
