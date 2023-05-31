package test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/catalystsquad/go-notifications/internal"
	notificationsv1alpha1 "github.com/catalystsquad/protos-go-notifications/gen/proto/go/notifications/v1alpha1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

var NotificationsClient notificationsv1alpha1.NotificationsServiceClient
var NotificationsConn *grpc.ClientConn

const oncePerSecondCron = "* * * * * * *"

// these tests can be used to test notifo and the notifications api. Notifo requries setup that is difficult to replicate in CI, so I'm leaving these tests as local/manual tests for now.
type NotificationsSuite struct {
	suite.Suite
}

func (s *NotificationsSuite) SetupSuite() {
	initializeNotificationsClient(5 * time.Second)
}

func (s *NotificationsSuite) TearDownSuite() {
	NotificationsConn.Close()
}

func (s *NotificationsSuite) TearDownTest() {
	// delete all users
	err := deleteAllUsers()
	require.NoError(s.T(), err)
}

func TestNotificationsSuite(t *testing.T) {
	suite.Run(t, new(NotificationsSuite))
}

func (s *NotificationsSuite) TestUserCrud() {
	// upsert
	numUsers := 1
	users := generateUsers(numUsers)
	req := &notificationsv1alpha1.NotificationsServiceUpsertUsersRequest{Users: users}
	resp, err := NotificationsClient.UpsertUsers(context.Background(), req)
	require.NoError(s.T(), err)
	require.Len(s.T(), resp.Users, numUsers)
	// get
	ids := []string{}
	for _, user := range users {
		ids = append(ids, user.Id)
	}
	getReq := &notificationsv1alpha1.NotificationsServiceGetUsersRequest{Ids: ids}
	getResp, err := NotificationsClient.GetUsers(context.Background(), getReq)
	require.NoError(s.T(), err)
	require.Len(s.T(), getResp.Users, numUsers)
	testUser := users[0]
	testUserTopic := internal.GetUserTopic(testUser.Id)
	// send event
	publishResponse, err := sendNotifications(testUser.Id, testUserTopic, `{"some": "stuff"}`, "test subject", "test body")
	require.NoError(s.T(), err)
	require.True(s.T(), publishResponse.Success)
	// get notifications, need to sleep to give notifo time to process the notification, it has rabbit/mongo stuff
	// to do and if we don't sleep, this call fails because no notifications are present.
	time.Sleep(2 * time.Second)
	getNotificationsResponse, err := getNotifications(testUser.Id, []string{"web"}, 10, 0)
	require.NoError(s.T(), err)
	require.Len(s.T(), getNotificationsResponse.Notifications, 1)
	// schedule a notification for 5 seconds from now
	trigger := &notificationsv1alpha1.ScheduledNotification_ExecuteOnceTrigger{ExecuteOnceTrigger: &notificationsv1alpha1.ExecuteOnceTrigger{FireAt: time.Now().Add(5 * time.Second).Format(time.RFC3339)}}
	scheduleNotificationResponse, err := scheduleNotification(trigger, nil, 1*time.Minute.Nanoseconds(), testUser.Id, testUserTopic, `{"scheduled": "data"}`, "test subject", "test body")
	require.NoError(s.T(), err)
	require.True(s.T(), scheduleNotificationResponse.Success)
	// test simple scheduled notification list
	listedScheduledNotificationsResponse, err := NotificationsClient.GetScheduledNotifications(context.Background(), &notificationsv1alpha1.NotificationsServiceGetScheduledNotificationsRequest{Skip: 0, Limit: 100})
	require.NoError(s.T(), err)
	require.Len(s.T(), listedScheduledNotificationsResponse.ScheduledNotifications, 1)
	scheduledNotification := listedScheduledNotificationsResponse.ScheduledNotifications[0]
	// test list by user id
	listedScheduledNotificationsResponse, err = NotificationsClient.GetScheduledNotifications(context.Background(), &notificationsv1alpha1.NotificationsServiceGetScheduledNotificationsRequest{UserId: testUser.Id})
	require.NoError(s.T(), err)
	require.Len(s.T(), listedScheduledNotificationsResponse.ScheduledNotifications, 1)
	// test list by id
	listedScheduledNotificationsResponse, err = NotificationsClient.GetScheduledNotifications(context.Background(), &notificationsv1alpha1.NotificationsServiceGetScheduledNotificationsRequest{Ids: []string{scheduledNotification.Id}})
	require.NoError(s.T(), err)
	require.Len(s.T(), listedScheduledNotificationsResponse.ScheduledNotifications, 1)
	// sleep for 2 seconds, get notifications, verify it's not there
	time.Sleep(2 * time.Second)
	// get notifications
	getNotificationsResponse, err = getNotifications(testUser.Id, []string{"web"}, 10, 0)
	require.NoError(s.T(), err)
	// should still only be one because the user unsubscribed
	require.Len(s.T(), getNotificationsResponse.Notifications, 1)
	// sleep for 4 seconds, get notifications, verify it's there
	time.Sleep(4 * time.Second)
	getNotificationsResponse, err = getNotifications(testUser.Id, []string{"web"}, 10, 0)
	require.NoError(s.T(), err)
	require.Len(s.T(), getNotificationsResponse.Notifications, 2)
	// delete
	deleteReq := &notificationsv1alpha1.NotificationsServiceDeleteUsersRequest{Ids: ids}
	deleteResp, err := NotificationsClient.DeleteUsers(context.Background(), deleteReq)
	require.NoError(s.T(), err)
	require.True(s.T(), deleteResp.Success)
	// get to ensure delete
	getResp, err = NotificationsClient.GetUsers(context.Background(), getReq)
	require.NoError(s.T(), err)
	require.Len(s.T(), getResp.Users, 0)
}

func (s *NotificationsSuite) TestScheduledExecuteOnceNotification() {
	// upsert
	numUsers := 1
	users := generateUsers(numUsers)
	req := &notificationsv1alpha1.NotificationsServiceUpsertUsersRequest{Users: users}
	_, err := NotificationsClient.UpsertUsers(context.Background(), req)
	require.NoError(s.T(), err)
	testUser := users[0]
	testUserTopic := internal.GetUserTopic(testUser.Id)
	// schedule a notification for 5 seconds from now
	trigger := &notificationsv1alpha1.ScheduledNotification_ExecuteOnceTrigger{ExecuteOnceTrigger: &notificationsv1alpha1.ExecuteOnceTrigger{FireAt: time.Now().Add(5 * time.Second).UTC().Format(time.RFC3339)}}
	scheduleNotificationResponse, err := scheduleNotification(trigger, nil, 1*time.Minute.Nanoseconds(), testUser.Id, testUserTopic, `{"scheduled": "data"}`, "test subject", "test body")
	require.NoError(s.T(), err)
	require.True(s.T(), scheduleNotificationResponse.Success)
	// sleep for 2 seconds, get notifications, verify it's not there
	time.Sleep(2 * time.Second)
	// get notifications
	getNotificationsResponse, err := getNotifications(testUser.Id, []string{"web"}, 10, 0)
	require.NoError(s.T(), err)
	require.Len(s.T(), getNotificationsResponse.Notifications, 0)
	// sleep for 4 seconds, get notifications, verify it's there
	time.Sleep(4 * time.Second)
	getNotificationsResponse, err = getNotifications(testUser.Id, []string{"web"}, 10, 0)
	require.NoError(s.T(), err)
	require.Len(s.T(), getNotificationsResponse.Notifications, 1)
	// sleep for 6 seconds, verify it wasn't redelivered
	time.Sleep(6 * time.Second)
	getNotificationsResponse, err = getNotifications(testUser.Id, []string{"web"}, 10, 0)
	require.NoError(s.T(), err)
	require.Len(s.T(), getNotificationsResponse.Notifications, 1)
}

func (s *NotificationsSuite) TestScheduledCronNotification() {
	// upsert
	numUsers := 1
	users := generateUsers(numUsers)
	req := &notificationsv1alpha1.NotificationsServiceUpsertUsersRequest{Users: users}
	_, err := NotificationsClient.UpsertUsers(context.Background(), req)
	require.NoError(s.T(), err)
	testUser := users[0]
	testUserTopic := internal.GetUserTopic(testUser.Id)
	// schedule a notification for once per second
	trigger := &notificationsv1alpha1.ScheduledNotification_CronTrigger{CronTrigger: &notificationsv1alpha1.CronTrigger{Expression: oncePerSecondCron}}
	scheduleNotificationResponse, err := scheduleNotification(nil, trigger, 1*time.Minute.Nanoseconds(), testUser.Id, testUserTopic, `{"scheduled": "data"}`, gofakeit.HackeringVerb(), gofakeit.HackerPhrase())
	require.NoError(s.T(), err)
	require.True(s.T(), scheduleNotificationResponse.Success)
	// sleep for 6 seconds, get notifications, verify it's executed at least 4 times, no more than 6 times. This is imprecise because
	// notifo has to process things in it's internal event structure before the notifications appear and that's not always as exact as
	// we'd want for testing.
	time.Sleep(7 * time.Second)
	getNotificationsResponse, err := getNotifications(testUser.Id, []string{"web"}, 10, 0)
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), len(getNotificationsResponse.Notifications), 4)
	require.LessOrEqual(s.T(), len(getNotificationsResponse.Notifications), 7)
}

func generateUsers(num int) []*notificationsv1alpha1.NotificationUser {
	users := []*notificationsv1alpha1.NotificationUser{}
	for i := 0; i < num; i++ {
		user := generateUser()
		users = append(users, user)
	}
	return users
}

func generateUser() *notificationsv1alpha1.NotificationUser {
	return &notificationsv1alpha1.NotificationUser{
		Id:           uuid.NewString(),
		EmailAddress: gofakeit.Email(),
	}
}

func initializeNotificationsClient(timeout time.Duration) {
	connectTo := "127.0.0.1:6000"
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, connectTo, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	NotificationsConn = conn
	NotificationsClient = notificationsv1alpha1.NewNotificationsServiceClient(NotificationsConn)
}

func scheduleNotification(executeOnceTrigger *notificationsv1alpha1.ScheduledNotification_ExecuteOnceTrigger, cronTrigger *notificationsv1alpha1.ScheduledNotification_CronTrigger, expireAfter int64, user_id, topic, data, subject, body string) (*notificationsv1alpha1.NotificationsServiceUpsertScheduledNotificationsResponse, error) {
	event, err := buildNotificationEvent(topic, data, subject, body)
	if err != nil {
		return nil, err
	}
	scheduledNotification := &notificationsv1alpha1.ScheduledNotification{
		UserId:       user_id,
		Notification: event,
		ExpireAfter:  expireAfter,
	}
	if executeOnceTrigger != nil {
		scheduledNotification.Trigger = executeOnceTrigger
	} else {
		scheduledNotification.Trigger = cronTrigger
	}
	req := &notificationsv1alpha1.NotificationsServiceUpsertScheduledNotificationsRequest{Notifications: []*notificationsv1alpha1.ScheduledNotification{scheduledNotification}}
	return NotificationsClient.UpsertScheduledNotifications(context.Background(), req)
}

func sendNotifications(user_id, topic, data, subject, body string) (*notificationsv1alpha1.NotificationsServiceSendNotificationsResponse, error) {
	event, err := buildNotificationEvent(topic, data, subject, body)
	if err != nil {
		return nil, err
	}
	publishReq := &notificationsv1alpha1.NotificationsServiceSendNotificationsRequest{
		Notifications: []*notificationsv1alpha1.NotificationEvent{event},
	}
	return NotificationsClient.SendNotifications(context.Background(), publishReq)
}

func buildNotificationEvent(topic, data, subject, body string) (*notificationsv1alpha1.NotificationEvent, error) {
	subjectMap := map[string]interface{}{"en": subject}
	bodyMap := map[string]interface{}{"en": body}
	subjectStruct, err := mapToStruct(subjectMap)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	bodyStruct, err := mapToStruct(bodyMap)
	if err != nil {
		return nil, err
	}
	return &notificationsv1alpha1.NotificationEvent{
		Topic: topic,
		Data:  data,
		Preformatted: &notificationsv1alpha1.NotificationEventFormatting{
			Subject: subjectStruct,
			Body:    bodyStruct,
		},
	}, nil
}

func getNotifications(userId string, channels []string, limit, skip int32) (*notificationsv1alpha1.NotificationsServiceGetNotificationsResponse, error) {
	getNotificationsRequest := &notificationsv1alpha1.NotificationsServiceGetNotificationsRequest{
		UserId:   userId,
		Channels: channels,
		Limit:    limit,
		Skip:     skip,
	}
	return NotificationsClient.GetNotifications(context.Background(), getNotificationsRequest)
}

func updateSubscriptions(userId string, subscribe []*notificationsv1alpha1.SubscriptionSettings, unsubscribe []string) (*notificationsv1alpha1.NotificationsServiceUpdateSubscriptionsResponse, error) {
	subscribeReq := &notificationsv1alpha1.NotificationsServiceUpdateSubscriptionsRequest{
		UserId:      userId,
		Subscribe:   subscribe,
		Unsubscribe: unsubscribe,
	}
	return NotificationsClient.UpdateSubscriptions(context.Background(), subscribeReq)
}

func mapToStruct(theMap map[string]interface{}) (*structpb.Struct, error) {
	mapJson, err := json.Marshal(theMap)
	if err != nil {
		return nil, err
	}
	theStruct := &structpb.Struct{}
	err = protojson.Unmarshal(mapJson, theStruct)
	return theStruct, err
}

func deleteAllUsers() error {
	response, err := NotificationsClient.ListUsers(context.Background(), &notificationsv1alpha1.NotificationsServiceListUsersRequest{Limit: 1000, Skip: 0})
	if err != nil {
		return err
	}
	ids := []string{}
	for _, user := range response.Users {
		ids = append(ids, user.Id)
	}
	_, err = NotificationsClient.DeleteUsers(context.Background(), &notificationsv1alpha1.NotificationsServiceDeleteUsersRequest{Ids: ids})
	return err
}
