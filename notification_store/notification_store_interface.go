package notification_store

import (
	notificationsv1alpha1 "github.com/catalystsquad/protos-go-notifications/gen/proto/go/notifications/v1alpha1"
)

var NotificationStore NotificationStoreInterface

type NotificationStoreInterface interface {
	Initialize() (deferredFunc func(), err error)
	UpsertUsers(users []*notificationsv1alpha1.NotificationUser) ([]*notificationsv1alpha1.NotificationUser, error)
	GetUsers(ids []string) ([]*notificationsv1alpha1.NotificationUser, error)
	ListUsers(skip, limit int32) ([]*notificationsv1alpha1.NotificationUser, error)
	DeleteUsers(ids []string) error
	GetNotifications(channels []string, userId, query string, limit, skip int32) ([]*notificationsv1alpha1.Notification, int32, error)
	PublishEvents(events []*notificationsv1alpha1.NotificationEvent) error
	UpdateSubscriptions(userId string, subscriptions []*notificationsv1alpha1.SubscriptionSettings, unsubscribe []string) error
}
