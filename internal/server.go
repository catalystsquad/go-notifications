package internal

import (
	"context"
	"fmt"

	"github.com/catalystsquad/app-utils-go/logging"
	"github.com/catalystsquad/go-notifications/internal/errors"
	"github.com/catalystsquad/go-notifications/notification_store"
	"github.com/catalystsquad/go-scheduler/pkg"
	notificationsv1alpha1 "github.com/catalystsquad/protos-go-notifications/gen/proto/go/notifications/v1alpha1"
	"github.com/joomcode/errorx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// exists for side effects, checks implementation at compile time, so compile
// will fail if the interface isn't implemented corectly.
var _ notificationsv1alpha1.NotificationsServiceServer = &NotificationsServiceServer{}

type NotificationsServiceServer struct{}

func (n NotificationsServiceServer) UpsertScheduledNotifications(ctx context.Context, request *notificationsv1alpha1.NotificationsServiceUpsertScheduledNotificationsRequest) (*notificationsv1alpha1.NotificationsServiceUpsertScheduledNotificationsResponse, error) {
	err := upsertNotifications(request.Notifications)
	if err != nil {
		logging.Log.WithError(err).Error("error scheduling notifications")
		return nil, status.Error(codes.Internal, errors.UnexpectedError)
	}
	return &notificationsv1alpha1.NotificationsServiceUpsertScheduledNotificationsResponse{Success: true}, nil
}

func (n NotificationsServiceServer) GetScheduledNotifications(ctx context.Context, request *notificationsv1alpha1.NotificationsServiceGetScheduledNotificationsRequest) (*notificationsv1alpha1.NotificationsServiceGetScheduledNotificationsResponse, error) {
	var taskDefinitions []pkg.TaskDefinition
	var err error
	// default the limit if it's not set
	if request.Limit == 0 {
		request.Limit = 10
	}
	if len(request.Ids) > 0 {
		// query for ids directly
		uuids, err := GetUuidsFromStrings(request.Ids)
		if err != nil {
			return nil, err
		}
		taskDefinitions, err = Scheduler.GetTaskDefinitions(uuids)
	} else if request.UserId != "" {
		// query by user id
		query := fmt.Sprintf("metadata->>'user_id' = '%s'", request.UserId)
		taskDefinitions, err = Scheduler.ListTaskDefinitions(int(request.Skip), int(request.Limit), query)
		if err != nil {
			return nil, err
		}
	} else {
		// simple list
		taskDefinitions, err = Scheduler.ListTaskDefinitions(int(request.Skip), int(request.Limit), nil)
	}
	if err != nil {
		logging.Log.WithError(err).Error("error getting scheduled notifications")
		return nil, err
	}
	scheduledNotifications, err := GetScheduledNotificationsFromTaskDefinitions(taskDefinitions)
	if err != nil {
		logging.Log.WithError(err).Error("error getting scheduled notifications")
		return nil, err
	}
	return &notificationsv1alpha1.NotificationsServiceGetScheduledNotificationsResponse{ScheduledNotifications: scheduledNotifications}, nil
}

func (n NotificationsServiceServer) DeleteScheduledNotifications(ctx context.Context, request *notificationsv1alpha1.NotificationsServiceDeleteScheduledNotificationsRequest) (*notificationsv1alpha1.NotificationsServiceDeleteScheduledNotificationsResponse, error) {
	ids, err := GetUuidsFromStrings(request.Ids)
	if err != nil {
		return nil, err
	}
	// delete from scheduler
	err = Scheduler.DeleteTaskDefinitions(ids)
	if err != nil {
		return nil, err
	}
	return &notificationsv1alpha1.NotificationsServiceDeleteScheduledNotificationsResponse{Success: true}, nil
}

func (n NotificationsServiceServer) UpsertUsers(ctx context.Context, request *notificationsv1alpha1.NotificationsServiceUpsertUsersRequest) (*notificationsv1alpha1.NotificationsServiceUpsertUsersResponse, error) {
	users, err := notification_store.NotificationStore.UpsertUsers(request.Users)
	if err != nil {
		logging.Log.WithError(err).Error("error upserting users")
		return nil, status.Error(codes.Internal, errors.UnexpectedError)
	}
	return &notificationsv1alpha1.NotificationsServiceUpsertUsersResponse{Users: users}, nil
}

func (n NotificationsServiceServer) GetUsers(ctx context.Context, request *notificationsv1alpha1.NotificationsServiceGetUsersRequest) (*notificationsv1alpha1.NotificationsServiceGetUsersResponse, error) {
	users, err := notification_store.NotificationStore.GetUsers(request.Ids)
	if err != nil {
		logging.Log.WithError(err).Error("error getting users")
		return nil, err
	}
	return &notificationsv1alpha1.NotificationsServiceGetUsersResponse{Users: users}, nil
}

func (n NotificationsServiceServer) ListUsers(ctx context.Context, request *notificationsv1alpha1.NotificationsServiceListUsersRequest) (*notificationsv1alpha1.NotificationsServiceListUsersResponse, error) {
	users, err := notification_store.NotificationStore.ListUsers(request.Skip, request.Limit)
	if err != nil {
		logging.Log.WithError(err).Error("error listing users")
		return nil, status.Error(codes.Internal, errors.UnexpectedError)
	}
	return &notificationsv1alpha1.NotificationsServiceListUsersResponse{Users: users}, nil
}

func (n NotificationsServiceServer) DeleteUsers(ctx context.Context, request *notificationsv1alpha1.NotificationsServiceDeleteUsersRequest) (*notificationsv1alpha1.NotificationsServiceDeleteUsersResponse, error) {
	// delete from notifications store
	err := notification_store.NotificationStore.DeleteUsers(request.Ids)
	if err != nil {
		logging.Log.WithError(err).Error("error deleting users")
		return nil, status.Error(codes.Internal, errors.UnexpectedError)
	}
	formattedIds := []string{}
	for _, id := range request.Ids {
		formattedIds = append(formattedIds, fmt.Sprintf("'%s'", id))
	}
	query := fmt.Sprintf("metadata->'user_id'?|array%s;", formattedIds)
	err = Scheduler.DeleteTaskDefinitionsByMetadataQuery(query)
	if err != nil {
		return nil, err
	}
	return &notificationsv1alpha1.NotificationsServiceDeleteUsersResponse{Success: true}, nil
}

func (n NotificationsServiceServer) GetNotifications(ctx context.Context, request *notificationsv1alpha1.NotificationsServiceGetNotificationsRequest) (*notificationsv1alpha1.NotificationsServiceGetNotificationsResponse, error) {
	notifications, total, err := notification_store.NotificationStore.GetNotifications(request.Channels, request.UserId, request.Query, request.Limit, request.Skip)
	if err != nil {
		logging.Log.WithError(err).Error("error getting notifications")
		return nil, status.Error(codes.Internal, errors.UnexpectedError)
	}
	return &notificationsv1alpha1.NotificationsServiceGetNotificationsResponse{
		Notifications: notifications,
		Total:         total,
	}, nil
}

func (n NotificationsServiceServer) SendNotifications(ctx context.Context, request *notificationsv1alpha1.NotificationsServiceSendNotificationsRequest) (*notificationsv1alpha1.NotificationsServiceSendNotificationsResponse, error) {
	err := notification_store.NotificationStore.PublishEvents(request.Notifications)
	if err != nil {
		logging.Log.WithError(err).Error("error sending notifications")
		return nil, status.Error(codes.Internal, errors.UnexpectedError)
	}
	return &notificationsv1alpha1.NotificationsServiceSendNotificationsResponse{Success: true}, nil
}

func (n NotificationsServiceServer) UpdateSubscriptions(ctx context.Context, request *notificationsv1alpha1.NotificationsServiceUpdateSubscriptionsRequest) (*notificationsv1alpha1.NotificationsServiceUpdateSubscriptionsResponse, error) {
	err := notification_store.NotificationStore.UpdateSubscriptions(request.UserId, request.Subscribe, request.Unsubscribe)
	if err != nil {
		logging.Log.WithError(err).Error("error updating subscriptions")
		return nil, status.Error(codes.Internal, errors.UnexpectedError)
	}
	return &notificationsv1alpha1.NotificationsServiceUpdateSubscriptionsResponse{Success: true}, nil
}

func upsertNotifications(scheduledNotifications []*notificationsv1alpha1.ScheduledNotification) error {
	for _, scheduledNotification := range scheduledNotifications {
		err := upsertNotification(scheduledNotification)
		if err != nil {
			return err
		}
	}
	return nil
}

func upsertNotification(notification *notificationsv1alpha1.ScheduledNotification) error {
	if notification.UserId == "" {
		return errorx.IllegalArgument.New("scheduled notifications must have a user id")
	}
	definition, err := GetTaskDefinitionFromScheduledNotification(notification)
	if err != nil {
		return err
	}
	err = Scheduler.UpsertTaskDefinition(*definition)
	if err != nil {
		return err
	}
	return nil
}
