package notifo_store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/catalystsquad/app-utils-go/logging"
	"github.com/catalystsquad/go-notifications/internal/config"
	"github.com/catalystsquad/notifo-client-go"
	notificationsv1alpha1 "github.com/catalystsquad/protos-go-notifications/gen/proto/go/notifications/v1alpha1"
	"github.com/deepmap/oapi-codegen/pkg/securityprovider"
	"github.com/joomcode/errorx"
	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"net/http"
)

var NotifoStore = NotifoNotificationStore{}
var notifoClient *notifo_client_go.ClientWithResponses
var standardJSON = jsoniter.ConfigCompatibleWithStandardLibrary
var snakeCaseJSON = jsoniter.Config{TagKey: "snake"}.Froze()
var camelCaseJSON = jsoniter.Config{TagKey: "camel"}.Froze()
var protoUnmarshaller = protojson.UnmarshalOptions{DiscardUnknown: true}

type NotifoNotificationStore struct{}

func (n NotifoNotificationStore) UpdateSubscriptions(userId string, subscriptions []*notificationsv1alpha1.SubscriptionSettings, unsubscribe []string) error {
	subscribe := []notifo_client_go.SubscribeDto{}
	for _, subscriptionProto := range subscriptions {
		bytes, err := protojson.Marshal(subscriptionProto)
		if err != nil {
			return err
		}
		var subscribeDto notifo_client_go.SubscribeDto
		err = json.Unmarshal(bytes, &subscribeDto)
		if err != nil {
			return err
		}
		subscribe = append(subscribe, subscribeDto)
	}
	body := notifo_client_go.UsersPostSubscriptionsJSONRequestBody{
		Subscribe:   &subscribe,
		Unsubscribe: &unsubscribe,
	}
	response, err := notifoClient.UsersPostSubscriptionsWithResponse(context.Background(), config.AppConfig.NotifoAppId, userId, body)
	if err != nil {
		return err
	}
	if response.StatusCode() != http.StatusNoContent {
		return unexpectedStatusCodeErrorr(response.StatusCode(), response.HTTPResponse)
	}
	return nil
}

func (n NotifoNotificationStore) Initialize() (deferredFunc func(), err error) {
	notifoClient = initializeNotifoClient()
	return
}

func (n NotifoNotificationStore) GetNotifications(channels []string, userId, query string, limit, skip int32) ([]*notificationsv1alpha1.Notification, int32, error) {
	return getNotifications(channels, userId, query, limit, skip)
}

func (n NotifoNotificationStore) PublishEvents(events []*notificationsv1alpha1.NotificationEvent) error {
	return publishEvents(events)
}

func (n NotifoNotificationStore) ListUsers(skip, limit int32) ([]*notificationsv1alpha1.NotificationUser, error) {
	params := &notifo_client_go.UsersGetUsersParams{
		Take: &limit,
		Skip: &skip,
	}
	response, err := notifoClient.UsersGetUsersWithResponse(context.Background(), config.AppConfig.NotifoAppId, params)
	if err != nil {
		logging.Log.WithError(err).Error("error listing users")
		return nil, err
	}
	if response.StatusCode() != http.StatusOK {
		return nil, unexpectedStatusCodeErrorr(http.StatusOK, response.HTTPResponse)
	}
	users := []*notificationsv1alpha1.NotificationUser{}
	for _, user := range response.JSON200.Items {
		proto := &notificationsv1alpha1.NotificationUser{}
		err = GetProtoFromStructWithMarshaller(protoUnmarshaller, user, proto)
		if err != nil {
			logging.Log.WithError(err).Error("error marshalling notifo response to users response")
			return nil, err
		}
		users = append(users, proto)
	}
	return users, nil
}

func (n NotifoNotificationStore) UpsertUsers(users []*notificationsv1alpha1.NotificationUser) ([]*notificationsv1alpha1.NotificationUser, error) {
	// format request
	requestUsers := []notifo_client_go.UpsertUserDto{}
	for _, user := range users {
		dto := notifo_client_go.UpsertUserDto{
			Id:           &user.Id,
			EmailAddress: &user.EmailAddress,
		}
		requestUsers = append(requestUsers, dto)
	}
	request := notifo_client_go.UsersPostUsersJSONRequestBody{Requests: requestUsers}
	response, err := notifoClient.UsersPostUsersWithResponse(context.Background(), config.AppConfig.NotifoAppId, request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode() != http.StatusOK {
		return nil, unexpectedStatusCodeErrorr(http.StatusOK, response.HTTPResponse)
	}
	// marshall response to proto, more complicated because we use snake case and notifo uses camel case
	protos := []*notificationsv1alpha1.NotificationUser{}
	for _, user := range *response.JSON200 {
		proto := &notificationsv1alpha1.NotificationUser{}
		err = GetProtoFromStructWithMarshaller(protoUnmarshaller, user, proto)
		if err != nil {
			return nil, err
		}
		protos = append(protos, proto)
	}
	return protos, nil
}

func (n NotifoNotificationStore) GetUsers(ids []string) ([]*notificationsv1alpha1.NotificationUser, error) {
	users := []*notificationsv1alpha1.NotificationUser{}
	group := errgroup.Group{}
	for _, id := range ids {
		id := id // https://golang.org/doc/faq#closures_and_goroutines
		group.Go(func() error {
			proto, err := getUser(id, false)
			if err != nil {
				return err
			}
			if proto != nil {
				users = append(users, proto)
			}
			return nil
		})
	}
	err := group.Wait()
	return users, err
}

func (n NotifoNotificationStore) DeleteUsers(ids []string) error {
	group := errgroup.Group{}
	for _, id := range ids {
		id := id // https://golang.org/doc/faq#closures_and_goroutines
		group.Go(func() error {
			return deleteUser(id)
		})
	}
	return group.Wait()
}

func initializeNotifoClient() *notifo_client_go.ClientWithResponses {
	logging.Log.WithFields(logrus.Fields{"base_url": config.AppConfig.NotifoBaseUrl}).Info("initializing notifo client")
	apiKeyProvider, err := securityprovider.NewSecurityProviderApiKey("header", "X-ApiKey", config.AppConfig.NotifoApiKey)
	if err != nil {
		panic(err)
	}
	notifoClient, err := notifo_client_go.NewClientWithResponses(config.AppConfig.NotifoBaseUrl, notifo_client_go.WithRequestEditorFn(apiKeyProvider.Intercept))
	if err != nil {
		panic(err)
	}
	return notifoClient
}

func unexpectedStatusCodeErrorr(expectedStatusCode int, resp *http.Response) error {
	body, _ := getResponseBody(resp)
	return errorx.IllegalState.New("expected status code %d but got %d with body %s", expectedStatusCode, resp.StatusCode, body)
}

func getUser(id string, withDetails bool) (*notificationsv1alpha1.NotificationUser, error) {
	params := &notifo_client_go.UsersGetUserParams{
		WithDetails: &withDetails,
	}
	response, err := notifoClient.UsersGetUserWithResponse(context.Background(), config.AppConfig.NotifoAppId, id, params)
	if err != nil {
		return nil, err
	}
	if response.StatusCode() != http.StatusOK && response.StatusCode() != http.StatusNotFound {
		// not found is notifo's response when there are no users, which is not an error case
		return nil, unexpectedStatusCodeErrorr(response.StatusCode(), response.HTTPResponse)
	}
	proto := &notificationsv1alpha1.NotificationUser{}
	if response.JSON200 == nil {
		return nil, nil
	}
	err = GetProtoFromStructWithMarshaller(protoUnmarshaller, response.JSON200, proto)
	return proto, err
}

func deleteUser(id string) error {
	response, err := notifoClient.UsersDeleteUser(context.Background(), config.AppConfig.NotifoAppId, id)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusNoContent {
		return unexpectedStatusCodeErrorr(http.StatusNoContent, response)
	}
	return nil
}

func getNotifications(channels []string, userId, query string, take, skip int32) ([]*notificationsv1alpha1.Notification, int32, error) {
	params := &notifo_client_go.NotificationsGetNotificationsParams{
		Channels: &channels,
		Take:     &take,
		Skip:     &skip,
	}
	if query != "" {
		params.Query = &query
	}
	response, err := notifoClient.NotificationsGetNotificationsWithResponse(context.Background(), config.AppConfig.NotifoAppId, userId, params)
	if err != nil {
		return nil, 0, err
	}
	if response.StatusCode() != http.StatusOK {
		return nil, 0, errors.New(string(response.Body))
	}
	protos := []*notificationsv1alpha1.Notification{}
	for _, notification := range response.JSON200.Items {
		proto := &notificationsv1alpha1.Notification{}
		err = GetProtoFromStructWithMarshaller(protoUnmarshaller, notification, proto)
		if err != nil {
			return nil, 0, err
		}
		protos = append(protos, proto)
	}

	return protos, int32(response.JSON200.Total), err
}

func publishEvents(events []*notificationsv1alpha1.NotificationEvent) error {
	publishes := []notifo_client_go.PublishDto{}
	for _, event := range events {
		bytes, err := protojson.Marshal(event)
		if err != nil {
			return err
		}
		var publishDto notifo_client_go.PublishDto
		err = json.Unmarshal(bytes, &publishDto)
		if err != nil {
			return err
		}
		publishes = append(publishes, publishDto)
	}
	params := notifo_client_go.EventsPostEventsJSONRequestBody{
		Requests: publishes,
	}
	response, err := notifoClient.EventsPostEventsWithResponse(context.Background(), config.AppConfig.NotifoAppId, params)
	if err != nil {
		return err
	}
	if response.StatusCode() != http.StatusNoContent {
		return unexpectedStatusCodeErrorr(http.StatusNoContent, response.HTTPResponse)
	}
	return nil
}

func getResponseBody(response *http.Response) (string, error) {
	defer response.Body.Close()
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}
